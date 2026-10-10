<script setup lang="ts">
import { NButton, NDataTable, NPopover, NRadioButton, NRadioGroup, NSpin, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import { useConfirm } from '@/components/system/composables/useConfirm'
import CreateModal from '@/views/firewall/CreateModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const createModalShow = ref(false)
const strategyUpdating = ref<Record<string, boolean>>({})

const strategyOptions = [
  { label: $gettext('Accept'), value: 'accept' },
  { label: $gettext('Drop'), value: 'drop' },
  { label: $gettext('Reject'), value: 'reject' },
]

const handleStrategyUpdate = (row: any, strategy: string) => {
  if (row.strategy === strategy) return

  const key = JSON.stringify(row)
  if (strategyUpdating.value[key]) return
  strategyUpdating.value[key] = true
  useRequest(firewall.updateRuleStrategy(row, strategy))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => {
      delete strategyUpdating.value[key]
    })
}

// 端口进程信息缓存
const portUsageCache = ref<Record<string, any>>({})
const portUsageLoading = ref<Record<string, boolean>>({})

const fetchPortUsage = async (port: number, protocol: string) => {
  const key = `${protocol}:${port}`
  if (portUsageCache.value[key]) return
  portUsageLoading.value[key] = true
  try {
    const proto = protocol === 'tcp/udp' ? 'tcp' : protocol
    const data = await firewall.portUsage(port, proto)
    portUsageCache.value[key] = data || []
  } finally {
    portUsageLoading.value[key] = false
  }
}

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Transport Protocol'),
    key: 'protocol',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          if (row.protocol !== '') {
            return row.protocol
          }
          return $gettext('None')
        },
      })
    },
  },
  {
    title: $gettext('Network Protocol'),
    key: 'family',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          if (row.family !== '') {
            return row.family
          }
          return $gettext('None')
        },
      })
    },
  },
  {
    title: $gettext('Port'),
    key: 'port',
    width: 250,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      if (row.port_start == row.port_end) {
        return row.port_start
      }
      return `${row.port_start}-${row.port_end}`
    },
  },
  {
    title: $gettext('Status'),
    key: 'in_use',
    width: 150,
    render(row: any): any {
      if (!row.in_use) {
        return h(NTag, { type: 'default' }, { default: () => $gettext('Not Used') })
      }

      const key = `${row.protocol}:${row.port_start}`
      return h(
        NPopover,
        {
          trigger: 'click',
          placement: 'bottom',
          onUpdateShow: (show: boolean) => {
            if (show) fetchPortUsage(row.port_start, row.protocol)
          },
        },
        {
          trigger: () =>
            h(
              NTag,
              {
                type: 'success',
                style: 'cursor: pointer;',
              },
              { default: () => $gettext('In Use') },
            ),
          default: () => {
            if (portUsageLoading.value[key]) {
              return h(NSpin, { size: 'small', style: 'padding: 12px;' })
            }
            const processes = portUsageCache.value[key]
            if (!processes || processes.length === 0) {
              return h(
                'div',
                { style: 'padding: 4px; color: var(--n-text-color);' },
                $gettext('No process information'),
              )
            }
            return h(
              'div',
              { style: 'max-height: 300px; overflow-y: auto;' },
              processes.map((p: any, i: number) => {
                return h(
                  'div',
                  {
                    style:
                      i > 0
                        ? 'padding-top: 8px; margin-top: 8px; border-top: 1px solid var(--n-border-color);'
                        : '',
                  },
                  [
                    h('div', { style: 'font-size: 13px;' }, [
                      h('span', { style: 'font-weight: bold;' }, `${p.name}`),
                      h('span', { style: 'margin-left: 8px; opacity: 0.6;' }, `PID: ${p.pid}`),
                    ]),
                    h(
                      'div',
                      {
                        style:
                          'font-size: 12px; opacity: 0.8; margin-top: 4px; word-break: break-all; font-family: monospace;',
                      },
                      p.command,
                    ),
                  ],
                )
              }),
            )
          },
        },
      )
    },
  },
  {
    title: $gettext('Strategy'),
    key: 'strategy',
    width: 280,
    render(row: any): any {
      if (strategyOptions.some((option) => option.value === row.strategy)) {
        const key = JSON.stringify(row)
        return h(
          NRadioGroup,
          {
            value: row.strategy,
            name: `strategy-${row.family}-${row.protocol}-${row.port_start}-${row.port_end}-${row.address}-${row.direction}`,
            size: 'small',
            disabled: strategyUpdating.value[key],
            onUpdateValue: (strategy: string) => handleStrategyUpdate(row, strategy),
          },
          {
            default: () =>
              strategyOptions.map((option) =>
                h(NRadioButton, {
                  key: option.value,
                  label: option.label,
                  value: option.value,
                }),
              ),
          },
        )
      }

      return h(
        NTag,
        { type: 'default' },
        {
          default: () => {
            switch (row.strategy) {
              case 'mark':
                return $gettext('Mark')
              default:
                return $gettext('Unknown')
            }
          },
        },
      )
    },
  },
  {
    title: $gettext('Direction'),
    key: 'direction',
    width: 150,
    render(row: any): any {
      return h(
        NTag,
        {
          type: row.direction === 'in' ? 'info' : 'default',
        },
        {
          default: () => {
            switch (row.direction) {
              case 'in':
                return $gettext('Inbound')
              case 'out':
                return $gettext('Outbound')
              default:
                return $gettext('Unknown')
            }
          },
        },
      )
    },
  },
  {
    title: $gettext('Target'),
    key: 'address',
    minWidth: 200,
    render(row: any): any {
      return h(NTag, null, {
        default: () => {
          if (row.address === '') {
            return $gettext('All')
          }
          return row.address
        },
      })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              content: $gettext('Are you sure you want to delete?'),
            })
            if (ok) handleDelete(row)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  },
]

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => firewall.rules(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

const selectedRowKeys = ref<any>([])
const importing = ref(false)
const selectingUnused = ref(false)

// 勾选所有未使用的端口规则，配合批量删除清理
// 后端 limit 校验上限 10000，规则总数超限时按 total 循环拉取所有分页
const selectUnused = async () => {
  selectingUnused.value = true
  try {
    const unused: any[] = []
    let total = Infinity
    for (let page = 1; (page - 1) * 10000 < total; page++) {
      await useRequest(firewall.rules(page, 10000)).onSuccess(({ data }: any) => {
        total = data.total
        unused.push(...data.items.filter((row: any) => !row.in_use))
      })
    }
    selectedRowKeys.value = unused.map((row) => JSON.stringify(row))
    if (selectedRowKeys.value.length === 0) {
      window.$message.info($gettext('No unused rules'))
    }
  } finally {
    selectingUnused.value = false
  }
}

// 从 xlsx 导入规则
const handleImport = ({ file }: any) => {
  const formData = new FormData()
  formData.append('file', file.file)
  importing.value = true
  useRequest(firewall.importRules(formData))
    .onSuccess(({ data }: any) => {
      window.$message.success(
        $gettext('Import finished: %{succeeded} succeeded, %{failed} failed', {
          succeeded: data.succeeded,
          failed: data.failed,
        }),
      )
      refresh()
    })
    .onComplete(() => {
      importing.value = false
    })
}

const handleDelete = async (row: any) => {
  useRequest(firewall.deleteRule(row)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const batchDelete = async () => {
  if (selectedRowKeys.value.length === 0) {
    window.$message.info($gettext('Please select rules to delete'))
    return
  }

  const promises = selectedRowKeys.value.map((key: any) => {
    const rule = JSON.parse(key)
    return firewall.deleteRule(rule)
  })
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex items-center>
      <n-button type="primary" @click="createModalShow = true">
        {{ $gettext('Create Rule') }}
      </n-button>
      <n-button :loading="selectingUnused" :disabled="selectingUnused" ghost @click="selectUnused">
        {{ $gettext('Select Unused') }}
      </n-button>
      <ConfirmDialog
        type="danger"
        :content="$gettext('Are you sure you want to delete the selected rules?')"
        @confirm="batchDelete"
      >
        <template #trigger>
          <n-button type="error" ghost>
            {{ $gettext('Delete') }}
          </n-button>
        </template>
      </ConfirmDialog>
      <n-button tag="a" :href="firewall.ruleExportUrl" target="_blank" ghost>
        {{ $gettext('Export') }}
      </n-button>
      <!-- n-upload 默认占满整行,收窄避免按钮组换行 -->
      <n-upload
        accept=".xlsx"
        :show-file-list="false"
        :custom-request="handleImport"
        style="width: auto"
      >
        <n-button :loading="importing" :disabled="importing" ghost>
          {{ $gettext('Import') }}
        </n-button>
      </n-upload>
    </n-flex>
    <n-data-table
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1650"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => JSON.stringify(row)"
      :pagination="{
        page: page,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200],
      }"
    />
  </n-flex>
  <create-modal v-model:show="createModalShow" @created="refresh" />
</template>

<style scoped lang="scss"></style>
