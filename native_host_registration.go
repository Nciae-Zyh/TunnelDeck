package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var chromeExtensionIDPattern = regexp.MustCompile(`^[a-p]{32}$`)

type NativeHostRegistrationResult struct {
	OK           bool   `json:"ok"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
	ExtensionID  string `json:"extensionId,omitempty"`
	ManifestPath string `json:"manifestPath,omitempty"`
	BinaryPath   string `json:"binaryPath,omitempty"`
}

type nativeHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func validateChromeExtensionID(extensionID string) error {
	if !chromeExtensionIDPattern.MatchString(extensionID) {
		return fmt.Errorf("扩展 ID 必须是 32 位小写字母，且只能使用 a 到 p")
	}
	return nil
}

func currentExecutablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("读取应用路径失败: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("解析应用路径失败: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析应用绝对路径失败: %w", err)
	}
	return path, nil
}

func writeNativeHostManifest(manifestPath, binaryPath, extensionID string) error {
	if err := validateChromeExtensionID(extensionID); err != nil {
		return err
	}
	if !filepath.IsAbs(binaryPath) {
		return fmt.Errorf("Native Host 可执行文件必须使用绝对路径")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("Native Host 可执行文件不可用: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("Native Host 路径不能是目录")
	}

	manifest := nativeHostManifest{
		Name:           nativeHostName,
		Description:    "TunnelDeck SSH tunnel controller",
		Path:           binaryPath,
		Type:           "stdio",
		AllowedOrigins: []string{"chrome-extension://" + extensionID + "/"},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("生成 Native Host 清单失败: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		return fmt.Errorf("创建 Native Host 目录失败: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return fmt.Errorf("写入 Native Host 清单失败: %w", err)
	}
	if err := os.Chmod(manifestPath, 0o600); err != nil {
		return fmt.Errorf("设置 Native Host 清单权限失败: %w", err)
	}
	return nil
}
