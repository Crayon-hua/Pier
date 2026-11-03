export interface DeployState {
  currentVersion: string
  previousVersion: string
  deployedAt: string
  type?: string
  status?: string
}
export interface Project {
  id: string
  name: string
  description: string
  type: string
  repository: string
  asset: string
  configYaml: string
  hasToken: boolean
  createdAt: string
  root: string
  state?: DeployState
  busy: boolean
  diskBytes: number
  stateError?: string
  versions?: { version: string; type: string; createdAt: string; bytes: number }[]
  backups?: { name: string; bytes: number; createdAt: string }[]
  cleanup?: {
    BundleBytes: number
    OldReleaseBytes: number
    TmpBytes: number
    ProtectedVersions: string[]
    ActiveVersionUnknown: boolean
  }
  versionsError?: string
  backupsError?: string
  cleanupError?: string
}
export interface Task {
  id: string
  projectId: string
  projectName: string
  kind: string
  target: string
  status: string
  error?: string
  createdAt: string
  startedAt?: string
  finishedAt?: string
}
export interface Release {
  tag_name: string
  name: string
  prerelease: boolean
  assets: { name: string; size: number }[]
}
export interface System {
  os: string
  arch: string
  dataDir: string
  version: string
  maxUploadMB: number
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData))
    headers.set('Content-Type', 'application/json')
  const response = await fetch(`/api/v1${path}`, {
    ...options,
    headers,
    credentials: 'same-origin',
  })
  const data = await response.json().catch(() => ({ message: `请求失败 (${response.status})` }))
  if (!response.ok) {
    if (response.status === 401 && !path.startsWith('/auth/'))
      window.dispatchEvent(new Event('pier:unauthorized'))
    throw new Error(data.message || `请求失败 (${response.status})`)
  }
  return data as T
}
export const post = <T>(path: string, data: unknown = {}) =>
  request<T>(path, { method: 'POST', body: JSON.stringify(data) })
export const typeNames: Record<string, string> = {
  static: '静态站点',
  docker: 'Docker',
  system: '系统服务',
}
export const kindNames: Record<string, string> = {
  deploy: '部署',
  rollback: '版本回退',
  backup: '备份',
  clean: '清理',
}
export const statuses: Record<string, { label: string; color: string }> = {
  queued: { label: '等待中', color: 'gray' },
  running: { label: '执行中', color: 'arcoblue' },
  succeeded: { label: '已完成', color: 'green' },
  failed: { label: '失败', color: 'red' },
  interrupted: { label: '结果待确认', color: 'orange' },
}
export function bytes(n = 0): string {
  if (n < 1024) return `${n} B`
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), 4)
  return `${(n / 1024 ** i).toFixed(1)} ${['B', 'KB', 'MB', 'GB', 'TB'][i]}`
}
export function date(value?: string): string {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
}
export function duration(t: Task): string {
  if (!t.startedAt) return '—'
  const s = Math.max(
    0,
    Math.round(
      ((t.finishedAt ? new Date(t.finishedAt).getTime() : Date.now()) -
        new Date(t.startedAt).getTime()) /
        1000,
    ),
  )
  return s < 60 ? `${s} 秒` : `${Math.floor(s / 60)} 分 ${s % 60} 秒`
}
export function message(e: unknown): string {
  return e instanceof Error ? e.message : '操作失败'
}
