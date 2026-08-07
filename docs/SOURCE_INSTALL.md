# 从源码安装 TunnelDeck

当前版本不提供未签名的桌面二进制、安装器或 Chrome 扩展 ZIP。CI 只验证各平台能够成功构建，不上传构建产物。用户在自己的机器上从公开源码构建并安装。

这种方式不等于第三方代码签名。安装前仍应检查仓库地址、版本标签、提交记录和脚本内容，不要运行来源不明的复制版脚本。

## macOS

支持当前机器的原生架构，包括 Apple Silicon 和 Intel。

安装前置工具：

```bash
xcode-select --install
brew install go node git
```

克隆并安装：

```bash
git clone https://github.com/Nciae-Zyh/TunnelDeck.git
cd TunnelDeck
git checkout v0.2.0
./scripts/install-from-source.sh
```

应用默认安装到 `~/Applications/TunnelDeck.app`。扩展构建到仓库内的 `extension/dist`。

随后：

1. 在 `chrome://extensions` 开启开发者模式。
2. 点击“加载已解压的扩展程序”，选择 `extension/dist`。
3. 复制 Chrome 显示的扩展 ID。
4. 启动 TunnelDeck，在“Chrome 浏览器集成”中粘贴 ID 并注册。
5. 重新加载 Chrome 扩展。

也可以在安装时直接传入已知 ID：

```bash
./scripts/install-from-source.sh --extension-id 你的扩展ID
```

## Windows

在 PowerShell 中安装前置工具：

```powershell
winget install --id Git.Git -e
winget install --id GoLang.Go -e
winget install --id OpenJS.NodeJS.LTS -e
```

重新打开 PowerShell，然后执行：

```powershell
git clone https://github.com/Nciae-Zyh/TunnelDeck.git
Set-Location TunnelDeck
git checkout v0.2.0
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\install-from-source.ps1
```

脚本会把本机编译的程序复制到 `%LOCALAPPDATA%\Programs\TunnelDeck\TunnelDeck.exe`，并创建当前用户的开始菜单快捷方式。Chrome 扩展位于 `extension\dist`，仍需用户在 `chrome://extensions` 中主动加载。

已知扩展 ID 时可以同时注册 Native Host：

```powershell
.\scripts\install-from-source.ps1 -ExtensionId 你的扩展ID
```

## Linux

Debian/Ubuntu 示例：

```bash
sudo apt update
sudo apt install -y git golang-go nodejs npm build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
git clone https://github.com/Nciae-Zyh/TunnelDeck.git
cd TunnelDeck
git checkout v0.2.0
./scripts/install-from-source.sh
```

桌面程序默认安装到 `~/.local/bin/TunnelDeck`。如果该目录不在 `PATH`，可以直接使用完整路径启动。

## 更新

进入原仓库，切换到新的可信标签，再重新运行安装脚本：

```bash
git fetch --tags
git checkout 新版本标签
./scripts/install-from-source.sh
```

Windows 使用相同流程并重新运行 `install-from-source.ps1`。
