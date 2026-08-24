import { ref } from 'vue'

export const providerOverrideModalOpen = ref(false)
export const providerOverrideTarget = ref('')

export const openProviderOverride = (tag = '') => {
  providerOverrideTarget.value = tag
  providerOverrideModalOpen.value = true
}
