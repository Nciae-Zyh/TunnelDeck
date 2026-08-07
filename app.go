package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	mu         sync.RWMutex
	store      *ConfigStore
	secrets    SecretStore
	manager    *TunnelManager
	profiles   []TunnelProfile
	startupErr error
}

func NewApp() *App {
	store, err := NewConfigStore()
	app := &App{store: store, secrets: SystemSecretStore{}, startupErr: err}
	if err == nil {
		profiles, loadErr := store.Load()
		if loadErr != nil {
			app.startupErr = loadErr
		} else {
			app.profiles = profiles
		}
	}
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initializeManager(func(status TunnelStatus) {
		runtime.EventsEmit(a.ctx, "tunnel:status", status)
	})
}

func (a *App) initializeManager(emit func(TunnelStatus)) {
	if a.store != nil {
		a.manager = NewTunnelManager(a.store.knownHostsPath, emit)
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.manager != nil {
		a.manager.StopAll()
	}
}

func (a *App) Bootstrap() BootstrapData {
	a.mu.RLock()
	profiles := append([]TunnelProfile(nil), a.profiles...)
	a.mu.RUnlock()
	views := make([]ProfileView, 0, len(profiles))
	statuses := make([]TunnelStatus, 0, len(profiles))
	for _, profile := range profiles {
		views = append(views, ProfileView{TunnelProfile: profile, HasStoredSecret: profile.RememberSecret})
		if a.manager != nil {
			statuses = append(statuses, a.manager.Status(profile))
		} else {
			statuses = append(statuses, stoppedStatus(profile))
		}
	}
	result := BootstrapData{Profiles: views, Statuses: statuses}
	if a.store != nil {
		result.ConfigPath = a.store.configPath
		result.KnownHostsPath = a.store.knownHostsPath
	}
	if a.startupErr != nil {
		result.StartupError = a.startupErr.Error()
	}
	return result
}

func (a *App) SaveProfile(request SaveProfileRequest) OperationResult {
	if a.store == nil {
		return failedResult("STORE_UNAVAILABLE", "配置存储不可用")
	}
	profile := request.Profile
	profile.applyDefaults()
	profile.Name = strings.TrimSpace(profile.Name)
	profile.SSHHost = strings.TrimSpace(profile.SSHHost)
	profile.Username = strings.TrimSpace(profile.Username)
	profile.LocalBind = strings.TrimSpace(profile.LocalBind)
	profile.RemoteHost = strings.TrimSpace(profile.RemoteHost)
	profile.PrivateKeyPath = strings.TrimSpace(profile.PrivateKeyPath)
	if err := validateProfile(profile); err != nil {
		return failedResult("VALIDATION_FAILED", err.Error())
	}
	if profile.ID != "" && a.manager != nil && a.manager.IsRunning(profile.ID) {
		return failedResult("PROFILE_RUNNING", "请先停止隧道，再修改配置")
	}

	a.mu.Lock()
	index := -1
	for candidate := range a.profiles {
		if a.profiles[candidate].ID == profile.ID && profile.ID != "" {
			index = candidate
			break
		}
	}
	if profile.ID == "" {
		id, err := newProfileID()
		if err != nil {
			a.mu.Unlock()
			return failedResult("ID_GENERATION_FAILED", err.Error())
		}
		profile.ID = id
		profile.CreatedAt = nowRFC3339()
	}
	profile.UpdatedAt = nowRFC3339()
	profilesCopy := append([]TunnelProfile(nil), a.profiles...)
	if index >= 0 {
		profile.CreatedAt = a.profiles[index].CreatedAt
		profilesCopy[index] = profile
	} else {
		profilesCopy = append(profilesCopy, profile)
	}
	if err := a.store.Save(profilesCopy); err != nil {
		a.mu.Unlock()
		return failedResult("SAVE_FAILED", err.Error())
	}
	a.profiles = profilesCopy
	a.mu.Unlock()
	if profile.RememberSecret && request.Secret != "" {
		if err := a.secrets.Set(profile.ID, request.Secret); err != nil {
			return OperationResult{OK: false, Code: "KEYRING_SAVE_FAILED", Message: "配置已保存，但系统钥匙串写入失败: " + err.Error(), Profile: &profile}
		}
	} else if !profile.RememberSecret {
		if err := a.secrets.Delete(profile.ID); err != nil {
			return OperationResult{OK: false, Code: "KEYRING_DELETE_FAILED", Message: "配置已保存，但旧凭据删除失败: " + err.Error(), Profile: &profile}
		}
	}
	return OperationResult{OK: true, Message: "配置已保存", Profile: &profile}
}

func (a *App) DeleteProfile(profileID string) OperationResult {
	profile, ok := a.findProfile(profileID)
	if !ok {
		return failedResult("NOT_FOUND", "配置不存在")
	}
	a.mu.Lock()
	profiles := make([]TunnelProfile, 0, len(a.profiles)-1)
	for _, item := range a.profiles {
		if item.ID != profileID {
			profiles = append(profiles, item)
		}
	}
	profilesCopy := append([]TunnelProfile(nil), profiles...)
	if err := a.store.Save(profilesCopy); err != nil {
		a.mu.Unlock()
		return failedResult("DELETE_FAILED", err.Error())
	}
	a.profiles = profiles
	a.mu.Unlock()
	if a.manager != nil {
		a.manager.Stop(profile)
	}
	if err := a.secrets.Delete(profileID); err != nil {
		return failedResult("KEYRING_DELETE_FAILED", "配置已删除，但钥匙串凭据删除失败: "+err.Error())
	}
	return OperationResult{OK: true, Message: "配置已删除"}
}

func (a *App) StartTunnel(request StartTunnelRequest) OperationResult {
	profile, ok := a.findProfile(request.ProfileID)
	if !ok {
		return failedResult("NOT_FOUND", "请先保存配置")
	}
	secret := request.Secret
	if secret == "" && profile.RememberSecret {
		stored, err := a.secrets.Get(profile.ID)
		if err == nil {
			secret = stored
		}
	}
	if profile.AuthMode == AuthPassword && secret == "" {
		return failedResult("SECRET_REQUIRED", "请输入 SSH 密码")
	}
	if a.manager == nil {
		return failedResult("MANAGER_UNAVAILABLE", "隧道管理器尚未初始化")
	}
	return a.manager.Start(profile, secret)
}

func (a *App) StopTunnel(profileID string) OperationResult {
	profile, ok := a.findProfile(profileID)
	if !ok {
		return failedResult("NOT_FOUND", "配置不存在")
	}
	if a.manager == nil {
		return failedResult("MANAGER_UNAVAILABLE", "隧道管理器尚未初始化")
	}
	return a.manager.Stop(profile)
}

func (a *App) BrowserURL(profileID string) OperationResult {
	profile, ok := a.findProfile(profileID)
	if !ok {
		return failedResult("NOT_FOUND", "配置不存在")
	}
	if a.manager == nil || !a.manager.IsRunning(profileID) {
		return failedResult("TUNNEL_NOT_RUNNING", "请先启动隧道")
	}
	target, err := profile.browserURL()
	if err != nil {
		return failedResult("NOT_A_WEB_SERVICE", err.Error())
	}
	return OperationResult{OK: true, Message: "网页地址已就绪", URL: target}
}

func (a *App) OpenProfileInBrowser(profileID string) OperationResult {
	result := a.BrowserURL(profileID)
	if !result.OK {
		return result
	}
	if a.ctx == nil {
		return failedResult("BROWSER_UNAVAILABLE", "当前运行模式不能直接打开浏览器")
	}
	runtime.BrowserOpenURL(a.ctx, result.URL)
	result.Message = "已使用默认浏览器打开"
	return result
}

func (a *App) RegisterNativeHost(extensionID string) NativeHostRegistrationResult {
	extensionID = strings.TrimSpace(extensionID)
	return registerNativeMessagingHost(extensionID)
}

func (a *App) TrustHost(profileID string) OperationResult {
	profile, ok := a.findProfile(profileID)
	if !ok {
		return failedResult("NOT_FOUND", "配置不存在")
	}
	if a.manager == nil {
		return failedResult("MANAGER_UNAVAILABLE", "隧道管理器尚未初始化")
	}
	return a.manager.TrustHost(profile)
}

func (a *App) ParseSSHCommand(command string) ParseCommandResult {
	profile, err := parseSSHCommand(command)
	if err != nil {
		return ParseCommandResult{OK: false, Code: "PARSE_FAILED", Message: err.Error()}
	}
	return ParseCommandResult{OK: true, Profile: &profile}
}

func (a *App) BuildSSHCommand(profile TunnelProfile) string {
	return buildSSHCommand(profile)
}

func (a *App) PickPrivateKey() OperationResult {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 SSH 私钥",
		Filters: []runtime.FileFilter{
			{DisplayName: "SSH 私钥", Pattern: "id_*;*.pem;*.key"},
			{DisplayName: "全部文件", Pattern: "*"},
		},
	})
	if err != nil {
		return failedResult("DIALOG_FAILED", err.Error())
	}
	if path == "" {
		return failedResult("CANCELLED", "未选择文件")
	}
	profile := TunnelProfile{PrivateKeyPath: path}
	return OperationResult{OK: true, Profile: &profile}
}

func (a *App) CopyText(value string) OperationResult {
	if err := runtime.ClipboardSetText(a.ctx, value); err != nil {
		return failedResult("CLIPBOARD_FAILED", err.Error())
	}
	return OperationResult{OK: true, Message: "已复制"}
}

func (a *App) findProfile(profileID string) (TunnelProfile, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, profile := range a.profiles {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return TunnelProfile{}, false
}

func (a *App) ExportSafeConfig() string {
	a.mu.RLock()
	profiles := append([]TunnelProfile(nil), a.profiles...)
	a.mu.RUnlock()
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	lines := []string{"TunnelDeck 配置摘要（不含密码或私钥内容）"}
	for _, profile := range profiles {
		lines = append(lines, fmt.Sprintf("- %s: %s → %s via %s@%s", profile.Name, profile.localEndpoint(), profile.remoteEndpoint(), profile.Username, profile.sshEndpoint()))
	}
	return strings.Join(lines, "\n")
}
