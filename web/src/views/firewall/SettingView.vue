<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import safe from '@/api/panel/safe'

const { $gettext } = useGettext()
const model = ref({
  firewallStatus: false,
  pingStatus: false,
})

useRequest(firewall.status).onSuccess(({ data }) => {
  model.value.firewallStatus = data
})
useRequest(safe.pingStatus).onSuccess(({ data }) => {
  model.value.pingStatus = data
})

const handleFirewallStatus = () => {
  useRequest(firewall.updateStatus(model.value.firewallStatus)).onSuccess(() => {
    window.$message.success($gettext('Settings saved successfully'))
  })
}

const handlePingStatus = () => {
  useRequest(safe.updatePingStatus(model.value.pingStatus)).onSuccess(() => {
    window.$message.success($gettext('Settings saved successfully'))
  })
}

// 扫描感知设置
const scanEnabled = ref(false)
const saveDays = ref(30)
const selectedInterfaces = ref<string[]>([])
const interfaceOptions = ref<any[]>([])
const scanSettingLoading = ref(false)

// 自动屏蔽设置
const autoBlock = ref(false)
const blockThreshold = ref(100)
const blockWindow = ref(5)
const blockDuration = ref(0)
const whitelist = ref<string[]>([])

useRequest(firewall.scanSetting()).onSuccess(({ data }) => {
  scanEnabled.value = data.enabled
  saveDays.value = data.days || 30
  selectedInterfaces.value = data.interfaces || []
  autoBlock.value = data.auto_block || false
  blockThreshold.value = data.block_threshold || 100
  blockWindow.value = data.block_window || 5
  blockDuration.value = data.block_duration || 0
  whitelist.value = data.whitelist || []
})

useRequest(firewall.scanInterfaces()).onSuccess(({ data }) => {
  interfaceOptions.value = (data || []).map((iface: any) => ({
    label: `${iface.name} (${iface.ips?.join(', ') || iface.status})`,
    value: iface.name,
  }))
})

const handleSaveScanSetting = () => {
  scanSettingLoading.value = true
  useRequest(
    firewall.updateScanSetting({
      enabled: scanEnabled.value,
      days: saveDays.value,
      interfaces: selectedInterfaces.value,
      auto_block: autoBlock.value,
      block_threshold: blockThreshold.value,
      block_window: blockWindow.value,
      block_duration: blockDuration.value,
      whitelist: whitelist.value,
    }),
  )
    .onSuccess(() => {
      window.$message.success($gettext('Settings saved successfully'))
    })
    .onComplete(() => {
      scanSettingLoading.value = false
    })
}
</script>

<template>
  <n-form :model="model" label-placement="left" label-width="160">
    <n-form-item path="firewall" :label="$gettext('System Firewall')">
      <n-switch v-model:value="model.firewallStatus" @update:value="handleFirewallStatus" />
    </n-form-item>
    <n-form-item path="ping" :label="$gettext('Allow Ping')">
      <n-switch v-model:value="model.pingStatus" @update:value="handlePingStatus" />
    </n-form-item>
    <n-divider />
    <n-form-item :label="$gettext('Scan Awareness')">
      <n-switch v-model:value="scanEnabled" />
    </n-form-item>
    <template v-if="scanEnabled">
      <n-form-item :label="$gettext('Retention Days')">
        <n-input-number v-model:value="saveDays" :min="1" :max="365" w-full />
      </n-form-item>
      <n-form-item :label="$gettext('Network Interfaces')">
        <n-select
          v-model:value="selectedInterfaces"
          :options="interfaceOptions"
          multiple
          clearable
          :placeholder="$gettext('Auto detect')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Auto Block')">
        <n-switch v-model:value="autoBlock" />
      </n-form-item>
      <template v-if="autoBlock">
        <n-form-item :label="$gettext('Block Threshold')">
          <n-input-number v-model:value="blockThreshold" :min="1" :max="100000" w-full />
        </n-form-item>
        <n-form-item :label="$gettext('Time Window (minutes)')">
          <n-input-number v-model:value="blockWindow" :min="1" :max="1440" w-full />
        </n-form-item>
        <n-form-item :label="$gettext('Block Duration (hours)')">
          <n-input-number v-model:value="blockDuration" :min="0" :max="87600" w-full>
            <template #suffix>
              <n-text v-if="blockDuration === 0" depth="3">{{ $gettext('Permanent') }}</n-text>
            </template>
          </n-input-number>
        </n-form-item>
        <n-form-item :label="$gettext('IP Whitelist')">
          <n-dynamic-tags v-model:value="whitelist" />
        </n-form-item>
      </template>
    </template>
    <n-form-item>
      <n-button type="primary" :loading="scanSettingLoading" @click="handleSaveScanSetting">
        {{ $gettext('Save') }}
      </n-button>
    </n-form-item>
  </n-form>
</template>

<style scoped lang="scss"></style>
