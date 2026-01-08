<script setup lang="ts">
defineOptions({
  name: 'toolbox-system'
})

import { DateTime } from 'luxon'
import { useGettext } from 'vue3-gettext'

import system from '@/api/panel/toolbox-system'

const { $gettext } = useGettext()
const currentTab = ref('dns')
const dns1 = ref('')
const dns2 = ref('')
const swap = ref(0)
const swapFree = ref('')
const swapUsed = ref('')
const swapTotal = ref('')
const hostname = ref('')
const hosts = ref('')
const timezone = ref('')
const timezones = ref<any[]>([])
const time = ref(DateTime.now().toMillis())

const dnsManager = ref('')

useRequest(system.dns()).onSuccess(({ data }) => {
  dns1.value = data.dns?.[0] ?? ''
  dns2.value = data.dns?.[1] ?? ''
  dnsManager.value = data.manager ?? ''
})
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

const handleUpdateDNS = () => {
  useRequest(system.updateDns(dns1.value, dns2.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleUpdateSwap = () => {
  useRequest(system.updateSwap(swap.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleUpdateHost = async () => {
  await Promise.all([
    useRequest(system.updateHostname(hostname.value)),
    useRequest(system.updateHosts(hosts.value))
  ]).then(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleUpdateTime = async () => {
  await Promise.all([
    useRequest(system.updateTime(String(DateTime.fromMillis(time.value).toISO()))),
    useRequest(system.updateTimezone(timezone.value))
  ]).then(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleSyncTime = () => {
  useRequest(system.syncTime()).onSuccess(() => {
    window.$message.success($gettext('Synchronized successfully'))
  })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="dns" tab="DNS">
      <n-flex vertical>
        <n-alert v-if="dnsManager === 'resolv.conf'" type="warning">
          {{ $gettext('DNS modifications will revert to default after system restart.') }}
        </n-alert>
        <n-alert v-else type="info">
          {{
            $gettext('Current DNS manager: %{ manager }', {
              manager: dnsManager
            })
          }}
        </n-alert>
        <n-form>
          <n-form-item label="DNS1">
            <n-input v-model:value="dns1" :placeholder="$gettext('Enter primary DNS server')" />
          </n-form-item>
          <n-form-item label="DNS2">
            <n-input v-model:value="dns2" :placeholder="$gettext('Enter secondary DNS server')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" @click="handleUpdateDNS">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="swap" tab="SWAP">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext('Total %{ total }, used %{ used }, free %{ free }', {
              total: swapTotal,
              used: swapUsed,
              free: swapFree
            })
          }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('SWAP Size')">
            <n-input-number v-model:value="swap" />
            MB
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" @click="handleUpdateSwap">
            {{ $gettext('Save') }}
          </n-button>
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
      <n-button type="primary" @click="handleUpdateHost">
        {{ $gettext('Save') }}
      </n-button>
    </n-tab-pane>
    <n-tab-pane name="time" :tab="$gettext('Time')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'After manually changing the time, it may still be overwritten by system automatic time synchronization.'
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
        </n-form>
        <n-flex>
          <n-button type="primary" @click="handleUpdateTime">
            {{ $gettext('Save') }}
          </n-button>
          <n-button type="info" @click="handleSyncTime">
            {{ $gettext('Synchronize Time') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>
