const nativeHostName = 'com.tunneldeck.native'
const panelPortName = 'tunneldeck-panel'

interface PanelRequest {
  kind: 'request'
  request: {
    id: string
    method: string
    params?: unknown
  }
}

const panels = new Set<chrome.runtime.Port>()
let nativePort: chrome.runtime.Port | null = null
let nativeConnected = false
let lastConnectionError = ''

function broadcast(message: unknown) {
  for (const panel of panels) {
    try {
      panel.postMessage(message)
    } catch {
      panels.delete(panel)
    }
  }
}

function connectNative(): chrome.runtime.Port {
  if (nativePort) return nativePort

  const port = chrome.runtime.connectNative(nativeHostName)
  nativePort = port
  nativeConnected = true
  lastConnectionError = ''
  broadcast({ kind: 'connection', connected: true })

  port.onMessage.addListener((message: unknown) => {
    broadcast({ kind: 'native', message })
  })
  port.onDisconnect.addListener(() => {
    const error = chrome.runtime.lastError?.message || 'TunnelDeck 本地服务已断开'
    if (nativePort === port) nativePort = null
    nativeConnected = false
    lastConnectionError = error
    broadcast({ kind: 'connection', connected: false, error })
  })
  return port
}

chrome.runtime.onConnect.addListener((panel) => {
  if (panel.name !== panelPortName) return
  panels.add(panel)
  panel.postMessage({
    kind: 'connection',
    connected: nativeConnected,
    error: lastConnectionError,
  })
  panel.onMessage.addListener((message: PanelRequest) => {
    if (message?.kind !== 'request' || !message.request) return
    try {
      connectNative().postMessage(message.request)
    } catch (error) {
      const description = error instanceof Error ? error.message : String(error)
      panel.postMessage({ kind: 'connection', connected: false, error: description })
    }
  })
  panel.onDisconnect.addListener(() => panels.delete(panel))
})

async function configureSidePanel() {
  await chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true })
}

chrome.runtime.onInstalled.addListener(() => {
  void configureSidePanel()
})

void configureSidePanel()
