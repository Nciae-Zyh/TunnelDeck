# Chrome Web Store 上架资料

本文只包含 TunnelDeck 扩展的商店资料和审核测试步骤，不包含开发者身份注册材料。

## 基础信息

- 名称：`TunnelDeck`
- 分类建议：`开发者工具`
- 语言：`中文（简体）`
- 支持邮箱：`support@sparkles-editor.com`
- 主页：`https://github.com/Nciae-Zyh/TunnelDeck`
- Chrome Web Store：`https://chromewebstore.google.com/detail/tunneldeck/jnfkjehpbkmfnidfcilehhkpbjjinmod`
- 隐私政策：`https://github.com/Nciae-Zyh/TunnelDeck/blob/main/docs/PRIVACY.md`

简短说明：

> 从 Chrome 侧边栏创建、启动和停止由本机 TunnelDeck 管理的 SSH 本地端口转发。

详细说明（商店当前公开版本应使用以下完整文案）：

> TunnelDeck 是一个需要配套开源桌面应用的 SSH 本地端口转发控制器。安装桌面端并注册 Chrome 集成后，可以直接在 Chrome 侧边栏管理本机隧道。
>
> 安装方法：
>
> 1. 打开 GitHub 项目，按 README 安装 TunnelDeck 桌面端：
>    https://github.com/Nciae-Zyh/TunnelDeck
> 2. macOS / Linux 可以使用：
>    curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.2/install.sh | sh
> 3. Windows PowerShell 可以使用：
>    irm https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.2/install.ps1 | iex
> 4. 从当前 Chrome Web Store 页面安装扩展，启动 TunnelDeck 桌面端，在底部“Chrome 浏览器集成”中确认正式扩展 ID，并点击“注册 Chrome 服务”；完成后重新加载扩展。
>
> 主要功能：
>
> • 从 Chrome 侧边栏创建、编辑和导入 SSH 本地端口转发；
> • 启动、停止并查看隧道实时状态；
> • 支持密码和 SSH 私钥认证，由桌面端调用操作系统安全存储；
> • 首次连接要求用户确认 SSH 主机指纹；
> • 只有用户把端口标记为 HTTP/HTTPS 并主动点击时，才会打开本地网页。
>
> 扩展不在浏览器内实现 SSH，也不执行任意 shell 命令。SSH、TCP 监听、私钥读取和系统凭据存储均由本机 TunnelDeck 桌面应用完成。扩展不读取浏览历史或网页内容，不申请所有网站访问权限，也不会自动打开端口页面。
>
> 项目源码、完整安装说明和问题反馈：
> https://github.com/Nciae-Zyh/TunnelDeck

## 单一用途

> 从 Chrome 侧边栏控制用户本机 TunnelDeck 创建的 SSH 本地端口转发。

## 权限说明

`nativeMessaging`：

> 连接用户自行安装并明确注册的 TunnelDeck 本机应用，用于发送连接管理命令并接收隧道状态。Native Host 只允许当前 TunnelDeck 商店扩展 ID，不能使用通配符。

`sidePanel`：

> 在 Chrome 侧边栏提供隧道配置、连接状态和用户操作入口。

## 隐私表单口径

- 不出售用户数据。
- 不把用户数据用于与扩展单一用途无关的目的。
- 不把用户数据用于信贷或借贷。
- 不收集或传输网页内容、浏览历史、位置、健康、金融或身份认证信息到开发者服务器。
- 会在用户设备本地处理 SSH 服务器地址、用户名、端口、隧道目标和连接设置。
- 认证信息不会写入 Chrome Storage；是否持久保存由用户明确选择，持久化由桌面应用写入操作系统安全存储。

提交表单时应依据当时实际代码逐项复核，不能只复制上述文字后跳过检查。

## 审核人员测试说明

1. 从 `https://github.com/Nciae-Zyh/TunnelDeck` 获取与待审核扩展版本相同的源码标签。
2. 在 macOS/Linux 运行一行安装命令，并把商店项目 ID 传给 `--chrome-store-id`：

   ```bash
   curl -fsSL https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.2/install.sh | sh
   ```

3. Windows PowerShell 使用：

   ```powershell
   irm https://raw.githubusercontent.com/Nciae-Zyh/TunnelDeck/v0.3.2/install.ps1 | iex
   ```

4. 启动 TunnelDeck 桌面端，确认底部 Chrome 集成显示相同 ID；如未注册，填写 ID 并点击“注册 Chrome 服务”。
5. 重新加载扩展，点击 Chrome 工具栏中的 TunnelDeck 图标打开侧边栏。
6. 创建测试隧道时需要审核人员自有的 SSH 测试服务器；也可以只验证创建、编辑、导入解析和 Native Host 状态。项目不提供共享 SSH 账号或远程测试凭据。
7. 启动隧道后确认状态变化；仅当配置显式标记为 HTTP/HTTPS 时才显示“打开网页”，且必须由用户点击。

审核备注中应明确：扩展必须依赖本机桌面应用，因此 Native Messaging 是核心功能而不是可选功能。

## 图片清单

上传前准备以下不包含真实服务器、IP、用户名、私钥路径或连接名称的演示素材：

- 128 × 128 扩展图标：仓库已有 `extension/public/icons/icon-128.png`；
- 至少一张 1280 × 800 或 640 × 400 的商店截图；
- 建议补充侧边栏配置、运行状态和“用户点击后打开网页”三个画面；
- 可选 440 × 280 小型宣传图。

截图应使用 `example.com`、环回地址和虚构端口，不要使用真实测试环境。

## 首次上传后的必要回填

首次在 Developer Dashboard 上传 ZIP 后，以商店实际分配的扩展 ID 为准：

1. 正式商店 ID 为 `jnfkjehpbkmfnidfcilehhkpbjjinmod`，不要使用开发者模式加载时的临时 ID；
2. `install.sh`、`install.ps1` 与桌面端默认输入框已回填正式 ID；
3. 使用正式 ID 重新测试 Native Host 的 `allowed_origins`；
4. 如果需要让本地解压开发构建稳定复用正式 ID，再从商店安装包取得公开密钥并评估是否加入 manifest 的 `key` 字段；私钥不得进入仓库；
5. 首次商店详情、隐私和分发配置完成后，后续 `v*` 标签通过 Chrome Web Store API V2 自动上传并提交审核。
