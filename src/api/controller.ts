import { activeBackend } from '@/store/setup'
import axios from 'axios'

const controllerClient = axios.create()

const controllerBaseURL = () => {
  const backend = activeBackend.value
  if (!backend) return ''
  const port = backend.controllerPort || (backend.port === '19091' ? backend.port : '19091')
  return `${backend.protocol}://${backend.host}:${port}`
}

export type ControllerStatus = {
  active: boolean
  coreReachable: boolean
  version: string
  supervisor: string
}

const controllerHeaders = () => ({
  Authorization: `Bearer ${activeBackend.value?.controllerToken || activeBackend.value?.password || ''}`,
})

export const fetchControllerStatusAPI = () =>
  controllerClient.get<ControllerStatus>('/controller/v1/status', {
    baseURL: controllerBaseURL(),
    timeout: 2500,
  })

export const controlCoreAPI = (action: 'start' | 'stop' | 'restart') =>
  controllerClient.post(`/controller/v1/${action}`, undefined, {
    baseURL: controllerBaseURL(),
    headers: controllerHeaders(),
    timeout: 10000,
  })

export type ProviderOverride = {
  definition: {
    type?: 'remote' | 'local'
    format?: string
    path?: string
    update_interval?: string
    download_detour?: string
    url_configured?: boolean
    headers_configured?: boolean
    health_check?: {
      enabled?: boolean
      url?: string
      interval?: string
      timeout?: string
    }
  }
  attach_to?: string[]
  overridden?: boolean
}

export const fetchProviderOverridesAPI = () =>
  controllerClient.get<{ version: number; providers: Record<string, ProviderOverride> }>(
    '/controller/v1/provider-overrides',
    { baseURL: controllerBaseURL(), headers: controllerHeaders(), timeout: 5000 },
  )

export const putProviderOverrideAPI = (tag: string, override: ProviderOverride) =>
  controllerClient.put(`/controller/v1/provider-overrides/${encodeURIComponent(tag)}`, override, {
    baseURL: controllerBaseURL(),
    headers: controllerHeaders(),
    timeout: 30_000,
  })

export const deleteProviderOverrideAPI = (tag: string) =>
  controllerClient.delete(`/controller/v1/provider-overrides/${encodeURIComponent(tag)}`, {
    baseURL: controllerBaseURL(),
    headers: controllerHeaders(),
    timeout: 30_000,
  })
