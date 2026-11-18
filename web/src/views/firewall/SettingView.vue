<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import safe from '@/api/panel/safe'
import { useFormLayout } from '@/components/system/composables/useFormLayout'

const { $gettext } = useGettext()
const formLayout = useFormLayout(160)
const model = ref({
  firewallStatus: false,
  pingStatus: false,
})

useRequest(firewall.status).onSuccess(({ data }) => {
  model.value.firewallStatus = data.running
})
useRequest(safe.pingStatus).onSuccess(({ data }) => {
  model.value.pingStatus = data
})

const firewallLoading = ref(false)
const pingLoading = ref(false)

// 开关不用 v-model，请求成功后才切换，失败时停在原状态
const handleFirewallStatus = (value: boolean) => {
  firewallLoading.value = true
  useRequest(firewall.updateStatus(value))
    .onSuccess(() => {
      model.value.firewallStatus = value
      window.$message.success($gettext('Settings saved successfully'))
      window.$bus.emit('firewall:refresh')
    })
    .onComplete(() => (firewallLoading.value = false))
}

const handlePingStatus = (value: boolean) => {
  pingLoading.value = true
  useRequest(safe.updatePingStatus(value))
    .onSuccess(() => {
      model.value.pingStatus = value
      window.$message.success($gettext('Settings saved successfully'))
    })
    .onComplete(() => (pingLoading.value = false))
}

// 扫描感知设置
const scanEnabled = ref(false)
const saveDays = ref(30)
const selectedInterfaces = ref<string[]>([])
const interfaceOptions = ref<any[]>([])

// 自动屏蔽设置
const autoBlock = ref(false)
const blockThreshold = ref(100)
const blockWindow = ref(5)
const blockDuration = ref(0)
const blockOutbound = ref(false)
const whitelist = ref<string[]>([])

useRequest(firewall.scanSetting()).onSuccess(({ data }) => {
  scanEnabled.value = data.enabled
  saveDays.value = data.days || 30
  selectedInterfaces.value = data.interfaces || []
  autoBlock.value = data.auto_block || false
  blockThreshold.value = data.block_threshold || 100
  blockWindow.value = data.block_window || 5
  blockDuration.value = data.block_duration || 0
  blockOutbound.value = data.block_outbound || false
  whitelist.value = data.whitelist || []
})

useRequest(firewall.scanInterfaces()).onSuccess(({ data }) => {
  interfaceOptions.value = (data || []).map((iface: any) => ({
    label: `${iface.name} (${iface.ips?.join(', ') || iface.status})`,
    value: iface.name,
  }))
})

const handleSaveScanSetting = async () => {
  await firewall.updateScanSetting({
    enabled: scanEnabled.value,
    days: saveDays.value,
    interfaces: selectedInterfaces.value,
    auto_block: autoBlock.value,
    block_threshold: blockThreshold.value,
    block_window: blockWindow.value,
    block_duration: blockDuration.value,
    block_outbound: blockOutbound.value,
    whitelist: whitelist.value,
  })
  window.$message.success($gettext('Settings saved successfully'))
}
</script>

<template>
  <n-form :model="model" v-bind="formLayout">
    <n-form-item path="firewall" :label="$gettext('System Firewall')">
      <n-switch
        :value="model.firewallStatus"
        :loading="firewallLoading"
        @update:value="handleFirewallStatus"
      />
    </n-form-item>
    <n-form-item path="ping" :label="$gettext('Allow Ping')">
      <n-switch :value="model.pingStatus" :loading="pingLoading" @update:value="handlePingStatus" />
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
        <n-form-item :label="$gettext('Block Outbound')">
          <n-flex align="center">
            <n-switch v-model:value="blockOutbound" />
            <n-text depth="3">
              {{ $gettext('Also drop traffic from this server to blocked IPs') }}
            </n-text>
          </n-flex>
        </n-form-item>
        <n-form-item :label="$gettext('IP Whitelist')">
          <n-dynamic-tags v-model:value="whitelist" />
        </n-form-item>
      </template>
    </template>
    <n-form-item>
      <app-button type="primary" @click="handleSaveScanSetting">
        {{ $gettext('Save') }}
      </app-button>
    </n-form-item>
  </n-form>
</template>

<style scoped lang="scss"></style>
