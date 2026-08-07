import type { TunnelStatus } from './types'

interface NativeResponse<T = unknown> {
  id: string
  ok: boolean
  result?: T
  error?: {
    code: string
    message: string
    data?: unknown
  }
}

interface NativeEvent {
  event: string
  payload: unknown
}

interface PendingRequest {
  resolve: (value: unknown) => void
  reject: (error: NativeBridgeError) => void
  timer: number
}

type StatusListener = (status: TunnelStatus) => void
type ConnectionListener = (connected: boolean, error: string) => void

export class NativeBridgeError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly data?: unknown,
  ) {
    super(message)
    this.name = 'NativeBridgeError'
  }
}

class NativeBridge {
  private readonly port = chrome.runtime.connect({ name: 'tunneldeck-panel' })
  private readonly pending = new Map<string, PendingRequest>()
  private readonly statusListeners = new Set<StatusListener>()
  private readonly connectionListeners = new Set<ConnectionListener>()

  constructor() {
    this.port.onMessage.addListener((envelope: unknown) => this.handleEnvelope(envelope))
    this.port.onDisconnect.addListener(() => {
      const message = chrome.runtime.lastError?.message || '扩展后台连接已断开'
      this.rejectPending(new NativeBridgeError('BRIDGE_DISCONNECTED', message))
      this.emitConnection(false, message)
    })
  }

  request<T>(method: string, params?: unknown, timeoutMs = 20_000): Promise<T> {
    const id = crypto.randomUUID()
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        this.pending.delete(id)
        reject(new NativeBridgeError('REQUEST_TIMEOUT', `请求 ${method} 超时`))
      }, timeoutMs)
      this.pending.set(id, {
        resolve: value => resolve(value as T),
        reject,
        timer,
      })
      this.port.postMessage({ kind: 'request', request: { id, method, params } })
    })
  }

  onStatus(listener: StatusListener): () => void {
    this.statusListeners.add(listener)
    return () => this.statusListeners.delete(listener)
  }

  onConnection(listener: ConnectionListener): () => void {
    this.connectionListeners.add(listener)
    return () => this.connectionListeners.delete(listener)
  }

  private handleEnvelope(envelope: unknown) {
    if (!isRecord(envelope)) return
    if (envelope.kind === 'connection') {
      const connected = envelope.connected === true
      const error = typeof envelope.error === 'string' ? envelope.error : ''
      if (!connected && error) {
        this.rejectPending(new NativeBridgeError('NATIVE_HOST_UNAVAILABLE', error))
      }
      this.emitConnection(connected, error)
      return
    }
    if (envelope.kind !== 'native' || !isRecord(envelope.message)) return
    const message = envelope.message as unknown as NativeResponse | NativeEvent
    if ('event' in message) {
      if (message.event === 'tunnel.status' && isRecord(message.payload)) {
        for (const listener of this.statusListeners) listener(message.payload as unknown as TunnelStatus)
      }
      return
    }
    if (!('id' in message) || typeof message.id !== 'string') return
    const pending = this.pending.get(message.id)
    if (!pending) return
    window.clearTimeout(pending.timer)
    this.pending.delete(message.id)
    if (message.ok) {
      pending.resolve(message.result)
    } else {
      pending.reject(new NativeBridgeError(
        message.error?.code || 'NATIVE_ERROR',
        message.error?.message || '本地服务操作失败',
        message.error?.data,
      ))
    }
  }

  private rejectPending(error: NativeBridgeError) {
    for (const pending of this.pending.values()) {
      window.clearTimeout(pending.timer)
      pending.reject(error)
    }
    this.pending.clear()
  }

  private emitConnection(connected: boolean, error: string) {
    for (const listener of this.connectionListeners) listener(connected, error)
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export const nativeBridge = new NativeBridge()
