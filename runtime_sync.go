package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	runtimeProtocolVersion = 1
	runtimeDescriptorName  = "runtime.json"
	runtimeLockName        = "runtime.lock"
)

type runtimeDescriptor struct {
	Version int    `json:"version"`
	Address string `json:"address"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
}

type runtimeCoordinator struct {
	descriptorPath string
	token          string
	lock           *flock.Flock
	server         *http.Server
	listener       net.Listener
}

type runtimeClient struct {
	address string
	token   string
	http    *http.Client
}

func (a *App) coordinateRuntime() error {
	a.coordinatorMu.Lock()
	defer a.coordinatorMu.Unlock()
	if a.coordinator != nil || a.remote != nil || a.store == nil {
		return nil
	}

	lockPath := filepath.Join(a.store.dir, runtimeLockName)
	descriptorPath := filepath.Join(a.store.dir, runtimeDescriptorName)
	deadline := time.Now().Add(2 * time.Second)
	for {
		fileLock := flock.New(lockPath)
		locked, err := fileLock.TryLock()
		if err != nil {
			_ = fileLock.Close()
			return fmt.Errorf("获取本地运行锁失败: %w", err)
		}
		if locked {
			if err := a.startRuntimeCoordinator(fileLock, descriptorPath); err != nil {
				_ = fileLock.Close()
				return err
			}
			return nil
		}
		_ = fileLock.Close()

		descriptor, err := readRuntimeDescriptor(descriptorPath)
		if err == nil {
			client := newRuntimeClient(descriptor)
			if pingErr := client.ping(); pingErr == nil {
				a.useRemoteRuntime(client)
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("另一个 TunnelDeck 进程正在初始化，请稍后重试")
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func (a *App) startRuntimeCoordinator(fileLock *flock.Flock, descriptorPath string) error {
	if err := a.reloadProfiles(); err != nil {
		return err
	}
	a.ensureLocalManager()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动本地同步服务失败: %w", err)
	}
	token, err := newRuntimeToken()
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("生成本地同步令牌失败: %w", err)
	}
	descriptor := runtimeDescriptor{
		Version: runtimeProtocolVersion,
		Address: "http://" + listener.Addr().String(),
		Token:   token,
		PID:     os.Getpid(),
	}
	if err := writeRuntimeDescriptor(descriptorPath, descriptor); err != nil {
		_ = listener.Close()
		return err
	}

	coordinator := &runtimeCoordinator{
		descriptorPath: descriptorPath,
		token:          token,
		lock:           fileLock,
		listener:       listener,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", a.runtimeRPCHandler(token))
	coordinator.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       35 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	a.coordinator = coordinator
	go func() {
		_ = coordinator.server.Serve(listener)
	}()
	return nil
}

func (a *App) runtimeRPCHandler(token string) http.HandlerFunc {
	host := &nativeHost{app: a}
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authorization := request.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(authorization, "Bearer ")
		if len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		defer request.Body.Close()
		payload, err := io.ReadAll(io.LimitReader(request.Body, nativeMaxMessageBytes+1))
		if err != nil || len(payload) > nativeMaxMessageBytes {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		var message nativeRequest
		if err := json.Unmarshal(payload, &message); err != nil {
			http.Error(writer, "invalid json", http.StatusBadRequest)
			return
		}
		result, responseErr := host.handle(message)
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(nativeResponse{
			ID: message.ID, OK: responseErr == nil, Result: result, Error: responseErr,
		})
	}
}

func (a *App) useRemoteRuntime(client *runtimeClient) {
	a.mu.Lock()
	manager := a.manager
	a.manager = nil
	a.mu.Unlock()
	if manager != nil {
		manager.StopAll()
	}
	a.remote = client
}

func (a *App) reloadProfiles() error {
	profiles, err := a.store.Load()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.profiles = profiles
	a.mu.Unlock()
	return nil
}

func (a *App) ensureLocalManager() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.manager == nil && a.store != nil {
		a.manager = NewTunnelManager(a.store.knownHostsPath, a.managerEmit)
	}
}

func (a *App) closeRuntimeCoordinator() {
	a.coordinatorMu.Lock()
	coordinator := a.coordinator
	a.coordinator = nil
	a.remote = nil
	a.coordinatorMu.Unlock()
	if coordinator == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = coordinator.server.Shutdown(ctx)
	_ = coordinator.listener.Close()
	if descriptor, err := readRuntimeDescriptor(coordinator.descriptorPath); err == nil && descriptor.Token == coordinator.token {
		_ = os.Remove(coordinator.descriptorPath)
	}
	_ = coordinator.lock.Close()
}

func (a *App) invokeRemote(method string, params any, target any) (bool, *nativeError) {
	a.coordinatorMu.Lock()
	client := a.remote
	coordinator := a.coordinator
	a.coordinatorMu.Unlock()
	a.mu.RLock()
	runtimeReady := a.runtimeReady
	storeAvailable := a.store != nil
	a.mu.RUnlock()
	if client == nil && coordinator == nil && runtimeReady && storeAvailable {
		if err := a.coordinateRuntime(); err != nil {
			return true, &nativeError{Code: "RUNTIME_SYNC_FAILED", Message: err.Error()}
		}
		a.coordinatorMu.Lock()
		client = a.remote
		a.coordinatorMu.Unlock()
	}
	if client == nil {
		return false, nil
	}

	response, err := client.call(method, params)
	if err != nil {
		a.coordinatorMu.Lock()
		if a.remote == client {
			a.remote = nil
		}
		a.coordinatorMu.Unlock()
		if coordinateErr := a.coordinateRuntime(); coordinateErr != nil {
			return true, &nativeError{Code: "RUNTIME_SYNC_FAILED", Message: coordinateErr.Error()}
		}
		a.coordinatorMu.Lock()
		client = a.remote
		a.coordinatorMu.Unlock()
		if client == nil {
			return false, nil
		}
		response, err = client.call(method, params)
		if err != nil {
			return true, &nativeError{Code: "RUNTIME_SYNC_FAILED", Message: "本地同步服务暂时不可用: " + err.Error()}
		}
	}
	if response.Error != nil {
		return true, response.Error
	}
	if !response.OK {
		return true, &nativeError{Code: "RUNTIME_REQUEST_FAILED", Message: "本地同步请求失败"}
	}
	if target != nil {
		payload, err := json.Marshal(response.Result)
		if err != nil {
			return true, &nativeError{Code: "RUNTIME_RESPONSE_INVALID", Message: err.Error()}
		}
		if err := json.Unmarshal(payload, target); err != nil {
			return true, &nativeError{Code: "RUNTIME_RESPONSE_INVALID", Message: err.Error()}
		}
	}
	return true, nil
}

func (a *App) remoteOperation(method string, params any) (OperationResult, bool) {
	var result OperationResult
	handled, responseErr := a.invokeRemote(method, params, &result)
	if !handled {
		return OperationResult{}, false
	}
	if responseErr == nil {
		return result, true
	}
	if responseErr.Data != nil {
		if payload, err := json.Marshal(responseErr.Data); err == nil {
			_ = json.Unmarshal(payload, &result)
		}
	}
	result.OK = false
	result.Code = responseErr.Code
	result.Message = responseErr.Message
	return result, true
}

func newRuntimeClient(descriptor runtimeDescriptor) *runtimeClient {
	transport := &http.Transport{Proxy: nil}
	return &runtimeClient{
		address: strings.TrimRight(descriptor.Address, "/"),
		token:   descriptor.Token,
		http:    &http.Client{Timeout: 35 * time.Second, Transport: transport},
	}
}

func (c *runtimeClient) ping() error {
	response, err := c.call("ping", nil)
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New("本地同步服务握手失败")
	}
	return nil
}

func (c *runtimeClient) call(method string, params any) (nativeResponse, error) {
	id, err := newRuntimeToken()
	if err != nil {
		return nativeResponse{}, err
	}
	var rawParams json.RawMessage
	if params != nil {
		payload, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return nativeResponse{}, marshalErr
		}
		rawParams = payload
	}
	payload, err := json.Marshal(nativeRequest{ID: id, Method: method, Params: rawParams})
	if err != nil {
		return nativeResponse{}, err
	}
	request, err := http.NewRequest(http.MethodPost, c.address+"/rpc", bytes.NewReader(payload))
	if err != nil {
		return nativeResponse{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nativeResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nativeResponse{}, fmt.Errorf("本地同步服务返回 HTTP %d", response.StatusCode)
	}
	var message nativeResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, nativeMaxMessageBytes+1))
	if err := decoder.Decode(&message); err != nil {
		return nativeResponse{}, err
	}
	if message.ID != id {
		return nativeResponse{}, errors.New("本地同步服务返回了不匹配的请求 ID")
	}
	return message, nil
}

func readRuntimeDescriptor(path string) (runtimeDescriptor, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return runtimeDescriptor{}, err
	}
	var descriptor runtimeDescriptor
	if err := json.Unmarshal(payload, &descriptor); err != nil {
		return runtimeDescriptor{}, err
	}
	address, parseErr := url.Parse(descriptor.Address)
	if parseErr != nil || descriptor.Version != runtimeProtocolVersion || descriptor.Token == "" ||
		address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.Port() == "" ||
		address.User != nil || address.Path != "" || address.RawQuery != "" || address.Fragment != "" {
		return runtimeDescriptor{}, errors.New("本地同步描述文件无效")
	}
	return descriptor, nil
}

func writeRuntimeDescriptor(path string, descriptor runtimeDescriptor) error {
	payload, err := json.Marshal(descriptor)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "runtime-*.tmp")
	if err != nil {
		return fmt.Errorf("创建本地同步描述文件失败: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(payload, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("保存本地同步描述文件失败: %w", err)
	}
	return nil
}

func newRuntimeToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
