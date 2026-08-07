//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func registerNativeMessagingHost(extensionID string) NativeHostRegistrationResult {
	if err := validateChromeExtensionID(extensionID); err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "INVALID_EXTENSION_ID", Message: err.Error()}
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "CONFIG_UNAVAILABLE", Message: "无法读取用户配置目录: " + err.Error()}
	}
	binaryPath, err := currentExecutablePath()
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "BINARY_UNAVAILABLE", Message: err.Error()}
	}
	manifestPath := filepath.Join(configDir, "google-chrome", "NativeMessagingHosts", nativeHostName+".json")
	if err := writeNativeHostManifest(manifestPath, binaryPath, extensionID); err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "REGISTRATION_FAILED", Message: err.Error()}
	}
	return NativeHostRegistrationResult{
		OK:           true,
		Message:      fmt.Sprintf("已为扩展 %s 注册 Chrome 服务，请重新加载扩展", extensionID),
		ExtensionID:  extensionID,
		ManifestPath: manifestPath,
		BinaryPath:   binaryPath,
	}
}
