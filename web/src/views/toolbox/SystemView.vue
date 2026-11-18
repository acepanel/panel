<script setup lang="ts">
defineOptions({
  name: 'toolbox-system',
})

import { DateTime } from 'luxon'
import { useGettext } from 'vue3-gettext'

import system from '@/api/panel/toolbox-system'
import { useResponsive } from '@/components/system/composables/useResponsive'

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const currentTab = ref('swap')
const swap = ref(0)
const swapFree = ref('')
const swapUsed = ref('')
const swapTotal = ref('')
const hostname = ref('')
const hosts = ref('')
const timezone = ref('')
const timezones = ref<any[]>([])
const time = ref(DateTime.now().toMillis())
const syncServer = ref('')
const ntpServers = ref<string[]>([])
const builtinNtpServers = ref<string[]>([])
const ntpServiceType = ref('')
const showNtpModal = ref(false)
const editingNtpServers = ref<string[]>([])

useRequest(system.swap()).onSuccess(({ data }) => {
  swap.value = data.size
  swapFree.value = data.free
  swapUsed.value = data.used
  swapTotal.value = data.total
})
useRequest(system.hostname()).onSuccess(({ data }) => {
  hostname.value = data
})
useRequest(system.hosts()).onSuccess(({ data }) => {
  hosts.value = data
})
useRequest(system.timezone()).onSuccess(({ data }) => {
  timezone.value = data.timezone
  timezones.value = data.timezones
})
useRequest(system.ntpServers()).onSuccess(({ data }) => {
  ntpServers.value = data.servers || []
  builtinNtpServers.value = data.builtins || []
  ntpServiceType.value = data.service_type || ''
})

const handleUpdateSwap = async () => {
  await system.updateSwap(swap.value)
  window.$message.success($gettext('Saved successfully'))
}

const handleUpdateHost = async () => {
  await Promise.all([system.updateHostname(hostname.value), system.updateHosts(hosts.value)])
  window.$message.success($gettext('Saved successfully'))
}

const handleUpdateTime = async () => {
  await Promise.all([
    system.updateTime(String(DateTime.fromMillis(time.value).toISO())),
    system.updateTimezone(timezone.value),
  ])
  window.$message.success($gettext('Saved successfully'))
}

const handleSyncTime = async () => {
  await system.syncTime(syncServer.value || undefined)
  window.$message.success($gettext('Synchronized successfully'))
}

const handleOpenNtpSettings = () => {
  editingNtpServers.value = [...ntpServers.value]
  showNtpModal.value = true
}

const handleAddNtpServer = () => {
  editingNtpServers.value.push('')
}

const handleRemoveNtpServer = (index: number) => {
  editingNtpServers.value.splice(index, 1)
}

const handleResetNtpServers = () => {
  editingNtpServers.value = [...builtinNtpServers.value]
}

const saveNtpServers = async (servers: string[]) => {
  await system.updateNtpServers(servers)
  ntpServers.value = servers
  showNtpModal.value = false
  window.$message.success($gettext('Saved successfully'))
}

// 校验不通过时同步返回，避免保存按钮闪一下加载图标
const handleSaveNtpServers = () => {
  // 过滤空字符串
  const servers = editingNtpServers.value.filter((s) => s.trim() !== '')
  if (servers.length === 0) {
    window.$message.error($gettext('At least one NTP server is required'))
    return
  }
  return saveNtpServers(servers)
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" :placement="isMobile ? 'top' : 'left'" animated>
    <n-tab-pane name="swap" tab="SWAP">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext('System total %{ total }, used %{ used }, free %{ free }', {
              total: swapTotal,
              used: swapUsed,
              free: swapFree,
            })
          }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Panel SWAP Size')">
            <n-input-number v-model:value="swap" />
            MB
          </n-form-item>
        </n-form>
        <n-flex>
          <app-button type="primary" @click="handleUpdateSwap">
            {{ $gettext('Save') }}
          </app-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="host" :tab="$gettext('Host')">
      <n-form>
        <n-form-item :label="$gettext('System Hostname')">
          <n-input
            v-model:value="hostname"
            :placeholder="$gettext('Enter hostname, e.g. myserver')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Hosts')">
          <common-editor v-model:value="hosts" height="60vh" />
        </n-form-item>
      </n-form>
      <app-button type="primary" @click="handleUpdateHost">
        {{ $gettext('Save') }}
      </app-button>
    </n-tab-pane>
    <n-tab-pane name="time" :tab="$gettext('Time')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'After manually changing the time, it may still be overwritten by system automatic time synchronization.',
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Select Timezone')">
            <n-select
              v-model:value="timezone"
              :placeholder="$gettext('Please select a timezone')"
              :options="timezones"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Modify Time')">
            <n-date-picker v-model:value="time" type="datetime" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('NTP Server')">
            <n-flex :size="8" align="center" style="width: 100%">
              <n-input
                v-model:value="syncServer"
                :placeholder="$gettext('Optional, leave empty to use default servers')"
                style="flex: 1"
              />
              <n-button @click="handleOpenNtpSettings">
                {{ $gettext('Configure Default Servers') }}
              </n-button>
            </n-flex>
          </n-form-item>
        </n-form>
        <n-flex>
          <app-button type="primary" @click="handleUpdateTime">
            {{ $gettext('Save') }}
          </app-button>
          <app-button type="info" @click="handleSyncTime">
            {{ $gettext('Synchronize Time') }}
          </app-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>

  <!-- NTP 服务器配置弹窗 -->
  <app-modal v-model:show="showNtpModal" :title="$gettext('System NTP Server Configuration')">
    <n-flex vertical>
      <n-alert v-if="ntpServiceType === 'unknown'" type="warning">
        {{
          $gettext(
            'Unable to detect NTP service. Please ensure chrony or systemd-timesyncd is installed.',
          )
        }}
      </n-alert>
      <n-alert v-else type="info" :show-icon="false">
        {{
          $gettext(
            'Current NTP service: %{ service }. Changes will be applied to system configuration.',
            {
              service: ntpServiceType === 'chrony' ? 'Chrony' : 'systemd-timesyncd',
            },
          )
        }}
      </n-alert>
      <n-list>
        <n-list-item v-for="(_, index) in editingNtpServers" :key="index">
          <n-flex :size="8" align="center">
            <n-input
              v-model:value="editingNtpServers[index]"
              :placeholder="$gettext('Enter NTP server address')"
              style="flex: 1"
            />
            <n-button
              quaternary
              type="error"
              :disabled="editingNtpServers.length <= 1"
              @click="handleRemoveNtpServer(index)"
            >
              <template #icon>
                <i-mdi-delete />
              </template>
            </n-button>
          </n-flex>
        </n-list-item>
      </n-list>
      <n-flex justify="space-between">
        <n-flex :size="8">
          <n-button @click="handleAddNtpServer">
            <template #icon>
              <i-mdi-plus />
            </template>
            {{ $gettext('Add') }}
          </n-button>
          <n-button @click="handleResetNtpServers">
            {{ $gettext('Reset to Default') }}
          </n-button>
        </n-flex>
        <app-button type="primary" @click="handleSaveNtpServers">
          {{ $gettext('Save') }}
        </app-button>
      </n-flex>
    </n-flex>
  </app-modal>
</template>
