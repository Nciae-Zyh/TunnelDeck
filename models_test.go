package main

import (
	"strings"
	"testing"
)

func validProfile() TunnelProfile {
	return TunnelProfile{
		ID:            "test-profile",
		Name:          "Local web",
		SSHHost:       "ssh.example.com",
		SSHPort:       22,
		Username:      "root",
		LocalBind:     "127.0.0.1",
		LocalPort:     9108,
		RemoteHost:    "127.0.0.1",
		RemotePort:    9108,
		AuthMode:      AuthPassword,
		AutoReconnect: true,
	}
}

func TestValidateProfile(t *testing.T) {
	profile := validProfile()
	if err := validateProfile(profile); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	profile.LocalBind = "0.0.0.0"
	if err := validateProfile(profile); err != nil {
		t.Fatalf("explicit broad bind should remain supported: %v", err)
	}
	profile.SSHHost = "host; command"
	if err := validateProfile(profile); err == nil {
		t.Fatal("host with whitespace should be rejected")
	}
}

func TestValidateProfileRejectsInvalidWebScheme(t *testing.T) {
	profile := validProfile()
	profile.WebService = true
	profile.WebScheme = "file"
	if err := validateProfile(profile); err == nil {
		t.Fatal("invalid browser scheme should be rejected")
	}
}

func TestBrowserURLUsesLoopbackForWildcardBindings(t *testing.T) {
	tests := []struct {
		name string
		bind string
		want string
	}{
		{name: "IPv4 wildcard", bind: "0.0.0.0", want: "https://127.0.0.1:9108/"},
		{name: "IPv6 wildcard", bind: "::", want: "https://[::1]:9108/"},
		{name: "explicit loopback", bind: "127.0.0.1", want: "https://127.0.0.1:9108/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := validProfile()
			profile.LocalBind = test.bind
			profile.WebService = true
			profile.WebScheme = "https"
			got, err := profile.browserURL()
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("browserURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBrowserURLRequiresExplicitWebServiceOptIn(t *testing.T) {
	profile := validProfile()
	if _, err := profile.browserURL(); err == nil {
		t.Fatal("non-web profile should not produce a browser URL")
	}
}

func TestFriendlySSHErrorDoesNotEchoSecretMarker(t *testing.T) {
	message := friendlySSHError(assertError("SECRET_REQUIRED: 请输入 SSH 密码"))
	if strings.Contains(message, "SECRET_REQUIRED") || message != "请输入 SSH 密码" {
		t.Fatalf("unexpected friendly error: %q", message)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
