package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type pendingHostKey struct {
	profileID string
	address   string
	key       ssh.PublicKey
	info      HostKeyInfo
}

type TunnelManager struct {
	mu             sync.RWMutex
	hostKeyMu      sync.Mutex
	tunnels        map[string]*runningTunnel
	statuses       map[string]TunnelStatus
	pendingHostKey map[string]pendingHostKey
	knownHostsPath string
	emit           func(TunnelStatus)
}

type runningTunnel struct {
	manager  *TunnelManager
	profile  TunnelProfile
	secret   string
	ctx      context.Context
	cancel   context.CancelFunc
	listener net.Listener

	mu       sync.RWMutex
	client   *ssh.Client
	status   TunnelStatus
	active   atomic.Int32
	stopOnce sync.Once
}

func NewTunnelManager(knownHostsPath string, emit func(TunnelStatus)) *TunnelManager {
	return &TunnelManager{
		tunnels:        make(map[string]*runningTunnel),
		statuses:       make(map[string]TunnelStatus),
		pendingHostKey: make(map[string]pendingHostKey),
		knownHostsPath: knownHostsPath,
		emit:           emit,
	}
}

func (m *TunnelManager) Start(profile TunnelProfile, secret string) OperationResult {
	m.mu.RLock()
	_, exists := m.tunnels[profile.ID]
	m.mu.RUnlock()
	if exists {
		return failedResult("ALREADY_RUNNING", "这个隧道已经在运行")
	}
	m.mu.Lock()
	delete(m.pendingHostKey, profile.ID)
	m.mu.Unlock()

	connecting := stoppedStatus(profile)
	connecting.State = "connecting"
	connecting.Message = "正在验证 SSH 连接"
	m.emitStatus(connecting)

	client, err := m.connect(context.Background(), profile, secret)
	if err != nil {
		if info, ok := m.pendingFor(profile.ID); ok {
			connecting.State = "host-key-required"
			connecting.Message = "请核对并信任服务器主机指纹"
			connecting.HostKey = &info
			m.emitStatus(connecting)
			return OperationResult{OK: false, Code: "HOST_KEY_UNKNOWN", Message: connecting.Message, Status: &connecting, HostKey: &info}
		}
		code := "SSH_CONNECT_FAILED"
		if strings.Contains(err.Error(), "SECRET_REQUIRED") {
			code = "SECRET_REQUIRED"
		} else if strings.Contains(err.Error(), "HOST_KEY_CHANGED") {
			code = "HOST_KEY_CHANGED"
		}
		connecting.State = "error"
		connecting.Message = friendlySSHError(err)
		m.emitStatus(connecting)
		return OperationResult{OK: false, Code: code, Message: connecting.Message, Status: &connecting}
	}

	listener, err := net.Listen("tcp", profile.localEndpoint())
	if err != nil {
		client.Close()
		connecting.State = "error"
		connecting.Message = fmt.Sprintf("本地端口监听失败: %v", err)
		m.emitStatus(connecting)
		return OperationResult{OK: false, Code: "LOCAL_LISTEN_FAILED", Message: connecting.Message, Status: &connecting}
	}

	ctx, cancel := context.WithCancel(context.Background())
	tunnel := &runningTunnel{
		manager:  m,
		profile:  profile,
		secret:   secret,
		ctx:      ctx,
		cancel:   cancel,
		listener: listener,
		client:   client,
	}
	tunnel.status = TunnelStatus{
		ProfileID:      profile.ID,
		State:          "running",
		Message:        "SSH 隧道已连接",
		LocalEndpoint:  profile.localEndpoint(),
		RemoteEndpoint: profile.remoteEndpoint(),
		ConnectedAt:    nowRFC3339(),
	}

	m.mu.Lock()
	m.tunnels[profile.ID] = tunnel
	m.mu.Unlock()
	tunnel.publish()
	go tunnel.acceptLoop()
	go tunnel.monitor(client)
	return OperationResult{OK: true, Message: "隧道已启动", Status: tunnel.statusCopy()}
}

func (m *TunnelManager) Stop(profile TunnelProfile) OperationResult {
	m.mu.Lock()
	tunnel, ok := m.tunnels[profile.ID]
	if ok {
		delete(m.tunnels, profile.ID)
	}
	m.mu.Unlock()
	if ok {
		tunnel.stop()
	}
	status := stoppedStatus(profile)
	m.emitStatus(status)
	return OperationResult{OK: true, Message: "隧道已停止", Status: &status}
}

func (m *TunnelManager) StopAll() {
	m.mu.Lock()
	tunnels := make([]*runningTunnel, 0, len(m.tunnels))
	for _, tunnel := range m.tunnels {
		tunnels = append(tunnels, tunnel)
	}
	m.tunnels = make(map[string]*runningTunnel)
	m.mu.Unlock()
	for _, tunnel := range tunnels {
		tunnel.stop()
	}
}

func (m *TunnelManager) Status(profile TunnelProfile) TunnelStatus {
	m.mu.RLock()
	tunnel := m.tunnels[profile.ID]
	status, hasStatus := m.statuses[profile.ID]
	m.mu.RUnlock()
	if tunnel == nil {
		if hasStatus {
			return status
		}
		return stoppedStatus(profile)
	}
	return *tunnel.statusCopy()
}

func (m *TunnelManager) Reset(profile TunnelProfile) {
	m.mu.Lock()
	delete(m.pendingHostKey, profile.ID)
	m.mu.Unlock()
	m.emitStatus(stoppedStatus(profile))
}

func (m *TunnelManager) Forget(profile TunnelProfile) {
	_ = m.Stop(profile)
	m.mu.Lock()
	delete(m.pendingHostKey, profile.ID)
	delete(m.statuses, profile.ID)
	m.mu.Unlock()
}

func (m *TunnelManager) IsRunning(profileID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.tunnels[profileID]
	return ok
}

func (m *TunnelManager) TrustHost(profile TunnelProfile) OperationResult {
	m.mu.Lock()
	pending, ok := m.pendingHostKey[profile.ID]
	if ok {
		delete(m.pendingHostKey, profile.ID)
	}
	m.mu.Unlock()
	if !ok || pending.address != profile.sshEndpoint() {
		return failedResult("NO_PENDING_HOST_KEY", "没有等待确认的主机指纹，请重新连接")
	}
	m.hostKeyMu.Lock()
	defer m.hostKeyMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(m.knownHostsPath), 0o700); err != nil {
		return failedResult("KNOWN_HOSTS_WRITE_FAILED", err.Error())
	}
	file, err := os.OpenFile(m.knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return failedResult("KNOWN_HOSTS_WRITE_FAILED", err.Error())
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(pending.address)}, pending.key) + "\n"
	if _, err := file.WriteString(line); err != nil {
		file.Close()
		return failedResult("KNOWN_HOSTS_WRITE_FAILED", err.Error())
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return failedResult("KNOWN_HOSTS_WRITE_FAILED", err.Error())
	}
	if err := file.Close(); err != nil {
		return failedResult("KNOWN_HOSTS_WRITE_FAILED", err.Error())
	}
	return OperationResult{OK: true, Message: "主机指纹已保存"}
}

func (m *TunnelManager) connect(ctx context.Context, profile TunnelProfile, secret string) (*ssh.Client, error) {
	auth, err := authMethods(profile, secret)
	if err != nil {
		return nil, err
	}
	callback, err := m.hostKeyCallback(profile)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            profile.Username,
		Auth:            auth,
		HostKeyCallback: callback,
		Timeout:         12 * time.Second,
	}
	dialer := net.Dialer{Timeout: 12 * time.Second, KeepAlive: 30 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", profile.sshEndpoint())
	if err != nil {
		return nil, err
	}
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		connection.Close()
		return nil, err
	}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, profile.sshEndpoint(), config)
	if err != nil {
		connection.Close()
		return nil, err
	}
	_ = connection.SetDeadline(time.Time{})
	return ssh.NewClient(clientConnection, channels, requests), nil
}

func (m *TunnelManager) hostKeyCallback(profile TunnelProfile) (ssh.HostKeyCallback, error) {
	if err := os.MkdirAll(filepath.Dir(m.knownHostsPath), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(m.knownHostsPath, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return nil, err
	}
	file.Close()
	base, err := knownhosts.New(m.knownHostsPath)
	if err != nil {
		return nil, err
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := base(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyError *knownhosts.KeyError
		if errors.As(err, &keyError) && len(keyError.Want) == 0 {
			info := HostKeyInfo{Host: profile.SSHHost, Port: profile.SSHPort, KeyType: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)}
			m.mu.Lock()
			m.pendingHostKey[profile.ID] = pendingHostKey{profileID: profile.ID, address: profile.sshEndpoint(), key: key, info: info}
			m.mu.Unlock()
			return fmt.Errorf("HOST_KEY_UNKNOWN: %s", info.Fingerprint)
		}
		return fmt.Errorf("HOST_KEY_CHANGED: %w", err)
	}, nil
}

func (m *TunnelManager) pendingFor(profileID string) (HostKeyInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pending, ok := m.pendingHostKey[profileID]
	return pending.info, ok
}

func authMethods(profile TunnelProfile, secret string) ([]ssh.AuthMethod, error) {
	if profile.AuthMode == AuthPassword {
		if secret == "" {
			return nil, fmt.Errorf("SECRET_REQUIRED: 请输入 SSH 密码")
		}
		return []ssh.AuthMethod{
			ssh.Password(secret),
			ssh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for index := range answers {
					answers[index] = secret
				}
				return answers, nil
			}),
		}, nil
	}
	path, err := expandHome(profile.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}
	var signer ssh.Signer
	if secret != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(secret))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, fmt.Errorf("SECRET_REQUIRED: 该私钥已加密，请输入私钥口令")
		}
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func expandHome(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func (t *runningTunnel) acceptLoop() {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			continue
		}
		go t.forward(local)
	}
}

func (t *runningTunnel) forward(local net.Conn) {
	defer local.Close()
	t.mu.RLock()
	client := t.client
	t.mu.RUnlock()
	if client == nil {
		return
	}
	remote, err := client.Dial("tcp", t.profile.remoteEndpoint())
	if err != nil {
		t.setMessage("远程目标连接失败: " + err.Error())
		return
	}
	defer remote.Close()
	t.active.Add(1)
	t.publish()
	defer func() {
		t.active.Add(-1)
		t.publish()
	}()
	errorsChannel := make(chan error, 2)
	go func() { _, err := io.Copy(remote, local); errorsChannel <- err }()
	go func() { _, err := io.Copy(local, remote); errorsChannel <- err }()
	<-errorsChannel
	local.Close()
	remote.Close()
	<-errorsChannel
}

func (t *runningTunnel) monitor(client *ssh.Client) {
	go t.keepAlive(client)
	err := client.Wait()
	if t.ctx.Err() != nil {
		return
	}
	t.mu.Lock()
	if t.client == client {
		t.client = nil
	}
	t.mu.Unlock()
	if !t.profile.AutoReconnect {
		t.fail("SSH 连接已断开: " + friendlySSHError(err))
		return
	}
	delay := 2 * time.Second
	for {
		t.setState("reconnecting", fmt.Sprintf("连接已断开，%d 秒后重试", int(delay.Seconds())))
		select {
		case <-time.After(delay):
		case <-t.ctx.Done():
			return
		}
		newClient, connectErr := t.manager.connect(t.ctx, t.profile, t.secret)
		if connectErr == nil {
			t.mu.Lock()
			t.client = newClient
			t.status.ConnectedAt = nowRFC3339()
			t.mu.Unlock()
			t.setState("running", "SSH 隧道已重新连接")
			go t.monitor(newClient)
			return
		}
		if strings.Contains(connectErr.Error(), "HOST_KEY_CHANGED") {
			t.fail("服务器主机密钥发生变化，已停止自动重连")
			return
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

func (t *runningTunnel) keepAlive(client *ssh.Client) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				failures++
				if failures >= 3 {
					client.Close()
					return
				}
			} else {
				failures = 0
			}
		}
	}
}

func (t *runningTunnel) fail(message string) {
	t.setState("error", message)
	t.manager.mu.Lock()
	if t.manager.tunnels[t.profile.ID] == t {
		delete(t.manager.tunnels, t.profile.ID)
	}
	t.manager.mu.Unlock()
	t.stop()
}

func (t *runningTunnel) stop() {
	t.stopOnce.Do(func() {
		t.cancel()
		t.listener.Close()
		t.mu.Lock()
		if t.client != nil {
			t.client.Close()
			t.client = nil
		}
		t.secret = ""
		t.mu.Unlock()
	})
}

func (t *runningTunnel) setState(state, message string) {
	t.mu.Lock()
	t.status.State = state
	t.status.Message = message
	t.mu.Unlock()
	t.publish()
}

func (t *runningTunnel) setMessage(message string) {
	t.mu.Lock()
	if t.status.State == "running" {
		t.status.Message = message
	}
	t.mu.Unlock()
	t.publish()
}

func (t *runningTunnel) statusCopy() *TunnelStatus {
	t.mu.RLock()
	copy := t.status
	t.mu.RUnlock()
	copy.ActiveConnections = int(t.active.Load())
	return &copy
}

func (t *runningTunnel) publish() {
	t.manager.emitStatus(*t.statusCopy())
}

func (m *TunnelManager) emitStatus(status TunnelStatus) {
	m.mu.Lock()
	m.statuses[status.ProfileID] = status
	m.mu.Unlock()
	if m.emit != nil {
		m.emit(status)
	}
}

func failedResult(code, message string) OperationResult {
	return OperationResult{OK: false, Code: code, Message: message}
}

func friendlySSHError(err error) string {
	if err == nil {
		return "连接已关闭"
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "SECRET_REQUIRED"):
		parts := strings.SplitN(message, "SECRET_REQUIRED:", 2)
		return strings.TrimSpace(parts[len(parts)-1])
	case strings.Contains(message, "unable to authenticate"):
		return "认证失败，请检查用户名、密码或私钥"
	case strings.Contains(message, "connection refused"):
		return "SSH 端口拒绝连接"
	case strings.Contains(message, "i/o timeout"):
		return "连接 SSH 服务器超时"
	case strings.Contains(message, "no such host"):
		return "无法解析 SSH 主机名"
	case strings.Contains(message, "HOST_KEY_CHANGED"):
		return "服务器主机密钥与已保存记录不一致，连接已被阻止"
	default:
		return message
	}
}
