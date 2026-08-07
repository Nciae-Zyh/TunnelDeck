<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  Bootstrap, CopyText, DeleteProfile, ParseSSHCommand, PickPrivateKey,
  OpenProfileInBrowser, RegisterNativeHost, SaveProfile, StartTunnel, StopTunnel, TrustHost,
} from '../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../wailsjs/runtime/runtime'
import { main } from '../wailsjs/go/models'

type AuthMode = 'password' | 'private-key'
type TunnelState = 'stopped' | 'connecting' | 'running' | 'reconnecting' | 'error' | 'host-key-required'

interface TunnelProfile {
  id: string
  name: string
  sshHost: string
  sshPort: number
  username: string
  localBind: string
  localPort: number
  remoteHost: string
  remotePort: number
  authMode: AuthMode
  privateKeyPath: string
  rememberSecret: boolean
  autoReconnect: boolean
  webService: boolean
  webScheme: 'http' | 'https'
  createdAt: string
  updatedAt: string
}

interface ProfileView extends TunnelProfile { hasStoredSecret: boolean }
interface HostKeyInfo { host: string; port: number; keyType: string; fingerprint: string }
interface TunnelStatus {
  profileId: string
  state: TunnelState
  message: string
  localEndpoint: string
  remoteEndpoint: string
  activeConnections: number
  connectedAt?: string
  hostKey?: HostKeyInfo
  url?: string
}
interface OperationResult {
  ok: boolean
  code?: string
  message?: string
  profile?: TunnelProfile
  status?: TunnelStatus
  hostKey?: HostKeyInfo
}
interface NativeHostRegistrationResult {
  ok: boolean
  code?: string
  message?: string
  extensionId?: string
  manifestPath?: string
  binaryPath?: string
}

const profiles = ref<ProfileView[]>([])
const statuses = reactive<Record<string, TunnelStatus>>({})
const selectedId = ref('')
const draft = reactive<TunnelProfile>(blankProfile())
const secret = ref('')
const busy = ref(false)
const booting = ref(true)
const toast = reactive({ visible: false, kind: 'success', message: '' })
const importOpen = ref(false)
const importCommand = ref('ssh -L 9108:127.0.0.1:9108 -p 33899 root@ssh.example.com')
const configPath = ref('')
const extensionId = ref('')
const nativeHostBusy = ref(false)
const nativeHostRegistration = ref<NativeHostRegistrationResult | null>(null)
let toastTimer: number | undefined

function blankProfile(): TunnelProfile {
  return {
    id: '', name: '新建隧道', sshHost: '', sshPort: 22, username: 'root',
    localBind: '127.0.0.1', localPort: 9108, remoteHost: '127.0.0.1', remotePort: 9108,
    authMode: 'password', privateKeyPath: '', rememberSecret: false, autoReconnect: true,
    webService: false, webScheme: 'http',
    createdAt: '', updatedAt: '',
  }
}

const selectedProfile = computed(() => profiles.value.find(profile => profile.id === selectedId.value))
const currentStatus = computed<TunnelStatus>(() => {
  if (draft.id && statuses[draft.id]) return statuses[draft.id]
  return {
    profileId: draft.id,
    state: 'stopped',
    message: draft.id ? '隧道未启动' : '保存配置后即可启动',
    localEndpoint: endpoint(draft.localBind, draft.localPort),
    remoteEndpoint: endpoint(draft.remoteHost, draft.remotePort),
    activeConnections: 0,
  }
})
const isRunning = computed(() => ['running', 'connecting', 'reconnecting'].includes(currentStatus.value.state))
const needsHostTrust = computed(() => currentStatus.value.state === 'host-key-required' && currentStatus.value.hostKey)
const broadBind = computed(() => ['0.0.0.0', '::'].includes(draft.localBind))
const secretLabel = computed(() => draft.authMode === 'password' ? 'SSH 密码' : '私钥口令（未加密可留空）')
const hasStoredSecret = computed(() => selectedProfile.value?.hasStoredSecret && draft.rememberSecret)
const extensionIdValid = computed(() => /^[a-p]{32}$/.test(extensionId.value.trim()))
const commandPreview = computed(() => {
  const bind = formatCommandHost(draft.localBind || '127.0.0.1')
  const remote = formatCommandHost(draft.remoteHost || '127.0.0.1')
  const host = formatCommandHost(draft.sshHost || 'ssh.example.com')
  const key = draft.authMode === 'private-key' && draft.privateKeyPath ? ' -i ' + shellQuote(draft.privateKeyPath) : ''
  return 'ssh -N -L ' + bind + ':' + draft.localPort + ':' + remote + ':' + draft.remotePort
    + ' -p ' + draft.sshPort + key + ' ' + (draft.username || 'root') + '@' + host
})

function endpoint(host: string, port: number): string {
  return host.includes(':') ? '[' + host + ']:' + port : host + ':' + port
}
function formatCommandHost(host: string): string {
  return host.includes(':') && !host.startsWith('[') ? '[' + host + ']' : host
}
function shellQuote(value: string): string {
  if (!/[\s'"]/.test(value)) return value
  return "'" + value.replaceAll("'", "'\\''") + "'"
}
function assignDraft(profile: TunnelProfile) {
  Object.assign(draft, blankProfile(), profile)
}
function notify(message: string, kind: 'success' | 'error' = 'success') {
  if (toastTimer) window.clearTimeout(toastTimer)
  Object.assign(toast, { visible: true, kind, message })
  toastTimer = window.setTimeout(() => { toast.visible = false }, 4200)
}

async function loadData(preferredId?: string) {
  const data = await Bootstrap()
  profiles.value = (data.profiles ?? []) as ProfileView[]
  configPath.value = data.configPath ?? ''
  for (const key of Object.keys(statuses)) delete statuses[key]
  for (const status of (data.statuses ?? []) as TunnelStatus[]) statuses[status.profileId] = status
  if (data.startupError) notify(data.startupError, 'error')
  const selected = profiles.value.find(profile => profile.id === (preferredId || selectedId.value)) ?? profiles.value[0]
  if (selected) {
    selectedId.value = selected.id
    assignDraft(selected)
  } else {
    createProfile()
  }
}

function createProfile() {
  selectedId.value = ''
  assignDraft(blankProfile())
  secret.value = ''
}
function selectProfile(profile: ProfileView) {
  selectedId.value = profile.id
  assignDraft(profile)
  secret.value = ''
}

async function saveCurrent(): Promise<string | null> {
  busy.value = true
  try {
    const result = await SaveProfile(new main.SaveProfileRequest({
      profile: { ...draft },
      secret: secret.value,
    })) as OperationResult
    if (result.profile) {
      assignDraft(result.profile)
      selectedId.value = result.profile.id
      await loadData(result.profile.id)
    }
    if (!result.ok) {
      notify(result.message || '保存失败', 'error')
      return null
    }
    notify(result.message || '配置已保存')
    return result.profile?.id || draft.id
  } catch (error) {
    notify(String(error), 'error')
    return null
  } finally {
    busy.value = false
  }
}

async function startCurrent() {
  const profileId = await saveCurrent()
  if (!profileId) return
  busy.value = true
  try {
    const result = await StartTunnel({ profileId, secret: secret.value }) as OperationResult
    applyResult(result)
    if (result.ok && !draft.rememberSecret) secret.value = ''
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    busy.value = false
  }
}
async function stopCurrent() {
  if (!draft.id) return
  busy.value = true
  try {
    applyResult(await StopTunnel(draft.id) as OperationResult)
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    busy.value = false
  }
}
async function openCurrentInBrowser() {
  if (!draft.id || currentStatus.value.state !== 'running' || !draft.webService) return
  try {
    const result = await OpenProfileInBrowser(draft.id) as OperationResult
    notify(result.message || '已使用默认浏览器打开', result.ok ? 'success' : 'error')
  } catch (error) {
    notify(String(error), 'error')
  }
}
async function trustAndConnect() {
  if (!draft.id) return
  busy.value = true
  try {
    const trusted = await TrustHost(draft.id) as OperationResult
    if (!trusted.ok) return notify(trusted.message || '保存主机指纹失败', 'error')
    applyResult(await StartTunnel({ profileId: draft.id, secret: secret.value }) as OperationResult)
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    busy.value = false
  }
}
function applyResult(result: OperationResult) {
  if (result.status) statuses[result.status.profileId] = result.status
  notify(result.message || (result.ok ? '操作完成' : '操作失败'), result.ok ? 'success' : 'error')
}

async function removeCurrent() {
  if (!draft.id) return createProfile()
  if (!window.confirm('确定删除“' + draft.name + '”吗？正在运行的隧道也会停止。')) return
  busy.value = true
  try {
    const result = await DeleteProfile(draft.id) as OperationResult
    if (!result.ok) return notify(result.message || '删除失败', 'error')
    notify('配置已删除')
    selectedId.value = ''
    await loadData()
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    busy.value = false
  }
}
async function pickKey() {
  try {
    const result = await PickPrivateKey() as OperationResult
    if (result.ok && result.profile?.privateKeyPath) draft.privateKeyPath = result.profile.privateKeyPath
    else if (result.code !== 'CANCELLED') notify(result.message || '选择文件失败', 'error')
  } catch (error) {
    notify(String(error), 'error')
  }
}
async function parseImport() {
  busy.value = true
  try {
    const result = await ParseSSHCommand(importCommand.value) as { ok: boolean; message?: string; profile?: TunnelProfile }
    if (!result.ok || !result.profile) return notify(result.message || '无法解析命令', 'error')
    createProfile()
    assignDraft({ ...blankProfile(), ...result.profile, id: '' })
    importOpen.value = false
    notify('命令已解析，请选择认证方式后保存')
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    busy.value = false
  }
}
async function copyCommand() {
  try {
    const result = await CopyText(commandPreview.value) as OperationResult
    notify(result.message || '已复制', result.ok ? 'success' : 'error')
  } catch (error) {
    notify(String(error), 'error')
  }
}
async function registerChromeService() {
  const normalizedId = extensionId.value.trim()
  if (!/^[a-p]{32}$/.test(normalizedId)) {
    notify('扩展 ID 必须是 32 位小写字母，且只能使用 a 到 p', 'error')
    return
  }
  nativeHostBusy.value = true
  try {
    const result = await RegisterNativeHost(normalizedId) as NativeHostRegistrationResult
    nativeHostRegistration.value = result
    notify(result.message || (result.ok ? 'Chrome 服务已注册' : '注册失败'), result.ok ? 'success' : 'error')
  } catch (error) {
    const result = { ok: false, code: 'CALL_FAILED', message: String(error) }
    nativeHostRegistration.value = result
    notify(result.message, 'error')
  } finally {
    nativeHostBusy.value = false
  }
}
function stateLabel(state: TunnelState): string {
  return {
    stopped: '已停止', connecting: '连接中', running: '运行中', reconnecting: '重连中',
    error: '异常', 'host-key-required': '待确认',
  }[state]
}

onMounted(async () => {
  EventsOn('tunnel:status', (status: TunnelStatus) => { statuses[status.profileId] = status })
  try {
    await loadData()
  } catch (error) {
    notify(String(error), 'error')
  } finally {
    booting.value = false
  }
})
onBeforeUnmount(() => {
  EventsOff('tunnel:status')
  if (toastTimer) window.clearTimeout(toastTimer)
})
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <header class="brand">
        <div class="brand-mark" aria-hidden="true"><span></span><span></span></div>
        <div><strong>TunnelDeck</strong><small>SSH PORT FORWARDING</small></div>
      </header>
      <div class="sidebar-actions">
        <button class="button primary compact" type="button" @click="createProfile">＋ 新建</button>
        <button class="button ghost compact" type="button" @click="importOpen = !importOpen">导入命令</button>
      </div>
      <section v-if="importOpen" class="import-card">
        <label for="ssh-command">粘贴 ssh -L 命令</label>
        <textarea id="ssh-command" v-model="importCommand" rows="4" spellcheck="false"></textarea>
        <div class="inline-actions">
          <button class="button primary compact" type="button" :disabled="busy" @click="parseImport">解析</button>
          <button class="button text compact" type="button" @click="importOpen = false">取消</button>
        </div>
      </section>
      <nav class="profile-list" aria-label="隧道配置">
        <button
          v-for="profile in profiles" :key="profile.id" class="profile-item"
          :class="{ active: profile.id === selectedId }" type="button" @click="selectProfile(profile)"
        >
          <span class="state-dot" :data-state="statuses[profile.id]?.state ?? 'stopped'"></span>
          <span class="profile-copy">
            <strong>{{ profile.name }}</strong>
            <small>{{ profile.localBind }}:{{ profile.localPort }} → {{ profile.remoteHost }}:{{ profile.remotePort }}</small>
          </span>
          <span v-if="statuses[profile.id]?.state === 'running'" class="connection-count">{{ statuses[profile.id]?.activeConnections ?? 0 }}</span>
        </button>
        <div v-if="!profiles.length && !booting" class="empty-list">还没有隧道配置。<br>从新建或导入命令开始。</div>
      </nav>
      <footer class="sidebar-footer"><span class="shield">◇</span><span>密码只在内存或系统钥匙串中保存</span></footer>
    </aside>

    <main class="workspace">
      <header class="workspace-header">
        <div><p class="eyebrow">LOCAL FORWARD / SSH</p><h1>{{ draft.name || '未命名隧道' }}</h1></div>
        <div class="header-actions">
          <button
            v-if="currentStatus.state === 'running' && draft.webService"
            class="button ghost" type="button" :disabled="busy" @click="openCurrentInBrowser"
          >打开网页 ↗</button>
          <button class="button ghost" type="button" :disabled="busy || isRunning" @click="saveCurrent">保存配置</button>
          <button v-if="!isRunning" class="button primary" type="button" :disabled="busy" @click="startCurrent"><span class="play" aria-hidden="true">▶</span> 保存并启动</button>
          <button v-else class="button danger" type="button" :disabled="busy" @click="stopCurrent">■ 停止隧道</button>
        </div>
      </header>

      <section class="status-panel" :data-state="currentStatus.state">
        <div class="status-main"><span class="pulse-dot"></span><div><span class="status-label">{{ stateLabel(currentStatus.state) }}</span><p>{{ currentStatus.message }}</p></div></div>
        <div class="route-line">
          <span><small>本机入口</small>{{ currentStatus.localEndpoint }}</span><span class="route-arrow">→</span>
          <span><small>SSH 内目标</small>{{ currentStatus.remoteEndpoint }}</span>
        </div>
        <div class="metric"><strong>{{ currentStatus.activeConnections }}</strong><small>活动连接</small></div>
      </section>

      <section v-if="needsHostTrust" class="host-key-card">
        <div class="host-key-icon">!</div>
        <div class="host-key-copy">
          <strong>首次连接：请核对服务器指纹</strong>
          <p>只有在它与服务器管理员提供的指纹一致时才信任。之后若主机密钥变化，TunnelDeck 会阻止连接。</p>
          <code>{{ currentStatus.hostKey?.keyType }} · {{ currentStatus.hostKey?.fingerprint }}</code>
        </div>
        <button class="button warning" type="button" :disabled="busy" @click="trustAndConnect">信任并连接</button>
      </section>

      <form class="editor-grid" @submit.prevent="startCurrent">
        <section class="panel identity-panel">
          <div class="panel-heading"><span class="step">01</span><div><h2>隧道标识</h2><p>方便从列表里快速找到它</p></div></div>
          <label class="field full"><span>配置名称</span><input v-model.trim="draft.name" :disabled="isRunning" autocomplete="off"></label>
        </section>

        <section class="panel">
          <div class="panel-heading"><span class="step">02</span><div><h2>SSH 跳板机</h2><p>建立加密连接的服务器</p></div></div>
          <div class="fields two-one">
            <label class="field"><span>服务器地址</span><input v-model.trim="draft.sshHost" :disabled="isRunning" placeholder="ssh.example.com" autocomplete="off"></label>
            <label class="field"><span>端口</span><input v-model.number="draft.sshPort" :disabled="isRunning" type="number" min="1" max="65535"></label>
          </div>
          <label class="field full"><span>用户名</span><input v-model.trim="draft.username" :disabled="isRunning" placeholder="root" autocomplete="username"></label>
        </section>

        <section class="panel">
          <div class="panel-heading"><span class="step">03</span><div><h2>端口映射</h2><p>等价于 OpenSSH 的 -L 参数</p></div></div>
          <div class="mapping">
            <div class="mapping-side">
              <small>THIS DEVICE</small>
              <label class="field"><span>本地绑定地址</span><input v-model.trim="draft.localBind" :disabled="isRunning" autocomplete="off"></label>
              <label class="field"><span>本地端口</span><input v-model.number="draft.localPort" :disabled="isRunning" type="number" min="1" max="65535"></label>
            </div>
            <div class="mapping-arrow"><span>SSH</span>→</div>
            <div class="mapping-side">
              <small>REMOTE NETWORK</small>
              <label class="field"><span>远程目标地址</span><input v-model.trim="draft.remoteHost" :disabled="isRunning" autocomplete="off"></label>
              <label class="field"><span>远程目标端口</span><input v-model.number="draft.remotePort" :disabled="isRunning" type="number" min="1" max="65535"></label>
            </div>
          </div>
          <p v-if="broadBind" class="inline-warning">当前地址会向局域网或公网暴露该本地端口。只供本机使用时请保持 127.0.0.1。</p>
        </section>

        <section class="panel auth-panel">
          <div class="panel-heading"><span class="step">04</span><div><h2>认证与凭据</h2><p>选择密码或 SSH 私钥</p></div></div>
          <div class="segmented" role="radiogroup" aria-label="认证方式">
            <button
              type="button" role="radio" :aria-checked="draft.authMode === 'password'"
              :class="{ active: draft.authMode === 'password' }" :disabled="isRunning"
              @click="draft.authMode = 'password'"
            >密码</button>
            <button
              type="button" role="radio" :aria-checked="draft.authMode === 'private-key'"
              :class="{ active: draft.authMode === 'private-key' }" :disabled="isRunning"
              @click="draft.authMode = 'private-key'"
            >SSH 私钥</button>
          </div>
          <div v-if="draft.authMode === 'private-key'" class="key-picker">
            <label class="field">
              <span>私钥文件</span>
              <input v-model.trim="draft.privateKeyPath" :disabled="isRunning" placeholder="~/.ssh/id_ed25519" autocomplete="off">
            </label>
            <button class="button ghost" type="button" :disabled="isRunning" @click="pickKey">选择…</button>
          </div>
          <label class="field full secret-field">
            <span>{{ secretLabel }}</span>
            <input
              v-model="secret" :disabled="isRunning" type="password"
              :placeholder="hasStoredSecret ? '已存入系统钥匙串；留空沿用' : '不会写入配置文件'"
              autocomplete="current-password"
            >
            <em v-if="hasStoredSecret">系统钥匙串中已有凭据</em>
          </label>
          <div class="toggles">
            <label class="toggle-row">
              <input v-model="draft.rememberSecret" :disabled="isRunning" type="checkbox">
              <span class="toggle"></span>
              <span><strong>记住凭据</strong><small>加密保存到系统钥匙串；关闭时仅驻留内存</small></span>
            </label>
            <label class="toggle-row">
              <input v-model="draft.autoReconnect" :disabled="isRunning" type="checkbox">
              <span class="toggle"></span>
              <span><strong>断线自动重连</strong><small>采用 2–30 秒退避，停止后立即清除内存凭据</small></span>
            </label>
          </div>
        </section>

        <section class="panel web-panel">
          <div class="panel-heading"><span class="step">05</span><div><h2>网页快捷入口</h2><p>仅在用户点击后打开，不会自动跳转</p></div></div>
          <div class="web-controls">
            <label class="toggle-row">
              <input v-model="draft.webService" :disabled="isRunning" type="checkbox">
              <span class="toggle"></span>
              <span><strong>这是网页服务</strong><small>启用后，连接成功时显示“打开网页”按钮</small></span>
            </label>
            <label v-if="draft.webService" class="field scheme-field">
              <span>网页协议</span>
              <select v-model="draft.webScheme" :disabled="isRunning">
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
              </select>
            </label>
          </div>
        </section>

        <section class="panel command-panel">
          <div class="panel-heading"><span class="step">06</span><div><h2>等价命令</h2><p>用于核对配置，不会包含密码</p></div></div>
          <div class="command-box">
            <code>{{ commandPreview }}</code>
            <button type="button" aria-label="复制等价命令" title="复制" @click="copyCommand">复制</button>
          </div>
          <div class="panel-footer">
            <span v-if="configPath">配置：{{ configPath }}</span>
            <button class="delete-link" type="button" :disabled="busy || isRunning" @click="removeCurrent">删除此配置</button>
          </div>
        </section>

        <section class="panel browser-panel">
          <div class="panel-heading"><span class="step">07</span><div><h2>Chrome 浏览器集成</h2><p>按扩展 ID 注册本机通信服务</p></div></div>
          <div class="browser-registration">
            <label class="field">
              <span>Chrome 扩展 ID</span>
              <input
                v-model.trim="extensionId"
                maxlength="32"
                pattern="[a-p]{32}"
                placeholder="例如：ipmjdganppehhljijcdndfjjmjjpalbp"
                autocomplete="off"
                spellcheck="false"
              >
            </label>
            <button
              class="button primary"
              type="button"
              :disabled="nativeHostBusy || !extensionIdValid"
              @click="registerChromeService"
            >{{ nativeHostBusy ? '注册中…' : '注册 Chrome 服务' }}</button>
          </div>
          <p class="browser-help">在 <code>chrome://extensions</code> 打开开发者模式即可复制扩展 ID。注册只允许该扩展访问 TunnelDeck；扩展 ID 变化后需要重新注册。</p>
          <div v-if="nativeHostRegistration" class="registration-result" :data-ok="nativeHostRegistration.ok">
            <strong>{{ nativeHostRegistration.ok ? '注册成功' : '注册失败' }}</strong>
            <span>{{ nativeHostRegistration.message }}</span>
            <code v-if="nativeHostRegistration.manifestPath">清单：{{ nativeHostRegistration.manifestPath }}</code>
            <code v-if="nativeHostRegistration.binaryPath">程序：{{ nativeHostRegistration.binaryPath }}</code>
          </div>
        </section>
      </form>
    </main>

    <transition name="toast">
      <div v-if="toast.visible" class="toast" :data-kind="toast.kind" role="status"><span>{{ toast.kind === 'success' ? '✓' : '!' }}</span>{{ toast.message }}</div>
    </transition>
  </div>
</template>
