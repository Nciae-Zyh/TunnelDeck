//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func registerNativeMessagingHost(extensionID string) NativeHostRegistrationResult {
	if err := validateChromeExtensionID(extensionID); err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "INVALID_EXTENSION_ID", Message: err.Error()}
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return NativeHostRegistrationResult{OK: false, Code: "CONFIG_UNAVAILABLE", Message: "LOCALAPPDATA 未设置，无法确定用户配置目录"}
	}
	binaryPath, err := currentExecutablePath()
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "BINARY_UNAVAILABLE", Message: err.Error()}
	}
	manifestPath := filepath.Join(localAppData, "TunnelDeck", "NativeMessagingHosts", nativeHostName+".json")
	if err := writeNativeHostManifest(manifestPath, binaryPath, extensionID); err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "REGISTRATION_FAILED", Message: err.Error()}
	}

	keyPath := `Software\Google\Chrome\NativeMessagingHosts\` + nativeHostName
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "REGISTRY_FAILED", Message: "创建 Chrome 注册表项失败: " + err.Error()}
	}
	defer key.Close()
	if err := key.SetStringValue("", manifestPath); err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "REGISTRY_FAILED", Message: "写入 Chrome 注册表项失败: " + err.Error()}
	}

	return NativeHostRegistrationResult{
		OK:           true,
		Message:      fmt.Sprintf("已为扩展 %s 注册 Chrome 服务，请重新加载扩展", extensionID),
		ExtensionID:  extensionID,
		ManifestPath: manifestPath,
		BinaryPath:   binaryPath,
	}
}
