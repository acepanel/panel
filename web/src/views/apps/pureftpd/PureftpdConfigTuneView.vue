<script setup lang="ts">
defineOptions({
  name: 'pureftpd-config-tune'
})

import { useGettext } from 'vue3-gettext'

import pureftpd from '@/api/apps/pureftpd'

const { $gettext } = useGettext()

const maxClientsNumber = ref('')
const maxClientsPerIP = ref('')
const maxIdleTime = ref('')
const maxLoad = ref('')
const passivePortRange = ref('')
const anonymousOnly = ref('')
const noAnonymous = ref('')
const maxDiskUsage = ref('')

const saveLoading = ref(false)

const yesNoOptions = [
  { label: 'yes', value: 'yes' },
  { label: 'no', value: 'no' }
]

useRequest(pureftpd.configTune()).onSuccess(({ data }: any) => {
  maxClientsNumber.value = data.max_clients_number ?? ''
  maxClientsPerIP.value = data.max_clients_per_ip ?? ''
  maxIdleTime.value = data.max_idle_time ?? ''
  maxLoad.value = data.max_load ?? ''
  passivePortRange.value = data.passive_port_range ?? ''
  anonymousOnly.value = data.anonymous_only ?? ''
  noAnonymous.value = data.no_anonymous ?? ''
  maxDiskUsage.value = data.max_disk_usage ?? ''
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    pureftpd.saveConfigTune({
      max_clients_number: maxClientsNumber.value,
      max_clients_per_ip: maxClientsPerIP.value,
      max_idle_time: maxIdleTime.value,
      max_load: maxLoad.value,
      passive_port_range: passivePortRange.value,
      anonymous_only: anonymousOnly.value,
      no_anonymous: noAnonymous.value,
      max_disk_usage: maxDiskUsage.value
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
        <n-input v-model:value="maxClientsNumber" :placeholder="$gettext('e.g. 50')" />
      </n-form-item>
      <n-form-item label="MaxClientsPerIP">
        <n-input v-model:value="maxClientsPerIP" :placeholder="$gettext('e.g. 8')" />
      </n-form-item>
      <n-form-item :label="$gettext('MaxIdleTime (minutes)')">
        <n-input v-model:value="maxIdleTime" :placeholder="$gettext('e.g. 15')" />
      </n-form-item>
      <n-form-item label="MaxLoad">
        <n-input v-model:value="maxLoad" :placeholder="$gettext('e.g. 4')" />
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
        <n-input v-model:value="maxDiskUsage" :placeholder="$gettext('e.g. 99')" />
      </n-form-item>
    </n-form>
    <n-flex>
      <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>
