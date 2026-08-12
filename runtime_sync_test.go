package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

type testSecretStore struct{}

func (testSecretStore) Get(string) (string, error) { return "", errors.New("not found") }
func (testSecretStore) Set(string, string) error   { return nil }
func (testSecretStore) Delete(string) error        { return nil }

func TestRuntimeCoordinatorSharesProfilesAndTunnelState(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{
		dir:            dir,
		configPath:     filepath.Join(dir, "profiles.json"),
		knownHostsPath: filepath.Join(dir, "known_hosts"),
	}
	owner := newTestApp(store)
	client := newTestApp(store)
	defer owner.shutdown(context.Background())
	defer client.shutdown(context.Background())

	if err := owner.coordinateRuntime(); err != nil {
		t.Fatal(err)
	}
	if err := client.coordinateRuntime(); err != nil {
		t.Fatal(err)
	}
	if owner.coordinator == nil || client.remote == nil {
		t.Fatalf("expected one owner and one client, owner=%v client=%v", owner.coordinator != nil, client.remote != nil)
	}

	target, closeTarget := startTestTarget(t)
	defer closeTarget()
	sshAddress, closeSSH := startTestSSHServer(t, "sync-test-password")
	defer closeSSH()
	sshHost, sshPortText, err := net.SplitHostPort(sshAddress)
	if err != nil {
		t.Fatal(err)
	}
	sshPort, _ := strconv.Atoi(sshPortText)
	targetHost, targetPortText, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	targetPort, _ := strconv.Atoi(targetPortText)

	profile := validProfile()
	profile.ID = ""
	profile.SSHHost = sshHost
	profile.SSHPort = sshPort
	profile.LocalPort = freeTCPPort(t)
	profile.RemoteHost = targetHost
	profile.RemotePort = targetPort
	saved := client.SaveProfile(SaveProfileRequest{Profile: profile})
	if !saved.OK || saved.Profile == nil {
		t.Fatalf("client SaveProfile failed: %#v", saved)
	}
	profile = *saved.Profile
	if got := owner.Bootstrap().Profiles; len(got) != 1 || got[0].ID != profile.ID {
		t.Fatalf("owner did not observe client profile: %#v", got)
	}

	first := client.StartTunnel(StartTunnelRequest{ProfileID: profile.ID, Secret: "sync-test-password"})
	if first.Code != "HOST_KEY_UNKNOWN" {
		t.Fatalf("first connection should require host trust, got %#v", first)
	}
	assertRuntimeState(t, owner.Bootstrap(), profile.ID, "host-key-required")
	assertRuntimeState(t, client.Bootstrap(), profile.ID, "host-key-required")
	if trusted := owner.TrustHost(profile.ID); !trusted.OK {
		t.Fatalf("owner could not trust client request: %#v", trusted)
	}
	if started := client.StartTunnel(StartTunnelRequest{ProfileID: profile.ID, Secret: "sync-test-password"}); !started.OK {
		t.Fatalf("client could not start shared tunnel: %#v", started)
	}
	assertRuntimeState(t, owner.Bootstrap(), profile.ID, "running")
	assertRuntimeState(t, client.Bootstrap(), profile.ID, "running")
	if stopped := owner.StopTunnel(profile.ID); !stopped.OK {
		t.Fatalf("owner could not stop client tunnel: %#v", stopped)
	}
	assertRuntimeState(t, client.Bootstrap(), profile.ID, "stopped")

	stale := profile
	profile.Name = "Updated elsewhere"
	updated := owner.SaveProfile(SaveProfileRequest{Profile: profile})
	if !updated.OK || updated.Profile == nil {
		t.Fatalf("owner update failed: %#v", updated)
	}
	stale.Name = "Stale overwrite"
	conflict := client.SaveProfile(SaveProfileRequest{Profile: stale})
	if conflict.OK || conflict.Code != "PROFILE_CONFLICT" {
		t.Fatalf("stale update should be rejected: %#v", conflict)
	}

	descriptor, err := readRuntimeDescriptor(filepath.Join(dir, runtimeDescriptorName))
	if err != nil {
		t.Fatal(err)
	}
	descriptor.Token = "invalid-token"
	if err := newRuntimeClient(descriptor).ping(); err == nil {
		t.Fatal("runtime coordinator accepted an invalid token")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, runtimeDescriptorName))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("runtime descriptor permissions = %o, want 600", info.Mode().Perm())
		}
	}

	owner.shutdown(context.Background())
	data := client.Bootstrap()
	if data.StartupError != "" {
		t.Fatalf("client did not recover after owner exit: %s", data.StartupError)
	}
	if client.coordinator == nil || client.remote != nil {
		t.Fatal("client should become the new runtime owner")
	}
	if len(data.Profiles) != 1 || data.Profiles[0].Name != "Updated elsewhere" {
		t.Fatalf("failover did not reload the latest profile: %#v", data.Profiles)
	}
}

func newTestApp(store *ConfigStore) *App {
	profiles, _ := store.Load()
	app := &App{store: store, secrets: testSecretStore{}, profiles: profiles}
	app.initializeManager(nil)
	return app
}

func assertRuntimeState(t *testing.T, data BootstrapData, profileID, want string) {
	t.Helper()
	for _, status := range data.Statuses {
		if status.ProfileID == profileID {
			if status.State != want {
				t.Fatalf("status = %q, want %q", status.State, want)
			}
			return
		}
	}
	t.Fatalf("missing status for profile %s", profileID)
}
