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

const sleep = (milliseconds: number) =>
  new Promise((resolve) => window.setTimeout(resolve, milliseconds))

const waitForCoreState = async (active: boolean, timeout = 15_000) => {
  const deadline = Date.now() + timeout
  let lastStatus: ControllerStatus | undefined

  while (Date.now() < deadline) {
    try {
      const { data } = await fetchControllerStatusAPI()
      lastStatus = data
      controllerAvailable.value = true
      controllerStatus.value = data
      if (data.active === active && (!active || data.coreReachable)) return data
    } catch {
      // The controller is independent from the core. Retry transient failures,
      // but never report a lifecycle action as successful without evidence.
    }
    await sleep(250)
  }

  throw new Error(
    `controller timed out waiting for core ${active ? 'start' : 'stop'}; last status=${JSON.stringify(lastStatus)}`,
  )
}

const controlAndWait = async (action: 'start' | 'stop' | 'restart') => {
  await controlCoreAPI(action)
  return waitForCoreState(action !== 'stop')
}

export const startCoreWithController = () => controlAndWait('start')
export const stopCoreWithController = () => controlAndWait('stop')
export const restartCoreWithController = () => controlAndWait('restart')
