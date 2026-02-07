<script setup lang="ts">
defineOptions({
  name: 'pureftpd-config-tune'
})

import { useGettext } from 'vue3-gettext'

import pureftpd from '@/api/apps/pureftpd'

const { $gettext } = useGettext()

const maxClientsNumber = ref<number | null>(null)
const maxClientsPerIP = ref<number | null>(null)
const maxIdleTime = ref<number | null>(null)
const maxLoad = ref<number | null>(null)
const passivePortRange = ref('')
const anonymousOnly = ref('')
const noAnonymous = ref('')
const maxDiskUsage = ref<number | null>(null)

const saveLoading = ref(false)

const yesNoOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'no', value: 'no' }
]

useRequest(pureftpd.configTune()).onSuccess(({ data }: any) => {
  maxClientsNumber.value = Number(data.max_clients_number) || null
  maxClientsPerIP.value = Number(data.max_clients_per_ip) || null
  maxIdleTime.value = Number(data.max_idle_time) || null
  maxLoad.value = Number(data.max_load) || null
  passivePortRange.value = data.passive_port_range ?? ''
  anonymousOnly.value = data.anonymous_only ?? ''
  noAnonymous.value = data.no_anonymous ?? ''
  maxDiskUsage.value = Number(data.max_disk_usage) || null
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    pureftpd.saveConfigTune({
      max_clients_number: String(maxClientsNumber.value ?? ''),
      max_clients_per_ip: String(maxClientsPerIP.value ?? ''),
      max_idle_time: String(maxIdleTime.value ?? ''),
      max_load: String(maxLoad.value ?? ''),
      passive_port_range: passivePortRange.value,
      anonymous_only: anonymousOnly.value,
      no_anonymous: noAnonymous.value,
      max_disk_usage: String(maxDiskUsage.value ?? '')
    })
  )
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{ $gettext('Common Pure-FTPd settings.') }}
    </n-alert>
    <n-form>
      <n-form-item label="MaxClientsNumber">
        <n-input-number class="w-full" v-model:value="maxClientsNumber" :placeholder="$gettext('e.g. 50')" :min="1" />
      </n-form-item>
      <n-form-item label="MaxClientsPerIP">
        <n-input-number class="w-full" v-model:value="maxClientsPerIP" :placeholder="$gettext('e.g. 8')" :min="1" />
      </n-form-item>
      <n-form-item :label="$gettext('MaxIdleTime (minutes)')">
        <n-input-number class="w-full" v-model:value="maxIdleTime" :placeholder="$gettext('e.g. 15')" :min="0" />
      </n-form-item>
      <n-form-item label="MaxLoad">
        <n-input-number class="w-full" v-model:value="maxLoad" :placeholder="$gettext('e.g. 4')" :min="1" />
      </n-form-item>
      <n-form-item :label="$gettext('PassivePortRange (start end)')">
        <n-input v-model:value="passivePortRange" :placeholder="$gettext('e.g. 39000 40000')" />
      </n-form-item>
      <n-form-item label="AnonymousOnly">
        <n-select v-model:value="anonymousOnly" :options="yesNoOptions" />
      </n-form-item>
      <n-form-item label="NoAnonymous">
        <n-select v-model:value="noAnonymous" :options="yesNoOptions" />
      </n-form-item>
      <n-form-item :label="$gettext('MaxDiskUsage (%)')">
        <n-input-number class="w-full" v-model:value="maxDiskUsage" :placeholder="$gettext('e.g. 99')" :min="1" :max="100" />
      </n-form-item>
    </n-form>
    <n-flex>
      <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>
