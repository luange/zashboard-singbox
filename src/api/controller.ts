import { activeBackend } from '@/store/setup'
import axios from 'axios'

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
  axios.get<ControllerStatus>('/controller/v1/status', { timeout: 2500 })

export const controlCoreAPI = (action: 'start' | 'stop' | 'restart') =>
  axios.post(`/controller/v1/${action}`, undefined, {
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
}

export const fetchProviderOverridesAPI = () =>
  axios.get<{ version: number; providers: Record<string, ProviderOverride> }>(
    '/controller/v1/provider-overrides',
    { headers: controllerHeaders(), timeout: 5000 },
  )

export const putProviderOverrideAPI = (tag: string, override: ProviderOverride) =>
  axios.put(`/controller/v1/provider-overrides/${encodeURIComponent(tag)}`, override, {
    headers: controllerHeaders(),
    timeout: 30_000,
  })

export const deleteProviderOverrideAPI = (tag: string) =>
  axios.delete(`/controller/v1/provider-overrides/${encodeURIComponent(tag)}`, {
    headers: controllerHeaders(),
    timeout: 30_000,
  })
