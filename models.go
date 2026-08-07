package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	AuthPassword   = "password"
	AuthPrivateKey = "private-key"
)

type TunnelProfile struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SSHHost        string `json:"sshHost"`
	SSHPort        int    `json:"sshPort"`
	Username       string `json:"username"`
	LocalBind      string `json:"localBind"`
	LocalPort      int    `json:"localPort"`
	RemoteHost     string `json:"remoteHost"`
	RemotePort     int    `json:"remotePort"`
	AuthMode       string `json:"authMode"`
	PrivateKeyPath string `json:"privateKeyPath,omitempty"`
	RememberSecret bool   `json:"rememberSecret"`
	AutoReconnect  bool   `json:"autoReconnect"`
	WebService     bool   `json:"webService"`
	WebScheme      string `json:"webScheme,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ProfileView struct {
	TunnelProfile
	HasStoredSecret bool `json:"hasStoredSecret"`
}

type SaveProfileRequest struct {
	Profile TunnelProfile `json:"profile"`
	Secret  string        `json:"secret"`
}

type StartTunnelRequest struct {
	ProfileID string `json:"profileId"`
	Secret    string `json:"secret"`
}

type HostKeyInfo struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
}

type TunnelStatus struct {
	ProfileID         string       `json:"profileId"`
	State             string       `json:"state"`
	Message           string       `json:"message"`
	LocalEndpoint     string       `json:"localEndpoint"`
	RemoteEndpoint    string       `json:"remoteEndpoint"`
	ActiveConnections int          `json:"activeConnections"`
	ConnectedAt       string       `json:"connectedAt,omitempty"`
	HostKey           *HostKeyInfo `json:"hostKey,omitempty"`
}

type OperationResult struct {
	OK      bool           `json:"ok"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Profile *TunnelProfile `json:"profile,omitempty"`
	Status  *TunnelStatus  `json:"status,omitempty"`
	HostKey *HostKeyInfo   `json:"hostKey,omitempty"`
	URL     string         `json:"url,omitempty"`
}

type BootstrapData struct {
	Profiles       []ProfileView  `json:"profiles"`
	Statuses       []TunnelStatus `json:"statuses"`
	ConfigPath     string         `json:"configPath"`
	KnownHostsPath string         `json:"knownHostsPath"`
	StartupError   string         `json:"startupError,omitempty"`
}

type ParseCommandResult struct {
	OK      bool           `json:"ok"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Profile *TunnelProfile `json:"profile,omitempty"`
}

func (p *TunnelProfile) applyDefaults() {
	if p.SSHPort == 0 {
		p.SSHPort = 22
	}
	if strings.TrimSpace(p.LocalBind) == "" {
		p.LocalBind = "127.0.0.1"
	}
	if strings.TrimSpace(p.RemoteHost) == "" {
		p.RemoteHost = "127.0.0.1"
	}
	if p.AuthMode == "" {
		p.AuthMode = AuthPassword
	}
	if p.Name == "" && p.SSHHost != "" {
		p.Name = p.SSHHost
	}
	if p.WebService && p.WebScheme == "" {
		p.WebScheme = "http"
	}
}

func validateProfile(p TunnelProfile) error {
	p.applyDefaults()
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("配置名称不能为空")
	}
	if err := validateHost("SSH 主机", p.SSHHost); err != nil {
		return err
	}
	if strings.TrimSpace(p.Username) == "" || strings.ContainsAny(p.Username, "\r\n\t ") {
		return fmt.Errorf("SSH 用户名不能为空且不能包含空白字符")
	}
	if err := validatePort("SSH 端口", p.SSHPort); err != nil {
		return err
	}
	if net.ParseIP(p.LocalBind) == nil && p.LocalBind != "localhost" {
		return fmt.Errorf("本地绑定地址必须是 IP 地址或 localhost")
	}
	if err := validatePort("本地端口", p.LocalPort); err != nil {
		return err
	}
	if err := validateHost("远程目标", p.RemoteHost); err != nil {
		return err
	}
	if err := validatePort("远程端口", p.RemotePort); err != nil {
		return err
	}
	if p.AuthMode != AuthPassword && p.AuthMode != AuthPrivateKey {
		return fmt.Errorf("认证方式无效")
	}
	if p.AuthMode == AuthPrivateKey && strings.TrimSpace(p.PrivateKeyPath) == "" {
		return fmt.Errorf("私钥认证需要选择私钥文件")
	}
	if p.WebService && p.WebScheme != "http" && p.WebScheme != "https" {
		return fmt.Errorf("网页协议只能是 http 或 https")
	}
	return nil
}

func validateHost(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s不能为空", label)
	}
	if strings.ContainsAny(value, "\r\n\t /@") {
		return fmt.Errorf("%s格式无效", label)
	}
	return nil
}

func validatePort(label string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s必须在 1 到 65535 之间", label)
	}
	return nil
}

func (p TunnelProfile) sshEndpoint() string {
	return net.JoinHostPort(p.SSHHost, fmt.Sprintf("%d", p.SSHPort))
}

func (p TunnelProfile) localEndpoint() string {
	return net.JoinHostPort(p.LocalBind, fmt.Sprintf("%d", p.LocalPort))
}

func (p TunnelProfile) remoteEndpoint() string {
	return net.JoinHostPort(p.RemoteHost, fmt.Sprintf("%d", p.RemotePort))
}

func (p TunnelProfile) browserURL() (string, error) {
	if !p.WebService {
		return "", fmt.Errorf("这个配置未标记为网页服务")
	}
	scheme := p.WebScheme
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("网页协议只能是 http 或 https")
	}
	host := p.LocalBind
	switch host {
	case "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return scheme + "://" + net.JoinHostPort(host, fmt.Sprintf("%d", p.LocalPort)) + "/", nil
}

func stoppedStatus(p TunnelProfile) TunnelStatus {
	return TunnelStatus{
		ProfileID:      p.ID,
		State:          "stopped",
		Message:        "隧道未启动",
		LocalEndpoint:  p.localEndpoint(),
		RemoteEndpoint: p.remoteEndpoint(),
	}
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
