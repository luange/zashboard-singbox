import type { Connection as SingboxConnectionRawMessage } from '@/gen/daemon/started_service_pb'

export type BackendType = 'clash' | 'singbox'

export type Backend = {
  // 后端登录类型:'clash' 走 Clash REST/WS API,'singbox' 走 sing-box API(gRPC)。
  // 旧记录缺省按 'clash' 迁移。
  type: BackendType
  protocol: string
  host: string
  port: string
  secondaryPath: string // 仅 clash
  password: string // 通用:Clash secret / sing-box gRPC Bearer token
  controllerToken?: string // optional zashboard-controller bearer token
  controllerPort?: string // optional independent controller port; defaults to 19091
  uuid: string
  label?: string
  disableUpgradeCore?: boolean // 仅 clash
  disableTunMode?: boolean // 仅 clash
}

export type Config = {
  port: number
  'socks-port': number
  'redir-port': number
  'tproxy-port': number
  'mixed-port': number
  'allow-lan': boolean
  'bind-address': string
  mode: string
  'mode-list': string[]
  modes: string[]
  'log-level': string
  ipv6: boolean
  tun: {
    enable: boolean
  }
}

export type History = {
  time: string
  delay: number
}[]

export type Proxy = {
  name: string
  type: string
  history: History
  extra: Record<
    string,
    {
      alive: boolean
      history: History
    }
  >
  all?: string[]
  udp?: boolean
  xudp?: boolean
  now: string
  fixed?: string
  icon: string
  hidden?: boolean
  selectable?: boolean
  testUrl?: string
  'dialer-proxy'?: string
  'provider-name'?: string
  smart?: SmartGroupStatus
  smart_mode?: 'auto' | 'pinned' | 'temporary' | 'warming'
}

export type SmartGroupStatus = {
  selected?: string
  pinned?: string
  temporary_override?: string
  reason?: string
  candidate_count: number
  state_counts: Record<string, number>
  switches_total?: number
  performance_switches?: number
  failure_failovers?: number
}

export type SubscriptionInfo = {
  Download?: number
  Upload?: number
  Total?: number
  Expire?: number
}

export type ProxyProvider = {
  subscriptionInfo?: SubscriptionInfo
  name: string
  proxies: Proxy[]
  testUrl: string
  updatedAt: string
  vehicleType: string
  paused?: boolean
  consumers?: number
  supportsPause?: boolean
}

export type Rule = {
  type: string
  payload: string
  proxy: string
  size: number
  uuid: string
  // sing-box-reFind
  disabled?: boolean
  // mihomo
  index: number
  extra?: {
    disabled: false
    hitAt: string
    hitCount: number
    missAt: string
    missCount: number
  }
}

export type RuleProvider = {
  behavior: string
  format: string
  name: string
  ruleCount: number
  type: string
  updatedAt: string
  vehicleType: string
}

export type ClashConnectionRawMessage = {
  id: string
  download: number
  upload: number
  chains: string[]
  rule: string
  rulePayload: string
  start: string | number
  metadata: {
    destinationGeoIP: string
    destinationIP: string
    destinationIPASN: string
    destinationPort: string
    dnsMode: string
    dscp: number
    host: string
    inboundIP: string
    inboundName: string
    inboundPort: string
    inboundUser: string
    network: string
    process: string
    processPath: string
    remoteDestination: string
    sniffHost: string
    sourceGeoIP: string
    sourceIP: string
    sourceIPASN: string
    sourcePort: string
    specialProxy: string
    specialRules: string
    type: string
    uid: number
    smartBlock: string
  }
}

export type ConnectionRawMessage = ClashConnectionRawMessage | SingboxConnectionRawMessage

export type Connection = ConnectionRawMessage & {
  downloadSpeed: number
  uploadSpeed: number
}

export type Log = {
  type: LOG_LEVEL
  payload: string
}

export type LogWithSeq = Log & { seq: number; time: string }

export type DNSQuery = {
  AD: boolean
  CD: boolean
  RA: boolean
  RD: boolean
  TC: boolean
  status: number
  Question: {
    Name: string
    Qtype: number
    Qclass: number
  }[]
  Answer?: {
    TTL: number
    data: string
    name: string
    type: number
  }[]
}

export type SourceIPLabel = {
  key: string
  label: string
  id: string
  scope?: string[]
}

// smart core
export interface NodeRank {
  Name: string
  Rank: string
  Weight: number
}
