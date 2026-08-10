package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateChromeExtensionID(t *testing.T) {
	valid := "ipmjdganppehhljijcdndfjjmjjpalbp"
	if err := validateChromeExtensionID(valid); err != nil {
		t.Fatalf("valid extension ID rejected: %v", err)
	}
	for _, invalid := range []string{"", "abc", "IPMJDGANPPEHHLJIJCDNDFJJMJJPALBP", "zpmjdganppehhljijcdndfjjmjjpalbp", valid + "a"} {
		if err := validateChromeExtensionID(invalid); err == nil {
			t.Fatalf("invalid extension ID accepted: %q", invalid)
		}
	}
}

func TestResolveChromeExtensionID(t *testing.T) {
	if got := resolveChromeExtensionID(""); got != officialChromeExtensionID {
		t.Fatalf("empty ID should resolve to the official store ID: %q", got)
	}
	if got := resolveChromeExtensionID("  " + officialChromeExtensionID + "  "); got != officialChromeExtensionID {
		t.Fatalf("ID should be trimmed: %q", got)
	}
	custom := "ipmjdganppehhljijcdndfjjmjjpalbp"
	if got := resolveChromeExtensionID(custom); got != custom {
		t.Fatalf("custom ID should be preserved: %q", got)
	}
}

func TestWriteNativeHostManifest(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "TunnelDeck")
	if err := os.WriteFile(binaryPath, []byte("test"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "native", nativeHostName+".json")
	extensionID := officialChromeExtensionID
	if err := writeNativeHostManifest(manifestPath, binaryPath, extensionID); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest nativeHostManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != nativeHostName || manifest.Path != binaryPath || manifest.Type != "stdio" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	wantOrigin := "chrome-extension://" + extensionID + "/"
	if len(manifest.AllowedOrigins) != 1 || manifest.AllowedOrigins[0] != wantOrigin {
		t.Fatalf("unexpected allowed origins: %#v", manifest.AllowedOrigins)
	}
}
