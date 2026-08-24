import { controlCoreAPI, fetchControllerStatusAPI, type ControllerStatus } from '@/api/controller'
import { ref } from 'vue'

export const controllerAvailable = ref(false)
export const controllerStatus = ref<ControllerStatus>()

export const probeController = async () => {
  try {
    const { data } = await fetchControllerStatusAPI()
    controllerAvailable.value = true
    controllerStatus.value = data
  } catch {
    controllerAvailable.value = false
    controllerStatus.value = undefined
  }
}

export const startCoreWithController = () => controlCoreAPI('start')
export const stopCoreWithController = () => controlCoreAPI('stop')
export const restartCoreWithController = () => controlCoreAPI('restart')
