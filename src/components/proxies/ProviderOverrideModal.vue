<template>
  <DialogWrapper
    v-model="providerOverrideModalOpen"
    :title="$t('providerOverride')"
    box-class="max-w-2xl"
  >
    <div class="flex flex-col gap-3">
      <label class="form-control gap-1">
        <span class="text-sm">{{ $t('providerName') }}</span>
        <input
          v-model.trim="form.tag"
          class="input input-bordered input-sm"
          :disabled="Boolean(providerOverrideTarget)"
          placeholder="airport-backup"
        />
      </label>
      <label class="form-control gap-1">
        <span class="text-sm">{{ $t('providerUrl') }}</span>
        <input
          v-model="form.url"
          class="input input-bordered input-sm font-mono"
          type="password"
          autocomplete="off"
          :placeholder="
            urlConfigured ? $t('providerSecretPreserved') : 'https://example.com/subscription'
          "
        />
      </label>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label class="form-control gap-1">
          <span class="text-sm">Format</span>
          <select
            v-model="form.format"
            class="select select-bordered select-sm"
          >
            <option value="clash">Clash</option>
            <option value="sing-box">sing-box</option>
          </select>
        </label>
        <label class="form-control gap-1">
          <span class="text-sm">{{ $t('updateInterval') }}</span>
          <input
            v-model="form.updateInterval"
            class="input input-bordered input-sm"
            placeholder="24h"
          />
        </label>
        <label class="form-control gap-1">
          <span class="text-sm">Download detour</span>
          <input
            v-model="form.downloadDetour"
            class="input input-bordered input-sm"
            placeholder="DIRECT"
          />
        </label>
      </div>
      <div class="border-base-content/10 rounded-lg border p-3">
        <label class="label cursor-pointer justify-start gap-3 p-0">
          <input
            v-model="form.healthEnabled"
            type="checkbox"
            class="toggle toggle-sm"
          />
          <span>{{ $t('healthCheck') }}</span>
        </label>
        <div
          v-if="form.healthEnabled"
          class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2"
        >
          <input
            v-model="form.healthUrl"
            class="input input-bordered input-sm font-mono"
            placeholder="https://www.gstatic.com/generate_204"
          />
          <input
            v-model="form.healthInterval"
            class="input input-bordered input-sm"
            placeholder="10m"
          />
        </div>
      </div>
      <div class="border-base-content/10 rounded-lg border p-3">
        <div class="mb-2 text-sm font-medium">{{ $t('attachProviderGroups') }}</div>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <label
            v-for="group in smartGroups"
            :key="group"
            class="label cursor-pointer justify-start gap-2 p-0"
          >
            <input
              v-model="form.attachTo"
              type="checkbox"
              class="checkbox checkbox-sm"
              :value="group"
            />
            <span>{{ group }}</span>
          </label>
        </div>
      </div>
      <div class="text-warning text-xs">{{ $t('providerOverrideRestartHint') }}</div>
      <div class="flex justify-end gap-2">
        <button
          v-if="providerOverrideTarget && hasOverride"
          class="btn btn-error btn-outline btn-sm"
          :disabled="saving"
          @click="restoreOriginal"
        >
          {{ $t('restoreOriginalProvider') }}
        </button>
        <button
          class="btn btn-primary btn-sm"
          :disabled="saving || !form.tag || (!providerOverrideTarget && !form.url)"
          @click="save"
        >
          <span
            v-if="saving"
            class="loading loading-spinner loading-sm"
          />
          {{ $t('save') }}
        </button>
      </div>
    </div>
  </DialogWrapper>
</template>

<script setup lang="ts">
import DialogWrapper from '@/components/common/DialogWrapper.vue'
import {
  deleteProviderOverrideAPI,
  fetchProviderOverridesAPI,
  putProviderOverrideAPI,
} from '@/api/controller'
import { fetchProxies, proxyGroupList, proxyMap } from '@/assembly/proxies'
import { providerOverrideModalOpen, providerOverrideTarget } from '@/composables/providerOverrides'
import { notifyRequestError } from '@/helper/requestError'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const saving = ref(false)
const hasOverride = ref(false)
const urlConfigured = ref(false)
const smartGroups = computed(() =>
  proxyGroupList.value.filter((name) => proxyMap.value[name]?.smart),
)
const form = reactive({
  tag: '',
  url: '',
  format: 'clash',
  updateInterval: '24h',
  downloadDetour: 'DIRECT',
  healthEnabled: true,
  healthUrl: 'https://www.gstatic.com/generate_204',
  healthInterval: '10m',
  attachTo: [] as string[],
})

const reset = () => {
  form.tag = providerOverrideTarget.value
  form.url = ''
  form.format = 'clash'
  form.updateInterval = '24h'
  form.downloadDetour = 'DIRECT'
  form.healthEnabled = true
  form.healthUrl = 'https://www.gstatic.com/generate_204'
  form.healthInterval = '10m'
  form.attachTo = smartGroups.value.slice()
  hasOverride.value = false
  urlConfigured.value = false
}

watch(providerOverrideModalOpen, async (open) => {
  if (!open) return
  reset()
  try {
    const { data } = await fetchProviderOverridesAPI()
    const override = data.providers[providerOverrideTarget.value]
    if (!override) return
    hasOverride.value = true
    urlConfigured.value = Boolean(override.definition.url_configured)
    form.format = override.definition.format || form.format
    form.updateInterval = override.definition.update_interval || form.updateInterval
    form.downloadDetour = override.definition.download_detour || form.downloadDetour
    form.healthEnabled = override.definition.health_check?.enabled ?? form.healthEnabled
    form.healthUrl = override.definition.health_check?.url || form.healthUrl
    form.healthInterval = override.definition.health_check?.interval || form.healthInterval
    form.attachTo = override.attach_to?.slice() || form.attachTo
  } catch (error) {
    notifyRequestError(error)
  }
})

const save = async () => {
  saving.value = true
  try {
    const definition: Record<string, unknown> = {
      type: 'remote',
      format: form.format,
      update_interval: form.updateInterval,
      download_detour: form.downloadDetour,
      health_check: {
        enabled: form.healthEnabled,
        url: form.healthUrl,
        interval: form.healthInterval,
      },
    }
    if (form.url) definition.url = form.url
    await putProviderOverrideAPI(form.tag, { definition, attach_to: form.attachTo })
    providerOverrideModalOpen.value = false
    window.setTimeout(fetchProxies, 1000)
  } catch (error) {
    notifyRequestError(error)
  } finally {
    saving.value = false
  }
}

const restoreOriginal = async () => {
  if (!window.confirm(t('restoreOriginalProviderConfirm'))) return
  saving.value = true
  try {
    await deleteProviderOverrideAPI(form.tag)
    providerOverrideModalOpen.value = false
    window.setTimeout(fetchProxies, 1000)
  } catch (error) {
    notifyRequestError(error)
  } finally {
    saving.value = false
  }
}
</script>
