export type AuthMode = 'password' | 'private-key'
export type TunnelState = 'stopped' | 'connecting' | 'running' | 'reconnecting' | 'error' | 'host-key-required'

export interface TunnelProfile {
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

export interface ProfileView extends TunnelProfile {
  hasStoredSecret: boolean
}

export interface HostKeyInfo {
  host: string
  port: number
  keyType: string
  fingerprint: string
}

export interface TunnelStatus {
  profileId: string
  state: TunnelState
  message: string
  localEndpoint: string
  remoteEndpoint: string
  activeConnections: number
  connectedAt?: string
  hostKey?: HostKeyInfo
}

export interface OperationResult {
  ok: boolean
  code?: string
  message?: string
  profile?: TunnelProfile
  status?: TunnelStatus
  hostKey?: HostKeyInfo
  url?: string
}

export interface BootstrapData {
  profiles: ProfileView[]
  statuses: TunnelStatus[]
  configPath: string
  knownHostsPath: string
  startupError?: string
}

export interface ParseCommandResult {
  ok: boolean
  code?: string
  message?: string
  profile?: TunnelProfile
}
