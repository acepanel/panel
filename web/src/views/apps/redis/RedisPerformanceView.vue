<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import type redis from '@/api/apps/redis'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'

const props = defineProps<{
  api: typeof redis
}>()

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const { data: slowLog, send: refreshSlowLog } = useRequest(props.api.slowLog, {
  initialData: [],
})
const { data: clients, send: refreshClients } = useRequest(props.api.clients, {
  initialData: [],
})
const { data: memory, send: refreshMemory } = useRequest(props.api.memory, {
  initialData: { doctor: '', no_limit: false, items: [] },
})

const slowLogColumns: any = [
  { title: 'ID', key: 'id', width: 90, sorter: (a: any, b: any) => a.id - b.id },
  { title: $gettext('Time'), key: 'time', width: 180, sorter: 'default' },
  {
    title: $gettext('Duration (ms)'),
    key: 'duration_us',
    width: 150,
    sorter: (a: any, b: any) => a.duration_us - b.duration_us,
    render: (row: any) => Math.round(row.duration_us / 10) / 100,
  },
  { title: $gettext('Command'), key: 'command', minWidth: 300, ellipsis: { tooltip: true } },
  { title: $gettext('Client'), key: 'client', width: 160, ellipsis: { tooltip: true } },
]

const clientColumns: any = [
  { title: 'ID', key: 'id', width: 90 },
  { title: $gettext('Address'), key: 'addr', width: 170, ellipsis: { tooltip: true } },
  { title: $gettext('Name'), key: 'name', width: 120, ellipsis: { tooltip: true } },
  { title: $gettext('Database'), key: 'db', width: 100 },
  {
    title: $gettext('Age (s)'),
    key: 'age',
    width: 130,
    sorter: (a: any, b: any) => Number(a.age) - Number(b.age),
  },
  {
    title: $gettext('Idle (s)'),
    key: 'idle',
    width: 130,
    sorter: (a: any, b: any) => Number(a.idle) - Number(b.idle),
  },
  { title: $gettext('Command'), key: 'cmd', minWidth: 150, ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 110,
    render(row: any) {
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              title: $gettext('Confirm Kill'),
              content: $gettext('Are you sure you want to kill connection %{ addr }?', {
                addr: row.addr,
              }),
              positiveText: $gettext('Kill'),
            })
            if (ok) await handleKillClient(Number(row.id))
          },
        },
        { default: () => $gettext('Kill') },
      )
    },
  },
]

const memoryColumns: any = [
  { title: $gettext('Property'), key: 'name', minWidth: 200, ellipsis: { tooltip: true } },
  { title: $gettext('Current Value'), key: 'value', minWidth: 200, ellipsis: { tooltip: true } },
]

const handleResetSlowLog = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Reset'),
    content: $gettext('Are you sure you want to reset the slow log?'),
  })
  if (!ok) return
  await props.api.resetSlowLog()
  window.$message.success($gettext('Reset successfully'))
  refreshSlowLog()
}

const handleKillClient = async (id: number) => {
  await props.api.killClient(id)
  window.$message.success($gettext('Killed successfully'))
  refreshClients()
}

const handleScanBigKeys = async () => {
  const ok = await confirmAction({
    type: 'info',
    title: $gettext('Confirm Scan'),
    content: $gettext(
      'Scanning traverses all keys and may take a while on large datasets. The result will be in the task log.',
    ),
  })
  if (!ok) return
  await props.api.scanBigKeys()
  window.$message.success($gettext('Task submitted, please check progress in background tasks'))
}
</script>

<template>
  <n-tabs type="segment" animated>
    <n-tab-pane name="slow-log" :tab="$gettext('Slow Log')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshSlowLog()">
            {{ $gettext('Refresh') }}
          </app-button>
          <app-button type="warning" @click="handleResetSlowLog">
            {{ $gettext('Reset Slow Log') }}
          </app-button>
        </n-flex>
        <n-data-table
          striped
          :columns="slowLogColumns"
          :data="slowLog"
          :row-key="(row: any) => row.id"
          :scroll-x="880"
          max-height="60vh"
          virtual-scroll
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="clients" :tab="$gettext('Clients')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshClients()">
            {{ $gettext('Refresh') }}
          </app-button>
        </n-flex>
        <n-data-table
          striped
          :columns="clientColumns"
          :data="clients"
          :scroll-x="1010"
          max-height="60vh"
        />
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="memory" :tab="$gettext('Memory')">
      <n-flex vertical>
        <n-flex>
          <app-button type="primary" @click="() => refreshMemory()">
            {{ $gettext('Refresh') }}
          </app-button>
          <app-button type="info" @click="handleScanBigKeys">
            {{ $gettext('Scan Big Keys') }}
          </app-button>
        </n-flex>
        <n-alert v-if="memory.no_limit" type="warning">
          {{
            $gettext(
              'maxmemory is not configured, memory usage can grow without limit. Set a memory limit and eviction policy in Parameter Tuning.',
            )
          }}
        </n-alert>
        <n-alert v-if="memory.doctor" type="info">
          {{ memory.doctor }}
        </n-alert>
        <n-data-table striped :columns="memoryColumns" :data="memory.items" :scroll-x="400" />
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>
