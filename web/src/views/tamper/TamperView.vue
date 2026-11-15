<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import tamper from '@/api/panel/tamper'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { formatDateTime } from '@/utils'
import RuleModal from '@/views/tamper/RuleModal.vue'

const { $gettext } = useGettext()
const { confirmAction, confirmDelete } = useConfirm()
const formLayout = useFormLayout(140)
const { isMobile } = useResponsive()

const currentTab = ref('setting')

// 状态与环境检测
const supported = ref(true)
const ebpf = ref<any>({
  available: false,
  kernel_version: '',
  bpf_lsm_active: false,
  active_lsm: '',
  reason: '',
})
const stats = ref<any>({ running: false, protected_files: 0, protected_dirs: 0 })

// 全局设置
const setting = ref({
  enabled: false,
  mode: 'chattr',
  block_new_files: false,
  log_days: 30,
})
const settingLoading = ref(false)
const activating = ref(false)

const loadStatus = () => {
  useRequest(tamper.status()).onSuccess(({ data }: any) => {
    supported.value = data.supported
    ebpf.value = data.ebpf
    stats.value = data.stats
    setting.value = data.setting
  })
}
loadStatus()

const handleSaveSetting = () => {
  // 选择 eBPF 但不可用时阻止
  if (setting.value.enabled && setting.value.mode === 'ebpf' && !ebpf.value.available) {
    window.$message.error($gettext('eBPF mode is not available, please activate it first'))
    return
  }
  settingLoading.value = true
  useRequest(tamper.saveSetting(setting.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      loadStatus()
    })
    .onComplete(() => {
      settingLoading.value = false
    })
}

// 激活 eBPF:改 grub 并重启系统
const handleActivateEBPF = async () => {
  const ok = await confirmDelete({
    title: $gettext('Activate eBPF LSM'),
    content: $gettext(
      'The panel will modify the kernel boot parameters (append bpf to the LSM list) and reboot the server immediately. All services will be interrupted during the reboot. Continue?',
    ),
    positiveText: $gettext('Continue'),
    countdown: 5,
  })
  if (!ok) return
  activating.value = true
  useRequest(tamper.activateEBPF())
    .onSuccess(() => {
      window.$message.success($gettext('The server is rebooting, please reconnect later'))
    })
    .onComplete(() => {
      activating.value = false
    })
}

// 规则
const ruleModalShow = ref(false)
const editingRule = ref<any>(null)

const {
  loading: rulesLoading,
  data: rules,
  page: rulePage,
  pageSize: rulePageSize,
  refresh: refreshRules,
  pagination: rulePagination,
} = usePagedTable((page, pageSize) => tamper.rules(page, pageSize), {
  pageSizes: [20, 50, 100],
})

const handleAddRule = () => {
  editingRule.value = null
  ruleModalShow.value = true
}
const handleEditRule = (row: any) => {
  editingRule.value = row
  ruleModalShow.value = true
}
const handleDeleteRule = async (row: any) => {
  const ok = await confirmDelete({
    content: $gettext('Are you sure to delete this rule?'),
  })
  if (!ok) return
  useRequest(tamper.deleteRule(row.id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refreshRules()
  })
}

const ruleColumns: any = [
  { title: $gettext('Name'), key: 'name', width: 160, ellipsis: { tooltip: true } },
  { title: $gettext('Protected Directory'), key: 'path', ellipsis: { tooltip: true } },
  {
    title: $gettext('Extensions'),
    key: 'exts',
    width: 200,
    render(row: any) {
      if (!row.exts || row.exts.length === 0)
        return h(NTag, { size: 'small', type: 'warning' }, () => $gettext('All files'))
      return row.exts.map((e: string) => h(NTag, { size: 'small', style: 'margin: 2px' }, () => e))
    },
  },
  {
    title: $gettext('Status'),
    key: 'enabled',
    width: 90,
    render(row: any) {
      return h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default' }, () =>
        row.enabled ? $gettext('Enabled') : $gettext('Disabled'),
      )
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 140,
    align: 'center',
    render(row: any) {
      return h('div', { style: 'display:flex;gap:8px;justify-content:center' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => handleEditRule(row) }, () =>
          $gettext('Edit'),
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            secondary: true,
            onClick: () => handleDeleteRule(row),
          },
          () => $gettext('Delete'),
        ),
      ])
    },
  },
]

// 日志
const {
  loading: logsLoading,
  data: logs,
  page: logPage,
  pageSize: logPageSize,
  refresh: refreshLogs,
  pagination: logPagination,
} = usePagedTable((page, pageSize) => tamper.logs(page, pageSize))

const opTypeMap: Record<string, 'error' | 'warning' | 'info' | 'default'> = {
  write: 'error',
  unlink: 'error',
  rename: 'warning',
  setattr: 'warning',
  create: 'info',
}

const logColumns: any = [
  {
    title: $gettext('Time'),
    key: 'created_at',
    width: 180,
    render: (row: any) => formatDateTime(row.created_at),
  },
  {
    title: $gettext('Operation'),
    key: 'op',
    width: 110,
    render(row: any) {
      return h(NTag, { size: 'small', type: opTypeMap[row.op] || 'default' }, () => row.op)
    },
  },
  { title: $gettext('Path'), key: 'path', ellipsis: { tooltip: true } },
  { title: $gettext('Process'), key: 'comm', width: 140, ellipsis: { tooltip: true } },
  { title: 'PID', key: 'pid', width: 90 },
]

const handleClearLogs = async () => {
  const ok = await confirmAction({
    title: $gettext('Clear Logs'),
    content: $gettext('Are you sure to clear all interception logs?'),
  })
  if (!ok) return
  useRequest(tamper.clearLogs()).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
    refreshLogs()
  })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" :placement="isMobile ? 'top' : 'left'" animated>
    <n-tab-pane name="setting" :tab="$gettext('Settings')">
      <n-flex vertical :size="16">
        <n-alert v-if="!supported" type="error">
          {{ $gettext('Tamper protection is only supported on Linux.') }}
        </n-alert>

        <n-card :title="$gettext('Running Status')" size="small">
          <n-descriptions :column="isMobile ? 1 : 3" label-placement="left" bordered size="small">
            <n-descriptions-item :label="$gettext('Status')">
              <n-tag :type="stats.running ? 'success' : 'default'" size="small">
                {{ stats.running ? $gettext('Protecting') : $gettext('Stopped') }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Protected Files')">
              {{ stats.protected_files }}
            </n-descriptions-item>
            <n-descriptions-item :label="$gettext('Protected Directories')">
              {{ stats.protected_dirs }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card :title="$gettext('Configuration')" size="small">
          <n-form v-bind="formLayout">
            <n-form-item :label="$gettext('Enable Protection')">
              <n-switch v-model:value="setting.enabled" />
            </n-form-item>
            <n-form-item :label="$gettext('Protection Mode')">
              <n-radio-group v-model:value="setting.mode">
                <n-radio-button value="chattr">{{ $gettext('File Lock (chattr)') }}</n-radio-button>
                <n-radio-button value="ebpf">eBPF-LSM</n-radio-button>
              </n-radio-group>
            </n-form-item>
            <n-form-item :label="$gettext('Mode Description')">
              <span v-if="setting.mode === 'chattr'" class="desc">
                {{
                  $gettext(
                    'Locks files with the immutable attribute, enforced by the kernel at the VFS layer. Works on all distributions with no kernel dependency. Blocks tampering by web processes, but not root.',
                  )
                }}
              </span>
              <span v-else class="desc">
                {{
                  $gettext(
                    'Intercepts write/delete/rename via eBPF-LSM with per-process tracing. More precise and auditable, but requires the bpf LSM to be active in the kernel.',
                  )
                }}
              </span>
            </n-form-item>
            <n-form-item :label="$gettext('Block New Files')">
              <n-flex vertical :size="4" align="start">
                <n-switch v-model:value="setting.block_new_files" />
                <span class="desc">
                  {{
                    $gettext(
                      'Reject newly created files of protected types in protected directories (denied by kernel in eBPF mode, deleted in chattr mode). When off, new files are logged and protected instead.',
                    )
                  }}
                </span>
              </n-flex>
            </n-form-item>
            <n-form-item :label="$gettext('Log Retention (days)')">
              <n-input-number
                v-model:value="setting.log_days"
                :min="1"
                :max="365"
                class="w-full md:!w-40"
              />
            </n-form-item>
            <n-form-item>
              <n-button
                type="primary"
                :loading="settingLoading"
                :disabled="settingLoading"
                @click="handleSaveSetting"
              >
                {{ $gettext('Save Changes') }}
              </n-button>
            </n-form-item>
          </n-form>
        </n-card>

        <!-- eBPF 环境检测与激活引导 -->
        <n-card :title="$gettext('eBPF Environment')" size="small">
          <n-flex vertical :size="10">
            <n-descriptions :column="isMobile ? 1 : 2" label-placement="left" bordered size="small">
              <n-descriptions-item :label="$gettext('Kernel Version')">
                {{ ebpf.kernel_version || '-' }}
              </n-descriptions-item>
              <n-descriptions-item :label="$gettext('Active LSM')">
                {{ ebpf.active_lsm || '-' }}
              </n-descriptions-item>
              <n-descriptions-item :label="$gettext('eBPF Available')" :span="2">
                <n-tag :type="ebpf.available ? 'success' : 'warning'" size="small">
                  {{ ebpf.available ? $gettext('Available') : $gettext('Unavailable') }}
                </n-tag>
              </n-descriptions-item>
            </n-descriptions>
            <n-alert v-if="!ebpf.available && ebpf.reason" type="warning" :bordered="false">
              {{ ebpf.reason }}
            </n-alert>
            <n-flex v-if="!ebpf.available && !ebpf.bpf_lsm_active && ebpf.kernel_version">
              <n-button
                type="warning"
                :loading="activating"
                :disabled="activating"
                @click="handleActivateEBPF"
              >
                {{ $gettext('Activate eBPF and Reboot') }}
              </n-button>
            </n-flex>
          </n-flex>
        </n-card>
      </n-flex>
    </n-tab-pane>

    <n-tab-pane name="rule" :tab="$gettext('Protection Rules')">
      <n-flex vertical>
        <n-flex>
          <n-button type="primary" @click="handleAddRule">
            <template #icon>
              <i-mdi-plus />
            </template>
            {{ $gettext('Add Rule') }}
          </n-button>
        </n-flex>
        <n-data-table
          v-model:page="rulePage"
          v-model:pageSize="rulePageSize"
          remote
          striped
          :scroll-x="800"
          :loading="rulesLoading"
          :columns="ruleColumns"
          :data="rules"
          :row-key="(row: any) => row.id"
          :pagination="rulePagination"
        />
      </n-flex>
    </n-tab-pane>

    <n-tab-pane name="log" :tab="$gettext('Interception Logs')">
      <n-flex vertical>
        <n-flex>
          <n-button type="error" secondary @click="handleClearLogs">
            <template #icon>
              <i-mdi-delete-outline />
            </template>
            {{ $gettext('Clear Logs') }}
          </n-button>
          <n-button secondary @click="() => refreshLogs()">
            <template #icon>
              <i-mdi-refresh />
            </template>
            {{ $gettext('Refresh') }}
          </n-button>
        </n-flex>
        <n-data-table
          v-model:page="logPage"
          v-model:pageSize="logPageSize"
          remote
          striped
          :scroll-x="800"
          :loading="logsLoading"
          :columns="logColumns"
          :data="logs"
          :row-key="(row: any) => row.id"
          :pagination="logPagination"
        />
      </n-flex>
    </n-tab-pane>
  </n-tabs>

  <rule-modal v-model:show="ruleModalShow" :rule="editingRule" @saved="refreshRules" />
</template>

<style scoped lang="scss">
.desc {
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.6;
}
</style>
