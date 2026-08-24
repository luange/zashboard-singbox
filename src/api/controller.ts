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
