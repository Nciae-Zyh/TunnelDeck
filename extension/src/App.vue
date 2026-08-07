<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NativeBridgeError, nativeBridge } from './native'
import type {
  BootstrapData,
  OperationResult,
  ParseCommandResult,
  ProfileView,
  TunnelProfile,
  TunnelState,
  TunnelStatus,
} from './types'

type Screen = 'profiles' | 'editor'

const screen = ref<Screen>('profiles')
const profiles = ref<ProfileView[]>([])
const statuses = ref<Record<string, TunnelStatus>>({})
const selectedId = ref('')
const draft = ref<TunnelProfile>(blankProfile())
const secret = ref('')
const importCommand = ref('ssh -L 9108:127.0.0.1:9108 -p 33899 root@ssh.example.com')
const importVisible = ref(false)
const loading = ref(true)
const busy = ref(false)
const nativeConnected = ref(false)
const nativeError = ref('')
const notice = ref({ visible: false, kind: 'success' as 'success' | 'error', message: '' })
let noticeTimer: number | undefined
let removeStatusListener: (() => void) | undefined
let removeConnectionListener: (() => void) | undefined

function blankProfile(): TunnelProfile {
  return {
    id: '',
    name: '新建隧道',
    sshHost: '',
    sshPort: 22,
    username: 'root',
    localBind: '127.0.0.1',
    localPort: 9108,
    remoteHost: '127.0.0.1',
    remotePort: 9108,
    authMode: 'password',
    privateKeyPath: '',
    rememberSecret: false,
    autoReconnect: true,
    webService: false,
    webScheme: 'http',
    createdAt: '',
    updatedAt: '',
  }
}

const selectedProfile = computed(() => profiles.value.find(profile => profile.id === selectedId.value))
const currentStatus = computed(() => {
  if (draft.value.id && statuses.value[draft.value.id]) return statuses.value[draft.value.id]
  return fallbackStatus(draft.value)
})
const currentRunning = computed(() => isActive(currentStatus.value.state))
const needsHostTrust = computed(() => currentStatus.value.state === 'host-key-required' && currentStatus.value.hostKey)
const secretLabel = computed(() => draft.value.authMode === 'password' ? 'SSH 密码' : '私钥口令（未加密可留空）')
const hasStoredSecret = computed(() => selectedProfile.value?.hasStoredSecret && draft.value.rememberSecret)

function fallbackStatus(profile: TunnelProfile): TunnelStatus {
  return {
    profileId: profile.id,
    state: 'stopped',
    message: profile.id ? '隧道未启动' : '保存配置后即可启动',
    localEndpoint: endpoint(profile.localBind, profile.localPort),
    remoteEndpoint: endpoint(profile.remoteHost, profile.remotePort),
    activeConnections: 0,
  }
}

function endpoint(host: string, port: number): string {
  return host.includes(':') ? `[${host}]:${port}` : `${host}:${port}`
}

function isActive(state: TunnelState): boolean {
  return state === 'running' || state === 'connecting' || state === 'reconnecting'
}

function stateLabel(state: TunnelState): string {
  return {
    stopped: '已停止',
    connecting: '连接中',
    running: '运行中',
    reconnecting: '重连中',
    error: '异常',
    'host-key-required': '待确认',
  }[state]
}

function showNotice(message: string, kind: 'success' | 'error' = 'success') {
  if (noticeTimer) window.clearTimeout(noticeTimer)
  notice.value = { visible: true, kind, message }
  noticeTimer = window.setTimeout(() => {
    notice.value.visible = false
  }, 4200)
}

function applyStatus(status?: TunnelStatus) {
  if (!status) return
  statuses.value = { ...statuses.value, [status.profileId]: status }
}

function unwrapOperationError(error: unknown): OperationResult | undefined {
  if (!(error instanceof NativeBridgeError) || typeof error.data !== 'object' || error.data === null) return
  return error.data as OperationResult
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

async function loadData(preferredId?: string) {
  const data = await nativeBridge.request<BootstrapData>('bootstrap')
  profiles.value = data.profiles ?? []
  statuses.value = Object.fromEntries((data.statuses ?? []).map(status => [status.profileId, status]))
  if (data.startupError) showNotice(data.startupError, 'error')
  const selected = profiles.value.find(profile => profile.id === (preferredId || selectedId.value))
  if (selected && screen.value === 'editor') {
    selectProfile(selected)
  }
}

function createProfile() {
  selectedId.value = ''
  draft.value = blankProfile()
  secret.value = ''
  screen.value = 'editor'
}

function selectProfile(profile: ProfileView) {
  selectedId.value = profile.id
  draft.value = { ...blankProfile(), ...profile }
  secret.value = ''
  screen.value = 'editor'
}

function backToProfiles() {
  secret.value = ''
  screen.value = 'profiles'
}

function reloadPanel() {
	window.location.reload()
}

async function saveCurrent(): Promise<string | null> {
  busy.value = true
  try {
    const result = await nativeBridge.request<OperationResult>('saveProfile', {
      profile: { ...draft.value },
      secret: secret.value,
    })
    if (result.profile) {
      draft.value = { ...draft.value, ...result.profile }
      selectedId.value = result.profile.id
      await loadData(result.profile.id)
    }
    showNotice(result.message || '配置已保存')
    return result.profile?.id || draft.value.id
  } catch (error) {
    showNotice(errorMessage(error), 'error')
    return null
  } finally {
    busy.value = false
  }
}

async function startCurrent() {
  const profileId = await saveCurrent()
  if (!profileId) return
  await startProfileById(profileId, secret.value)
}

async function quickStart(profile: ProfileView) {
  if (profile.authMode === 'password' && !profile.hasStoredSecret) {
    selectProfile(profile)
    showNotice('请输入 SSH 密码后启动', 'error')
    return
  }
  await startProfileById(profile.id, '')
}

async function startProfileById(profileId: string, suppliedSecret: string) {
  busy.value = true
  try {
    const result = await nativeBridge.request<OperationResult>('startTunnel', {
      profileId,
      secret: suppliedSecret,
    }, 30_000)
    applyStatus(result.status)
    showNotice(result.message || '隧道已启动')
    if (!draft.value.rememberSecret) secret.value = ''
    await loadData(profileId)
  } catch (error) {
    const result = unwrapOperationError(error)
    applyStatus(result?.status)
    if (error instanceof NativeBridgeError && error.code === 'SECRET_REQUIRED') {
      const profile = profiles.value.find(item => item.id === profileId)
      if (profile) selectProfile(profile)
    }
    showNotice(errorMessage(error), 'error')
  } finally {
    busy.value = false
  }
}

async function stopProfile(profileId: string) {
  busy.value = true
  try {
    const result = await nativeBridge.request<OperationResult>('stopTunnel', { profileId })
    applyStatus(result.status)
    showNotice(result.message || '隧道已停止')
    await loadData(profileId)
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  } finally {
    busy.value = false
  }
}

async function trustAndConnect() {
  if (!draft.value.id) return
  busy.value = true
  try {
    await nativeBridge.request<OperationResult>('trustHost', { profileId: draft.value.id })
    await startProfileById(draft.value.id, secret.value)
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  } finally {
    busy.value = false
  }
}

async function openProfile(profile: ProfileView | TunnelProfile) {
  if (!profile.id || !profile.webService) return
  try {
    const result = await nativeBridge.request<OperationResult>('browserURL', { profileId: profile.id })
    if (!result.url) throw new Error('本地服务没有返回网页地址')
    await chrome.tabs.create({ url: result.url, active: true })
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  }
}

async function removeCurrent() {
  if (!draft.value.id) return backToProfiles()
  if (!window.confirm(`确定删除“${draft.value.name}”吗？正在运行的隧道也会停止。`)) return
  busy.value = true
  try {
    await nativeBridge.request<OperationResult>('deleteProfile', { profileId: draft.value.id })
    showNotice('配置已删除')
    selectedId.value = ''
    screen.value = 'profiles'
    await loadData()
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  } finally {
    busy.value = false
  }
}

async function parseImport() {
  busy.value = true
  try {
    const result = await nativeBridge.request<ParseCommandResult>('parseSSHCommand', { command: importCommand.value })
    if (!result.profile) throw new Error('没有解析到隧道配置')
    draft.value = { ...blankProfile(), ...result.profile, id: '' }
    selectedId.value = ''
    secret.value = ''
    importVisible.value = false
    screen.value = 'editor'
    showNotice('命令已解析，请确认认证方式')
  } catch (error) {
    showNotice(errorMessage(error), 'error')
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  removeStatusListener = nativeBridge.onStatus(applyStatus)
  removeConnectionListener = nativeBridge.onConnection((connected, error) => {
    nativeConnected.value = connected
    nativeError.value = error
  })
  try {
    await nativeBridge.request('ping')
    nativeConnected.value = true
    nativeError.value = ''
    await loadData()
  } catch (error) {
    nativeConnected.value = false
    nativeError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
})

onBeforeUnmount(() => {
  removeStatusListener?.()
  removeConnectionListener?.()
  if (noticeTimer) window.clearTimeout(noticeTimer)
})
</script>

<template>
  <main class="app-shell">
    <header class="topbar">
      <button v-if="screen === 'editor'" class="icon-button" type="button" aria-label="返回隧道列表" @click="backToProfiles">←</button>
      <div class="brand-mark" aria-hidden="true"><span></span><span></span></div>
      <div class="brand-copy"><strong>TunnelDeck</strong><small>CHROME CONTROL</small></div>
      <span class="native-state" :class="{ online: nativeConnected }">{{ nativeConnected ? '本地服务在线' : '未连接' }}</span>
    </header>

    <section v-if="!nativeConnected && !loading" class="offline-card">
      <span class="offline-icon">!</span>
      <h1>需要 TunnelDeck 本地服务</h1>
      <p>{{ nativeError || 'Chrome 找不到 Native Messaging Host。' }}</p>
      <ol>
        <li>安装 TunnelDeck 桌面版。</li>
        <li>运行项目提供的 Native Host 注册脚本。</li>
        <li>关闭并重新打开此侧边栏。</li>
      </ol>
      <button class="button primary" type="button" @click="reloadPanel">重新检测</button>
    </section>

    <section v-else-if="screen === 'profiles'" class="content profiles-screen">
      <div class="section-heading">
        <div><p class="eyebrow">LOCAL FORWARD / SSH</p><h1>隧道连接</h1></div>
        <button class="button primary compact" type="button" :disabled="busy" @click="createProfile">＋ 新建</button>
      </div>

      <button class="import-toggle" type="button" @click="importVisible = !importVisible">
        <span>从 ssh -L 命令导入</span><span>{{ importVisible ? '−' : '+' }}</span>
      </button>
      <form v-if="importVisible" class="import-card" @submit.prevent="parseImport">
        <label for="ssh-command">SSH 命令</label>
        <textarea id="ssh-command" v-model="importCommand" rows="4" spellcheck="false"></textarea>
        <button class="button ghost compact" type="submit" :disabled="busy">解析命令</button>
      </form>

      <div class="profile-list" aria-live="polite">
        <article v-for="profile in profiles" :key="profile.id" class="profile-card" :data-state="statuses[profile.id]?.state ?? 'stopped'">
          <button class="profile-main" type="button" @click="selectProfile(profile)">
            <span class="state-dot"></span>
            <span class="profile-copy">
              <strong>{{ profile.name }}</strong>
              <small>{{ profile.localBind }}:{{ profile.localPort }} → {{ profile.remoteHost }}:{{ profile.remotePort }}</small>
              <em>{{ stateLabel(statuses[profile.id]?.state ?? 'stopped') }} · {{ statuses[profile.id]?.message ?? '隧道未启动' }}</em>
            </span>
            <span aria-hidden="true">›</span>
          </button>
          <div class="profile-actions">
            <button
              v-if="!isActive(statuses[profile.id]?.state ?? 'stopped')"
              class="button primary compact" type="button" :disabled="busy" @click="quickStart(profile)"
            >启动</button>
            <button v-else class="button danger compact" type="button" :disabled="busy" @click="stopProfile(profile.id)">停止</button>
            <button
              v-if="profile.webService && statuses[profile.id]?.state === 'running'"
              class="button ghost compact" type="button" @click="openProfile(profile)"
            >打开网页 ↗</button>
          </div>
        </article>
        <div v-if="!profiles.length && !loading" class="empty-state">
          <div class="empty-orbit"><span></span></div>
          <strong>还没有隧道</strong>
          <p>创建连接，或者粘贴一条现有的 ssh -L 命令。</p>
        </div>
      </div>
    </section>

    <section v-else class="content editor-screen">
      <div class="section-heading editor-title">
        <div><p class="eyebrow">TUNNEL PROFILE</p><h1>{{ draft.name || '未命名隧道' }}</h1></div>
        <span class="status-chip" :data-state="currentStatus.state">{{ stateLabel(currentStatus.state) }}</span>
      </div>

      <section class="route-card" :data-state="currentStatus.state">
        <div><span class="state-dot"></span><strong>{{ currentStatus.message }}</strong></div>
        <code>{{ currentStatus.localEndpoint }} → {{ currentStatus.remoteEndpoint }}</code>
        <button
          v-if="draft.webService && currentStatus.state === 'running'"
          class="button ghost compact" type="button" @click="openProfile(draft)"
        >在 Chrome 中打开 ↗</button>
      </section>

      <section v-if="needsHostTrust" class="host-key-card">
        <strong>核对 SSH 主机指纹</strong>
        <p>请通过可信渠道确认指纹。TunnelDeck 不会静默接受未知主机。</p>
        <code>{{ currentStatus.hostKey?.keyType }} · {{ currentStatus.hostKey?.fingerprint }}</code>
        <button class="button warning" type="button" :disabled="busy" @click="trustAndConnect">信任并连接</button>
      </section>

      <form class="editor-form" @submit.prevent="startCurrent">
        <fieldset :disabled="currentRunning || busy">
          <legend>基本信息</legend>
          <label class="field full"><span>配置名称</span><input v-model.trim="draft.name" autocomplete="off" required></label>
          <div class="field-grid host-port">
            <label class="field"><span>SSH 服务器</span><input v-model.trim="draft.sshHost" placeholder="ssh.example.com" autocomplete="off" required></label>
            <label class="field"><span>端口</span><input v-model.number="draft.sshPort" type="number" min="1" max="65535" required></label>
          </div>
          <label class="field full"><span>SSH 用户</span><input v-model.trim="draft.username" autocomplete="username" required></label>
        </fieldset>

        <fieldset :disabled="currentRunning || busy">
          <legend>端口映射</legend>
          <div class="mapping-label"><span>本机入口</span><span>SSH 内目标</span></div>
          <div class="mapping-grid">
            <div class="field-grid host-port">
              <label class="field"><span>绑定地址</span><input v-model.trim="draft.localBind" required></label>
              <label class="field"><span>端口</span><input v-model.number="draft.localPort" type="number" min="1" max="65535" required></label>
            </div>
            <span class="mapping-arrow">→</span>
            <div class="field-grid host-port">
              <label class="field"><span>目标地址</span><input v-model.trim="draft.remoteHost" required></label>
              <label class="field"><span>端口</span><input v-model.number="draft.remotePort" type="number" min="1" max="65535" required></label>
            </div>
          </div>
          <p v-if="draft.localBind === '0.0.0.0' || draft.localBind === '::'" class="inline-warning">当前会向局域网暴露端口，请确认系统防火墙设置。</p>
        </fieldset>

        <fieldset :disabled="currentRunning || busy">
          <legend>认证</legend>
          <div class="segmented">
            <button type="button" :class="{ active: draft.authMode === 'password' }" @click="draft.authMode = 'password'">密码</button>
            <button type="button" :class="{ active: draft.authMode === 'private-key' }" @click="draft.authMode = 'private-key'">SSH 私钥</button>
          </div>
          <label v-if="draft.authMode === 'private-key'" class="field full">
            <span>私钥文件路径</span>
            <input v-model.trim="draft.privateKeyPath" placeholder="~/.ssh/id_ed25519" autocomplete="off" required>
            <small>浏览器不能直接读取文件；路径会交给本地 TunnelDeck 打开。</small>
          </label>
          <label class="field full">
            <span>{{ secretLabel }}</span>
            <input v-model="secret" type="password" :placeholder="hasStoredSecret ? '已保存在系统钥匙串，留空即可' : '不会保存在浏览器中'" autocomplete="new-password">
          </label>
          <label class="toggle-row">
            <input v-model="draft.rememberSecret" type="checkbox"><span class="toggle"></span>
            <span><strong>记住凭据</strong><small>保存到操作系统钥匙串，不写入扩展存储</small></span>
          </label>
          <label class="toggle-row">
            <input v-model="draft.autoReconnect" type="checkbox"><span class="toggle"></span>
            <span><strong>自动重连</strong><small>断开后使用指数退避重新连接</small></span>
          </label>
        </fieldset>

        <fieldset :disabled="currentRunning || busy">
          <legend>网页快捷入口</legend>
          <label class="toggle-row">
            <input v-model="draft.webService" type="checkbox"><span class="toggle"></span>
            <span><strong>这是网页服务</strong><small>仅显示手动打开按钮，连接后不会自动跳转</small></span>
          </label>
          <label v-if="draft.webService" class="field full">
            <span>网页协议</span>
            <select v-model="draft.webScheme"><option value="http">HTTP</option><option value="https">HTTPS</option></select>
          </label>
        </fieldset>

        <div class="sticky-actions">
          <button class="button ghost" type="button" :disabled="busy || currentRunning" @click="saveCurrent">保存</button>
          <button v-if="!currentRunning" class="button primary" type="submit" :disabled="busy">保存并启动</button>
          <button v-else class="button danger" type="button" :disabled="busy" @click="stopProfile(draft.id)">停止隧道</button>
        </div>
        <button class="delete-button" type="button" :disabled="busy || currentRunning || !draft.id" @click="removeCurrent">删除这个配置</button>
      </form>
    </section>

    <Transition name="notice">
      <div v-if="notice.visible" class="notice" :data-kind="notice.kind">{{ notice.message }}</div>
    </Transition>
  </main>
</template>
