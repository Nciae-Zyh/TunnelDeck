# 从源码安装 TunnelDeck

TunnelDeck 当前不提供未签名的桌面二进制或桌面安装器。公开的一行安装命令会检查环境，从固定版本标签下载源码，并在用户电脑上构建和安装。Chrome 扩展可以单独提交 Chrome Web Store；开发者也可以继续加载本地构建目录。

这种方式不等于第三方代码签名。安装前仍应确认域名、仓库、版本标签和脚本内容，不要运行来源不明的复制版命令。希望先阅读脚本时，可以下载后再执行：

```bash
curl -fsSLo /tmp/tunneldeck-install.sh https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh
less /tmp/tunneldeck-install.sh
sh /tmp/tunneldeck-install.sh
```

## 自动依赖检查

macOS/Linux 只检测、不修改系统：

```bash
curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh -s -- --check
```

Windows PowerShell 只检测：

```powershell
$env:TUNNELDECK_CHECK_ONLY='1'; irm https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.ps1 | iex
Remove-Item Env:TUNNELDECK_CHECK_ONLY
```

安装器会报告：

- 操作系统与 CPU 架构；
- 下载工具与 SHA-256 校验工具；
- Go 1.25+；
- Node.js 20+ 和 npm；
- macOS Xcode Command Line Tools、Linux GTK3/WebKitGTK 4.1 或 Windows WebView2。

已有的合格 Go/Node 会直接复用。缺失时，安装器下载固定的 Go 1.26.5 和 Node.js 22.23.2 到 TunnelDeck 的用户数据目录，核对官方 SHA-256 后仅在当前安装进程使用，不写入用户或系统的全局 `PATH`。

## macOS

支持 Apple Silicon 和 Intel。先确保已经安装 Xcode Command Line Tools；缺失时脚本会打开 Apple 安装提示，并要求完成后重新运行。

```bash
curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh
```

没有 `curl` 时：

```bash
wget -qO- https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh
```

桌面应用默认安装到 `~/Applications/TunnelDeck.app`。在 macOS 首次启动本机编译但未经 Apple 公证的应用时，仍可能需要在“隐私与安全性”中确认打开。

## Linux

支持 AMD64 和 ARM64。GTK3、WebKitGTK 4.1 和编译工具是系统依赖；缺失时安装器会提示确认，并支持 apt、dnf、pacman 和 zypper。无人值守环境只有显式加入 `--yes` 才会安装系统包：

```bash
curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh -s -- --yes
```

桌面程序默认安装到 `~/.local/bin/TunnelDeck`。如果该目录不在 `PATH`，可使用完整路径启动。

## Windows

支持 Windows AMD64/x64。PowerShell 安装器会检查 WebView2，并在用户目录放置缺失的 Go/Node 工具链：

```powershell
irm https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.ps1 | iex
```

桌面程序默认安装到 `%LOCALAPPDATA%\Programs\TunnelDeck\TunnelDeck.exe`，并为当前用户创建开始菜单快捷方式。如果没有 WebView2，脚本会停止并给出 Microsoft Evergreen Runtime 的安装地址，不会静默运行未校验的第三方安装程序。

## 配合 Chrome Web Store

TunnelDeck 的正式 Chrome Web Store 扩展 ID 为 `jnfkjehpbkmfnidfcilehhkpbjjinmod`。macOS/Linux 默认在安装桌面端时同时注册该 ID：

```bash
curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.ps1 | iex
```

这会跳过本地扩展构建，只构建桌面端，并把 Native Messaging Host 的 `allowed_origins` 限制为该商店 ID。安装后重新加载或重新启动扩展即可。

商店扩展会在侧边栏启动时自动检测 Native Host。若安装尚未完成或注册信息失效，侧边栏会显示与本页相同的分系统安装命令；完成后点击“重新检测”即可，无需重新安装扩展。

桌面端与扩展可以同时打开。两者会连接同一个仅限本机访问的运行核心，因此配置变更、启动、停止和实时状态会自动同步；不需要手动复制配置，也不会同时建立两条占用相同端口的隧道。若两个窗口同时编辑同一配置，后保存的旧副本会被拒绝并提示重新选择，以保护较新的修改。

开发者模式加载时产生的 ID 不应当作正式商店 ID。需要注册其他商店项目时，仍可传 `--chrome-store-id` 或 `TUNNELDECK_CHROME_STORE_ID` 覆盖默认值；桌面端底部也支持手动修改 ID 后点击“注册 Chrome 服务”。

## 加载开发版扩展

不使用 Chrome Web Store 时，传入 `--extension-id`（Windows 使用 `TUNNELDECK_EXTENSION_ID`）会构建开发版扩展。源码缓存目录为：

- macOS/Linux：`${XDG_DATA_HOME:-$HOME/.local/share}/tunneldeck/src/v0.3.3/extension/dist`
- Windows：`%LOCALAPPDATA%\TunnelDeck\Source\v0.3.3\extension\dist`

然后：

1. 在 `chrome://extensions` 开启开发者模式；
2. 点击“加载已解压的扩展程序”，选择上面的 `extension/dist`；
3. 复制 Chrome 显示的扩展 ID；
4. 启动 TunnelDeck，在“Chrome 浏览器集成”中粘贴 ID 并注册；
5. 重新加载 Chrome 扩展。

已知开发版 ID 时，也可以一次完成构建和注册：

```bash
curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.3/install.sh | sh -s -- --extension-id 你的开发扩展ID
```

## 手动检出源码

不希望使用一行入口时，可以完整检出标签后运行仓库内脚本：

```bash
git clone https://github.com/Nciae-Zyh/TunnelDeck.git
cd TunnelDeck
git checkout v0.3.3
./scripts/install-from-source.sh
```

Windows：

```powershell
git clone https://github.com/Nciae-Zyh/TunnelDeck.git
Set-Location TunnelDeck
git checkout v0.3.3
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\install-from-source.ps1
```

## 目录与更新

一行安装器的默认目录：

- macOS/Linux 工具链：`${XDG_DATA_HOME:-$HOME/.local/share}/tunneldeck/toolchains`
- macOS/Linux 源码：`${XDG_DATA_HOME:-$HOME/.local/share}/tunneldeck/src/版本号`
- Windows 工具链：`%LOCALAPPDATA%\TunnelDeck\Toolchains`
- Windows 源码：`%LOCALAPPDATA%\TunnelDeck\Source\版本号`

更新时把命令中的固定标签替换为新版本即可。不同版本源码分目录缓存；现有桌面应用会在覆盖前备份或直接更新到固定安装位置。`--refresh-source` 可重新下载同一版本，已有源码会先移动到带时间戳的备份目录。
