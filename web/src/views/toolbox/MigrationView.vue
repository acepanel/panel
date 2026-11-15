<script setup lang="ts">
defineOptions({
  name: 'toolbox-migration',
})

import { useRequest } from 'alova/client'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NInput, NTag, NText } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import migration, {
  type MigrationConnection,
  type MigrationPanel,
  type MigrationResource,
  type MigrationResult,
} from '@/api/panel/toolbox-migration'
import ws from '@/api/ws'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import { formatBytes } from '@/utils/file'

const { $gettext } = useGettext()
const { isMobile } = useResponsive()

const step = ref(0)
const loading = ref(false)
const connection = ref<MigrationConnection>({
  source_panel: 'acepanel',
  url: '',
  token_id: 1,
  token: '',
  api_key: '',
})

const source = ref<Record<string, any> | null>(null)
const resources = ref<MigrationResource[]>([])
const selected = ref<string[]>([])
const page = ref(1)
const pageSize = ref(20)
const pagination = useTablePagination(page, pageSize, () => resources.value.length)
const formLayout = useFormLayout(120)
const targetPaths = ref<Record<string, string>>({})
const targetUsers = ref<Record<string, string>>({})
const skipBlocked = ref(false)
const stopSource = ref(false)

const logs = ref<string[]>([])
const results = ref<MigrationResult[]>([])
const running = ref(false)
const startedAt = ref<string | null>(null)
const endedAt = ref<string | null>(null)
const now = ref(Date.now())

let progressWs: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let clock: ReturnType<typeof setInterval> | null = null

const panels = computed(() => [
  {
    value: 'acepanel' as MigrationPanel,
    title: $gettext('AcePanel → AcePanel'),
    description: $gettext('Push websites, databases, users and projects to another AcePanel.'),
  },
  {
    value: 'baota' as MigrationPanel,
    title: $gettext('BaoTa → AcePanel'),
    description: $gettext('Pull websites, databases and projects from BT Panel.'),
  },
  {
    value: 'onepanel' as MigrationPanel,
    title: $gettext('1Panel → AcePanel'),
    description: $gettext('Pull websites and databases from 1Panel.'),
  },
])

const currentDescription = computed(
  () =>
    panels.value.find((panel) => panel.value === connection.value.source_panel)?.description ?? '',
)

const isPush = computed(() => connection.value.source_panel === 'acepanel')

const typeLabels = computed<Record<string, string>>(() => ({
  website: $gettext('Website'),
  database: $gettext('Database'),
  database_user: $gettext('Database User'),
  project: $gettext('Project'),
}))

const statusLabels = computed<Record<string, string>>(() => ({
  pending: $gettext('Pending'),
  running: $gettext('Running'),
  success: $gettext('Success'),
  partial: $gettext('Completed with warnings'),
  failed: $gettext('Failed'),
  skipped: $gettext('Skipped'),
}))

const stageLabels = computed<Record<string, string>>(() => ({
  backup: $gettext('Creating backup'),
  transfer: $gettext('Transferring'),
  import: $gettext('Importing'),
  done: $gettext('Done'),
}))

const blockedOf = (item: MigrationResource) => [
  ...item.blockers,
  ...item.depends_on
    .filter((key) => !selected.value.includes(key))
    .map(() => $gettext('a required dependency was not selected')),
]

const selectedBlocked = computed(
  () =>
    resources.value.filter((item) => selected.value.includes(item.key) && blockedOf(item).length)
      .length,
)

const stats = computed(() => {
  const counters = { success: 0, partial: 0, failed: 0, skipped: 0 }
  for (const result of results.value) {
    if (result.status in counters) counters[result.status as keyof typeof counters]++
  }
  return counters
})

const progress = computed(() => {
  if (!results.value.length) return 0
  const done = results.value.filter((item) => item.stage === 'done').length
  return Math.round((done / results.value.length) * 100)
})

const columns = computed<DataTableColumns<MigrationResource>>(() => [
  { type: 'selection', disabled: (row) => !skipBlocked.value && row.blockers.length > 0 },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: (row) => typeLabels.value[row.type] ?? row.type,
  },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    render: (row) =>
      h('div', null, [
        h('div', null, row.name),
        row.subtype ? h(NText, { depth: 3, style: 'font-size: 12px' }, () => row.subtype) : null,
      ]),
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 100,
    render: (row) =>
      h(NTag, { type: row.status === 'running' ? 'success' : 'default', size: 'small' }, () =>
        row.status === 'running' ? $gettext('Running') : $gettext('Stopped'),
      ),
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 100,
    render: (row) => (row.size > 0 ? formatBytes(row.size) : '—'),
  },
  {
    title: $gettext('Target'),
    key: 'target',
    minWidth: 260,
    render: (row) => {
      if (row.type !== 'project') return row.target_name
      return h('div', { style: 'display: flex; gap: 8px' }, [
        h(NInput, {
          size: 'small',
          value: targetPaths.value[row.key] ?? row.target_path,
          placeholder: $gettext('Target directory'),
          onUpdateValue: (value: string) => (targetPaths.value[row.key] = value),
        }),
        h(NInput, {
          size: 'small',
          style: 'width: 120px',
          value: targetUsers.value[row.key] ?? '',
          placeholder: $gettext('Run as'),
          onUpdateValue: (value: string) => (targetUsers.value[row.key] = value),
        }),
      ])
    },
  },
  {
    title: $gettext('Notes'),
    key: 'notes',
    minWidth: 240,
    render: (row) => {
      const notes = [
        ...blockedOf(row).map((text) => ({ type: 'error' as const, text })),
        ...row.warnings.map((text) => ({ type: 'warning' as const, text })),
      ]
      if (!notes.length) return '—'
      return h(
        'div',
        { style: 'display: flex; flex-direction: column; gap: 4px' },
        notes.map((note) =>
          h(NText, { type: note.type, style: 'font-size: 12px' }, () => note.text),
        ),
      )
    },
  },
])

const resultColumns = computed<DataTableColumns<MigrationResult>>(() => [
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: (row) => typeLabels.value[row.type] ?? row.type,
  },
  { title: $gettext('Name'), key: 'name', minWidth: 180 },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 160,
    render: (row) =>
      h(
        NTag,
        { type: statusType(row.status), size: 'small' },
        () => statusLabels.value[row.status],
      ),
  },
  {
    title: $gettext('Stage'),
    key: 'stage',
    width: 140,
    render: (row) => stageLabels.value[row.stage] ?? '—',
  },
  {
    title: $gettext('Duration'),
    key: 'duration',
    width: 100,
    render: (row) => (row.duration > 0 ? `${row.duration.toFixed(1)}s` : '—'),
  },
  {
    title: $gettext('Detail'),
    key: 'detail',
    minWidth: 260,
    render: (row) => {
      const lines = [
        ...(row.error ? [{ type: 'error' as const, text: row.error }] : []),
        ...row.warnings.map((text) => ({ type: 'warning' as const, text })),
      ]
      if (!lines.length) return '—'
      return h(
        'div',
        { style: 'display: flex; flex-direction: column; gap: 4px' },
        lines.map((line) =>
          h(NText, { type: line.type, style: 'font-size: 12px' }, () => line.text),
        ),
      )
    },
  },
])

const statusType = (status: string) => {
  if (status === 'success') return 'success'
  if (status === 'partial') return 'warning'
  if (status === 'failed') return 'error'
  if (status === 'running') return 'info'
  return 'default'
}

const formatDate = (value: string | null) => (value ? new Date(value).toLocaleString() : '—')

const formatDuration = (seconds: number) => {
  const hours = Math.floor(seconds / 3600)
  const minutes = String(Math.floor(seconds / 60) % 60)
  const rest = String(seconds % 60).padStart(2, '0')
  return hours > 0 ? `${hours}:${minutes.padStart(2, '0')}:${rest}` : `${minutes}:${rest}`
}

// 运行中显示实时累计耗时，结束后显示时间区间与总耗时
const timeLabel = computed(() => {
  if (!startedAt.value) return ''
  const started = formatDate(startedAt.value)
  const end = endedAt.value ? new Date(endedAt.value).getTime() : now.value
  const elapsed = formatDuration(
    Math.max(0, Math.round((end - new Date(startedAt.value).getTime()) / 1000)),
  )
  return endedAt.value
    ? `${started} — ${formatDate(endedAt.value)} (${elapsed})`
    : $gettext('%{ started }, running for %{ elapsed }', { started, elapsed })
})

const handleConnect = () => {
  if (!connection.value.url) {
    window.$message.error($gettext('Please enter the panel address.'))
    return
  }
  if (isPush.value && (!connection.value.token_id || !connection.value.token)) {
    window.$message.error($gettext('Please enter the AcePanel Token ID and access token.'))
    return
  }
  if (!isPush.value && !connection.value.api_key) {
    window.$message.error($gettext('Please enter the source panel API key.'))
    return
  }

  loading.value = true
  useRequest(migration.precheck(connection.value))
    .onSuccess(({ data }: any) => {
      source.value = data.source
      handleItems()
    })
    .onComplete(() => (loading.value = false))
}

const handleItems = () => {
  loading.value = true
  useRequest(migration.items())
    .onSuccess(({ data }: any) => {
      resources.value = (data.items || []).map((item: MigrationResource) => ({
        ...item,
        blockers: item.blockers || [],
        warnings: item.warnings || [],
        depends_on: item.depends_on || [],
      }))
      targetPaths.value = Object.fromEntries(
        resources.value.map((item) => [item.key, item.target_path || '']),
      )
      targetUsers.value = {}
      selected.value = []
      step.value = 1
    })
    .onComplete(() => (loading.value = false))
}

const handleStart = () => {
  if (!selected.value.length) {
    window.$message.warning($gettext('Please select at least one resource to migrate.'))
    return
  }
  if (selectedBlocked.value > 0 && !skipBlocked.value) {
    window.$message.error(
      $gettext('The selection contains blocked resources. Resolve them or enable skipping first.'),
    )
    return
  }

  window.$dialog.warning({
    title: $gettext('Start migration'),
    content: $gettext('Are you sure you want to start migrating the selected resources?'),
    positiveText: $gettext('Start migration'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      loading.value = true
      logs.value = []
      results.value = []
      useRequest(
        migration.start({
          items: selected.value.map((key) => ({
            key,
            target_path: targetPaths.value[key],
            target_user: targetUsers.value[key],
          })),
          skip_blocked: skipBlocked.value,
          stop_source: stopSource.value,
        }),
      )
        .onSuccess(() => {
          step.value = 2
          running.value = true
          connectProgress()
        })
        .onComplete(() => (loading.value = false))
    },
  })
}

const connectProgress = async () => {
  try {
    progressWs = await ws.migrationProgress()
    progressWs.onmessage = (event: MessageEvent) => {
      const data = JSON.parse(event.data)
      results.value = data.results || []
      startedAt.value = data.started_at
      endedAt.value = data.ended_at
      if (data.new_logs?.length) {
        logs.value.push(...data.new_logs)
        if (logs.value.length > 1500) logs.value = logs.value.slice(-1500)
      }
      if (data.step === 'done') {
        running.value = false
        step.value = 3
        closeProgress()
      }
    }
    progressWs.onclose = () => {
      progressWs = null
      if (running.value) reconnectTimer = setTimeout(connectProgress, 3000)
    }
  } catch {
    window.$message.error($gettext('Failed to subscribe to migration progress.'))
  }
}

const closeProgress = () => {
  if (progressWs) {
    progressWs.onclose = null
    progressWs.close()
    progressWs = null
  }
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

const handleReset = () => {
  window.$dialog.warning({
    title: $gettext('Start a new migration'),
    content: $gettext('The current migration results and logs will be cleared.'),
    positiveText: $gettext('Reset'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(migration.reset()).onSuccess(() => {
        closeProgress()
        step.value = 0
        connection.value = {
          source_panel: 'acepanel',
          url: '',
          token_id: 1,
          token: '',
          api_key: '',
        }
        source.value = null
        resources.value = []
        selected.value = []
        targetPaths.value = {}
        targetUsers.value = {}
        skipBlocked.value = false
        stopSource.value = false
        logs.value = []
        results.value = []
        startedAt.value = null
        endedAt.value = null
      })
    },
  })
}

// 迁移在后台执行，进入页面时恢复正在进行或已结束的任务
onMounted(() => {
  useRequest(migration.status()).onSuccess(({ data }: any) => {
    results.value = data.results || []
    logs.value = data.logs || []
    startedAt.value = data.started_at
    endedAt.value = data.ended_at
    if (data.step === 'running') {
      step.value = 2
      running.value = true
      connectProgress()
    } else if (data.step === 'done') {
      step.value = 3
    }
  })
})

// 迁移进行中才需要走时，结束后停掉避免空转
watch(
  running,
  (value) => {
    if (clock) {
      clearInterval(clock)
      clock = null
    }
    if (value) {
      now.value = Date.now()
      clock = setInterval(() => (now.value = Date.now()), 1000)
    }
  },
  { immediate: true },
)

onUnmounted(() => {
  closeProgress()
  if (clock) clearInterval(clock)
})

watch(
  () => connection.value.source_panel,
  () => {
    connection.value.url = ''
    connection.value.token = ''
    connection.value.api_key = ''
  },
)
</script>

<template>
  <n-flex vertical :size="16">
    <n-steps :current="step + 1" size="small" :vertical="isMobile">
      <n-step :title="$gettext('Connect')" />
      <n-step :title="$gettext('Select resources')" />
      <n-step :title="$gettext('Migrating')" />
      <n-step :title="$gettext('Done')" />
    </n-steps>

    <n-card v-if="step === 0" :title="$gettext('Migration source')">
      <n-flex vertical :size="16">
        <n-flex vertical :size="8">
          <n-select
            v-if="isMobile"
            v-model:value="connection.source_panel"
            :options="panels"
            label-field="title"
            size="large"
          />
          <n-radio-group v-else v-model:value="connection.source_panel" size="large">
            <n-radio-button v-for="panel in panels" :key="panel.value" :value="panel.value">
              {{ panel.title }}
            </n-radio-button>
          </n-radio-group>
          <n-text depth="3">{{ currentDescription }}</n-text>
        </n-flex>

        <n-form v-bind="formLayout">
          <n-form-item
            :label="isPush ? $gettext('Target address') : $gettext('Source address')"
            required
          >
            <n-input
              v-model:value="connection.url"
              placeholder="https://192.168.1.100:8888"
              @keydown.enter.prevent="handleConnect"
            />
          </n-form-item>
          <template v-if="isPush">
            <n-form-item :label="$gettext('Token ID')" required>
              <n-input-number v-model:value="connection.token_id" :min="1" w-full />
            </n-form-item>
            <n-form-item :label="$gettext('Access token')" required>
              <n-input
                v-model:value="connection.token"
                type="password"
                show-password-on="click"
                :placeholder="$gettext('Access token of the target AcePanel')"
              />
            </n-form-item>
          </template>
          <n-form-item v-else :label="$gettext('API key')" required>
            <n-input
              v-model:value="connection.api_key"
              type="password"
              show-password-on="click"
              :placeholder="$gettext('API key of the source panel')"
            />
          </n-form-item>
        </n-form>

        <n-alert type="info" :show-icon="false">
          {{
            isPush
              ? $gettext(
                  'The target address must include the access entrance and allow this server address.',
                )
              : $gettext('The source panel API must be enabled and allow this server address.')
          }}
        </n-alert>

        <n-flex justify="end">
          <n-button type="primary" :loading="loading" @click="handleConnect">
            {{ $gettext('Connect and read resources') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>

    <n-card v-if="step === 1" :title="$gettext('Select resources')">
      <template #header-extra>
        <n-flex align="center" :size="12">
          <n-text depth="3">
            {{ $gettext('Source: %{ panel }', { panel: source?.panel ?? '' }) }}
            {{ source?.version }}
          </n-text>
          <n-button size="small" :loading="loading" @click="handleItems">
            {{ $gettext('Refresh') }}
          </n-button>
        </n-flex>
      </template>
      <n-flex vertical :size="16">
        <n-data-table
          v-model:checked-row-keys="selected"
          v-model:page="page"
          v-model:pageSize="pageSize"
          :columns="columns"
          :data="resources"
          :row-key="(row: MigrationResource) => row.key"
          :pagination="pagination"
          size="small"
          striped
        />
        <n-flex align="center" :size="24">
          <n-checkbox v-model:checked="skipBlocked">
            {{ $gettext('Skip blocked resources during migration') }}
          </n-checkbox>
          <n-checkbox v-model:checked="stopSource">
            {{ $gettext('Stop running services while backups are created') }}
          </n-checkbox>
        </n-flex>
        <n-flex justify="space-between">
          <n-button @click="step = 0">{{ $gettext('Back') }}</n-button>
          <n-button type="primary" :loading="loading" @click="handleStart">
            {{ $gettext('Start migration (%{ count })', { count: String(selected.length) }) }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-card>

    <n-card v-if="step >= 2" :title="running ? $gettext('Migrating') : $gettext('Migration done')">
      <template #header-extra>
        <n-flex align="center" :size="12">
          <n-text depth="3">{{ timeLabel }}</n-text>
          <n-button v-if="!running" tag="a" :href="migration.logUrl" size="small">
            {{ $gettext('Download log') }}
          </n-button>
          <n-button v-if="!running" size="small" type="primary" @click="handleReset">
            {{ $gettext('Start a new migration') }}
          </n-button>
        </n-flex>
      </template>
      <n-flex vertical :size="16">
        <n-progress
          type="line"
          :percentage="progress"
          :status="stats.failed > 0 ? 'error' : running ? 'default' : 'success'"
        />
        <n-flex :size="12">
          <n-tag type="success" size="small">
            {{ $gettext('Success: %{ count }', { count: String(stats.success) }) }}
          </n-tag>
          <n-tag type="warning" size="small">
            {{ $gettext('With warnings: %{ count }', { count: String(stats.partial) }) }}
          </n-tag>
          <n-tag type="error" size="small">
            {{ $gettext('Failed: %{ count }', { count: String(stats.failed) }) }}
          </n-tag>
          <n-tag size="small">
            {{ $gettext('Skipped: %{ count }', { count: String(stats.skipped) }) }}
          </n-tag>
        </n-flex>
        <n-data-table
          :columns="resultColumns"
          :data="results"
          :row-key="(row: MigrationResult) => row.key"
          size="small"
          striped
        />
        <n-log :lines="logs" :rows="16" />
      </n-flex>
    </n-card>
  </n-flex>
</template>

<style scoped>
/* 末步去除空白 */
:deep(.n-step:last-child) {
  flex: 0 0 auto;
}
</style>
