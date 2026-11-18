<script setup lang="ts">
import { NDataTable, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import environment from '@/api/panel/environment'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

import { useAppColumns } from './composables/useAppColumns'
import { useEnvColumns } from './composables/useEnvColumns'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()

// 只有存在更新的行可勾选，表头全选即选中全部可更新项
const appColumns = [
  { type: 'selection', fixed: 'left', disabled: (row: any) => !row.update_exist },
  ...useAppColumns({ showStatus: true }),
]
const envColumns = [
  { type: 'selection', fixed: 'left', disabled: (row: any) => !row.has_update },
  ...useEnvColumns(),
]

// 不同类型的环境 slug 可能相同
const envRowKey = (row: any) => `${row.type}:${row.slug}`

const selectedApps = ref<string[]>([])
const selectedEnvs = ref<string[]>([])

// 获取已安装应用
const {
  loading: appLoading,
  data: appData,
  page: appPage,
  pageSize: appPageSize,
  pagination: appPagination,
} = usePagedTable((page, pageSize) => app.list(page, pageSize, undefined, undefined, true))

// 获取已安装环境
const { loading: envLoading, data: envData } = useRequest(
  () => environment.list(undefined, undefined, true),
  { initialData: [] },
)

const handleBulkUpdate = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Update'),
    content: $gettext(
      'Updating may reset related configurations to default state, are you sure to update the selected apps and environments?',
    ),
  })
  if (!ok) return

  const results = await Promise.allSettled([
    ...selectedApps.value.map((slug) => app.update(slug)),
    ...envData.value
      .filter((row: any) => selectedEnvs.value.includes(envRowKey(row)))
      .map((row: any) => environment.update(row.type, row.slug)),
  ])

  selectedApps.value = []
  selectedEnvs.value = []
  if (results.some((result) => result.status === 'fulfilled')) {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  }
}
</script>

<template>
  <n-flex vertical :size="24">
    <n-flex>
      <app-button
        type="warning"
        :disabled="selectedApps.length + selectedEnvs.length === 0"
        @click="handleBulkUpdate"
      >
        {{ $gettext('Update') }}
      </app-button>
    </n-flex>
    <!-- 已安装应用 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Native App') }}</n-h3>
      <n-data-table
        v-model:checked-row-keys="selectedApps"
        v-model:page="appPage"
        v-model:pageSize="appPageSize"
        striped
        remote
        :scroll-x="1540"
        :loading="appLoading"
        :columns="appColumns"
        :data="appData"
        :row-key="(row: any) => row.slug"
        :pagination="appPagination"
      />
    </n-flex>
    <!-- 已安装环境 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Operating Environment') }}</n-h3>
      <n-data-table
        v-model:checked-row-keys="selectedEnvs"
        striped
        :scroll-x="1120"
        :loading="envLoading"
        :columns="envColumns"
        :data="envData"
        :row-key="envRowKey"
      />
    </n-flex>
  </n-flex>
</template>
