//go:build darwin

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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "HOME_UNAVAILABLE", Message: "无法读取用户目录: " + err.Error()}
	}
	binaryPath, err := currentExecutablePath()
	if err != nil {
		return NativeHostRegistrationResult{OK: false, Code: "BINARY_UNAVAILABLE", Message: err.Error()}
	}
	manifestPath := filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts", nativeHostName+".json")
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
