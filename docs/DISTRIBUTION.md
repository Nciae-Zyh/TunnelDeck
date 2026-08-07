# TunnelDeck 分发指南

本文只说明 TunnelDeck 与 Chrome 扩展的技术分发流程，不包含个人身份材料或开发者账号注册资料。桌面端采用源码分发：CI 验证构建，但不上传未签名应用或桌面安装器。Chrome 扩展单独生成可提交商店的 ZIP。实际安装见 [源码安装指南](SOURCE_INSTALL.md)。

## 1. 发布 Chrome Web Store

### 准备扩展包

```bash
./scripts/package-chrome-extension.sh
```

产物写入 `artifacts/chrome-web-store/`。脚本会核对 package/manifest 版本、拒绝密钥和 source map 等不应发布的文件，并确认 ZIP 根目录直接包含 `manifest.json`。每次上传的新版本必须高于商店现有版本。

### 发布步骤

1. 在 Chrome Web Store Developer Dashboard 创建新项目。
2. 上传 `TunnelDeck-chrome-extension.zip`。
3. 填写商店详情、单一用途、隐私、分发范围和测试说明。
4. 明确说明扩展需要配套 TunnelDeck 桌面应用，并通过 Native Messaging 控制本机 SSH 隧道。
5. 首次版本在 Dashboard 提交审核，并使用审核通过后自动发布。

首次上传需要在 Dashboard 手动完成商店详情、隐私、分发范围和测试说明。正式扩展 ID 为 `jnfkjehpbkmfnidfcilehhkpbjjinmod`。首次版本发布后，推送 `v*` 标签会使用 GitHub OIDC、Google Workload Identity Federation 和 Chrome Web Store API V2 自动上传并提交审核；仓库不保存长期 Google 密钥。

自动发布使用 GitHub Environment `chrome-web-store-production`，需要以下 Environment Variables：

- `CWS_EXTENSION_ID`：Chrome Web Store 正式扩展 ID；
- `CWS_PUBLISHER_ID`：Developer Dashboard 的 Publisher ID；
- `GCP_WIF_PROVIDER`：Google Workload Identity Provider 完整资源名；
- `GCP_SERVICE_ACCOUNT`：已添加到 Chrome Web Store 发布者账号的服务账号邮箱。

工作流只在推送 `v*` 标签时发布，并先验证标签版本与 ZIP 内 `manifest.json` 一致。发布脚本会先读取商店状态；同一版本已经发布或提交审核时会安全退出，重复运行不会再次上传。新版本使用 `DEFAULT_PUBLISH`、`skipReview: false` 和 `blockOnWarnings: true` 提交：仍须经过 Google 审核，审核通过后自动上线；存在验证警告时工作流会失败。GitHub `id-token: write` 只用于换取短期 Google 访问令牌，不需要把服务账号 JSON、OAuth Client Secret 或刷新令牌保存到 GitHub。

建议的单一用途：`从 Chrome 侧边栏创建、启动和停止由本机 TunnelDeck 管理的 SSH 本地端口转发。`

当前扩展只需要：

- `nativeMessaging`：与本机 TunnelDeck 通信。
- `sidePanel`：提供连接管理界面。

扩展会在本地处理 SSH 地址、用户名、连接配置和认证信息。即使数据不上传服务器，也应在商店隐私表单中如实披露本地处理方式。认证信息不会写入 Chrome Storage；用户选择“记住凭据”时由桌面应用写入操作系统安全存储。

### 正式扩展 ID

TunnelDeck 的正式 Chrome Web Store 扩展 ID 是 `jnfkjehpbkmfnidfcilehhkpbjjinmod`。源码安装器和桌面端会默认使用该 ID 注册本机服务；开发构建仍可以手动改成自己的扩展 ID。

开发者模式加载的扩展 ID 可能与商店 ID 不同。ID 变化后必须重新注册，因为 Native Host 的 `allowed_origins` 不允许通配符。

## 2. Native Messaging 注册

桌面端注册时使用当前运行的 TunnelDeck 可执行文件路径，因此应先把应用安装到固定位置。

### macOS

1. 使用源码安装器把 `TunnelDeck.app` 安装到固定位置，默认是 `~/Applications/TunnelDeck.app`。
2. 启动应用，在“Chrome 浏览器集成”中填写扩展 ID。
3. TunnelDeck 写入：

```text
~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.tunneldeck.native.json
```

### Windows

1. 使用 PowerShell 源码安装器安装到固定位置，默认是 `%LOCALAPPDATA%\Programs\TunnelDeck\TunnelDeck.exe`。
2. 启动应用，在“Chrome 浏览器集成”中填写扩展 ID。
3. TunnelDeck 写入清单，并创建当前用户注册表项：

```text
HKCU\Software\Google\Chrome\NativeMessagingHosts\com.tunneldeck.native
```

卸载器会删除上述注册表项和 Native Host 清单。

### Linux

用户级清单写入：

```text
~/.config/google-chrome/NativeMessagingHosts/com.tunneldeck.native.json
```

仍可使用 `scripts/` 中的跨平台脚本进行自动化安装或卸载。

## 3. macOS 二进制公开分发的未来条件

当前 CI 只做 Apple Silicon 构建检查，不上传 `.app`。如未来公开分发二进制，应完成：

1. 加入 Apple Developer Program，并获取 Developer ID Application 证书。
2. 使用 Developer ID 对应用内所有可执行文件及 `.app` 签名，同时启用 Hardened Runtime 和安全时间戳。
3. 用 `xcrun notarytool` 将 ZIP、PKG 或 DMG 提交 Apple 公证。
4. 用 `xcrun stapler` 将公证票据附加到应用或安装包。
5. 发布前执行 `codesign --verify --deep --strict` 和 `spctl --assess`。

签名证书和公证凭据只能放在 GitHub Actions Secrets 或专用密钥系统中，不能提交到仓库。

## 4. Windows 二进制公开分发的未来条件

当前 CI 只做 Windows 构建检查，不上传 EXE 或 NSIS 安装器。源码安装脚本在用户电脑上本地编译并复制到固定路径。如未来公开分发二进制，未签名 EXE/安装器仍可能被 SmartScreen 拦截。

公开分发可选择：

- Microsoft Store 的 MSIX 路径，由 Microsoft 在审核后签名，最容易避免 SmartScreen 下载警告。
- Azure Artifact Signing 或受信任 CA 签发的 OV 证书，对 EXE 和安装器执行 Authenticode 签名。

自签名证书只适合本机测试或由企业统一下发信任根证书的环境。即使使用有效 OV/EV 证书，新应用仍可能需要逐步积累 SmartScreen 信誉。

发布前至少使用 `Get-AuthenticodeSignature` 或 SignTool 验证主程序与安装器签名，并在一台未安装开发证书的干净 Windows 机器上验证安装、启动、注册、卸载流程。

## 官方参考

- [Chrome Web Store：准备扩展](https://developer.chrome.com/docs/webstore/prepare)
- [Chrome Web Store：发布扩展](https://developer.chrome.com/docs/webstore/publish)
- [Chrome Web Store：更新扩展](https://developer.chrome.com/docs/webstore/update)
- [Chrome Web Store API](https://developer.chrome.com/docs/webstore/using-api)
- [Chrome Extensions：Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging)
- [Apple：Developer ID 与 Gatekeeper](https://developer.apple.com/developer-id/)
- [Apple：公证 macOS 软件](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)
- [Microsoft：Windows 代码签名选项](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options)
- [Microsoft：SmartScreen 信誉](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation)

## 5. 版本发布

版本号需要同步更新：

- `wails.json` 的 `info.productVersion`
- `extension/package.json`
- `extension/package-lock.json`
- `extension/public/manifest.json`

推送 `v*` 标签后，Release 工作流会执行测试和三平台构建检查，只创建 GitHub 自动生成的源码归档与版本说明，不附加桌面可执行文件。独立的 `Chrome Web Store package` 工作流会生成扩展 ZIP、保存 14 天工作流产物，并在生产环境变量与 Google OIDC 联邦身份就绪后自动上传到 Chrome Web Store、提交审核；审核通过后按现有公开范围自动发布。
