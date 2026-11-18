<script setup lang="ts">
import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import alert from '@/api/panel/alert'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'
import AlertRuleModal from '@/views/monitor/AlertRuleModal.vue'
import { useAlertMetrics } from '@/views/monitor/metrics'

const { $gettext } = useGettext()
const { confirmAction, confirmDelete } = useConfirm()
const { metricOf, conditionText } = useAlertMetrics()

const currentTab = ref('rule')

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
} = usePagedTable((page, pageSize) => alert.rules(page, pageSize))

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
  await alert.deleteRule(row.id)
  window.$message.success($gettext('Deleted successfully'))
  refreshRules()
}

const ruleColumns: any = [
  { title: $gettext('Name'), key: 'name', width: 160, ellipsis: { tooltip: true } },
  {
    title: $gettext('Metric'),
    key: 'type',
    minWidth: 180,
    render(row: any) {
      const label = metricOf(row.type).label
      return row.target ? `${label} (${row.target})` : label
    },
  },
  {
    title: $gettext('Condition'),
    key: 'threshold',
    minWidth: 180,
    render: (row: any) => conditionText(row),
  },
  {
    title: $gettext('Trigger After'),
    key: 'duration',
    width: 110,
    render: (row: any) => `${row.duration} ${$gettext('times')}`,
  },
  {
    title: $gettext('Silence Period'),
    key: 'silence',
    width: 110,
    render: (row: any) => `${row.silence} ${$gettext('minutes')}`,
  },
  {
    title: $gettext('Notify Channels'),
    key: 'channels',
    width: 110,
    render(row: any) {
      const count = row.channels?.length || 0
      return h(NTag, { size: 'small', type: count > 0 ? 'info' : 'default' }, () =>
        count > 0 ? `${count}` : $gettext('Record only'),
      )
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
      return h(NFlex, { justify: 'center', size: 8 }, () => [
        h(NButton, { size: 'small', secondary: true, onClick: () => handleEditRule(row) }, () =>
          $gettext('Edit'),
        ),
        h(
          AppButton,
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

// 记录
const {
  loading: recordsLoading,
  data: records,
  page: recordPage,
  pageSize: recordPageSize,
  refresh: refreshRecords,
  pagination: recordPagination,
} = usePagedTable((page, pageSize) => alert.records(page, pageSize))

const recordColumns: any = [
  {
    title: $gettext('Time'),
    key: 'created_at',
    width: 180,
    render: (row: any) => formatDateTime(row.created_at),
  },
  { title: $gettext('Rule'), key: 'rule_name', width: 160, ellipsis: { tooltip: true } },
  {
    title: $gettext('Metric'),
    key: 'type',
    width: 180,
    render(row: any) {
      const label = metricOf(row.type).label
      return row.target ? `${label} (${row.target})` : label
    },
  },
  { title: $gettext('Detail'), key: 'message', ellipsis: { tooltip: true } },
  {
    title: $gettext('Notified'),
    key: 'notified',
    width: 100,
    render(row: any) {
      return h(NTag, { size: 'small', type: row.notified ? 'success' : 'default' }, () =>
        row.notified ? $gettext('Sent') : $gettext('Not sent'),
      )
    },
  },
]

const handleClearRecords = async () => {
  const ok = await confirmAction({
    title: $gettext('Clear Records'),
    content: $gettext('Are you sure to clear all alert records?'),
  })
  if (!ok) return
  await alert.clearRecords()
  window.$message.success($gettext('Cleared successfully'))
  refreshRecords()
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" animated>
    <n-tab-pane name="rule" :tab="$gettext('Alert Rules')">
      <n-flex vertical :size="16">
        <n-flex justify="space-between" align="center">
          <n-alert type="info" :bordered="false" class="flex-1">
            {{
              $gettext(
                'Rules are checked every minute and notifications are sent through the selected channels.',
              )
            }}
          </n-alert>
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
          :scroll-x="1200"
          :loading="rulesLoading"
          :columns="ruleColumns"
          :data="rules"
          :row-key="(row: any) => row.id"
          :pagination="rulePagination"
        />
      </n-flex>
    </n-tab-pane>

    <n-tab-pane name="record" :tab="$gettext('Alert Records')">
      <n-flex vertical :size="16">
        <n-flex>
          <app-button type="error" secondary @click="handleClearRecords">
            <template #icon>
              <i-mdi-delete-outline />
            </template>
            {{ $gettext('Clear Records') }}
          </app-button>
          <n-button secondary @click="() => refreshRecords()">
            <template #icon>
              <i-mdi-refresh />
            </template>
            {{ $gettext('Refresh') }}
          </n-button>
        </n-flex>
        <n-data-table
          v-model:page="recordPage"
          v-model:pageSize="recordPageSize"
          remote
          striped
          :scroll-x="1000"
          :loading="recordsLoading"
          :columns="recordColumns"
          :data="records"
          :row-key="(row: any) => row.id"
          :pagination="recordPagination"
        />
      </n-flex>
    </n-tab-pane>
  </n-tabs>

  <alert-rule-modal v-model:show="ruleModalShow" :rule="editingRule" @saved="refreshRules" />
</template>
