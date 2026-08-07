package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

const (
	nativeHostName        = "com.tunneldeck.native"
	nativeProtocolVersion = 1
	nativeMaxMessageBytes = 1024 * 1024
)

type nativeRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type nativeResponse struct {
	ID     string       `json:"id"`
	OK     bool         `json:"ok"`
	Result any          `json:"result,omitempty"`
	Error  *nativeError `json:"error,omitempty"`
}

type nativeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type nativeEvent struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
}

type nativeWriter struct {
	mu     sync.Mutex
	output io.Writer
}

type nativeHost struct {
	app    *App
	writer *nativeWriter
}

func isNativeMessagingLaunch(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if strings.Contains(strings.ToLower(filepath.Base(args[0])), "nativehost") {
		return true
	}
	if len(args) < 2 {
		return false
	}
	return args[1] == "native-host" || strings.HasPrefix(args[1], "chrome-extension://")
}

func runNativeMessagingHost(input io.Reader, output io.Writer) error {
	return runNativeMessagingHostWithApp(NewApp(), input, output)
}

func runNativeMessagingHostWithApp(app *App, input io.Reader, output io.Writer) error {
	host := &nativeHost{app: app, writer: &nativeWriter{output: output}}
	host.app.initializeManager(func(status TunnelStatus) {
		_ = host.writer.write(nativeEvent{Event: "tunnel.status", Payload: status})
	})
	if host.app.manager != nil {
		defer host.app.manager.StopAll()
	}

	for {
		payload, err := readNativeMessage(input)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var request nativeRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			if writeErr := host.writer.write(nativeResponse{
				OK:    false,
				Error: &nativeError{Code: "INVALID_JSON", Message: "请求不是有效的 JSON"},
			}); writeErr != nil {
				return writeErr
			}
			continue
		}
		result, responseErr := host.handle(request)
		response := nativeResponse{ID: request.ID, OK: responseErr == nil, Result: result, Error: responseErr}
		if err := host.writer.write(response); err != nil {
			return err
		}
	}
}

func (h *nativeHost) handle(request nativeRequest) (any, *nativeError) {
	if strings.TrimSpace(request.ID) == "" {
		return nil, &nativeError{Code: "INVALID_REQUEST", Message: "请求缺少 id"}
	}
	switch request.Method {
	case "ping":
		return map[string]any{
			"host":            nativeHostName,
			"protocolVersion": nativeProtocolVersion,
		}, nil
	case "bootstrap":
		return h.app.Bootstrap(), nil
	case "saveProfile":
		var params SaveProfileRequest
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.SaveProfile(params))
	case "deleteProfile":
		var params struct {
			ProfileID string `json:"profileId"`
		}
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.DeleteProfile(params.ProfileID))
	case "startTunnel":
		var params StartTunnelRequest
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.StartTunnel(params))
	case "stopTunnel":
		var params struct {
			ProfileID string `json:"profileId"`
		}
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.StopTunnel(params.ProfileID))
	case "trustHost":
		var params struct {
			ProfileID string `json:"profileId"`
		}
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.TrustHost(params.ProfileID))
	case "parseSSHCommand":
		var params struct {
			Command string `json:"command"`
		}
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		result := h.app.ParseSSHCommand(params.Command)
		if !result.OK {
			return nil, &nativeError{Code: result.Code, Message: result.Message, Data: result}
		}
		return result, nil
	case "browserURL":
		var params struct {
			ProfileID string `json:"profileId"`
		}
		if err := decodeNativeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return nativeOperation(h.app.BrowserURL(params.ProfileID))
	default:
		return nil, &nativeError{Code: "METHOD_NOT_FOUND", Message: "不支持的方法: " + request.Method}
	}
}

func decodeNativeParams(payload json.RawMessage, target any) *nativeError {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return &nativeError{Code: "INVALID_PARAMS", Message: "请求参数无效: " + err.Error()}
	}
	return nil
}

func nativeOperation(result OperationResult) (any, *nativeError) {
	if result.OK {
		return result, nil
	}
	code := result.Code
	if code == "" {
		code = "OPERATION_FAILED"
	}
	return nil, &nativeError{Code: code, Message: result.Message, Data: result}
}

func readNativeMessage(reader io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(reader, binary.NativeEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, fmt.Errorf("native message cannot be empty")
	}
	if length > nativeMaxMessageBytes {
		return nil, fmt.Errorf("native message is too large: %d bytes", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (w *nativeWriter) write(message any) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(payload) > nativeMaxMessageBytes {
		return fmt.Errorf("native response is too large: %d bytes", len(payload))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := binary.Write(w.output, binary.NativeEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err = w.output.Write(payload)
	return err
}
