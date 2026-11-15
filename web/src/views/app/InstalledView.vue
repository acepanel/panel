<script setup lang="ts">
import { NDataTable, NFlex } from 'naive-ui'

import app from '@/api/panel/app'
import environment from '@/api/panel/environment'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

import { useAppColumns } from './composables/useAppColumns'
import { useEnvColumns } from './composables/useEnvColumns'

const appColumns = useAppColumns({ showStatus: true })
const envColumns = useEnvColumns()

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
</script>

<template>
  <n-flex vertical :size="24">
    <!-- 已安装应用 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Native App') }}</n-h3>
      <n-data-table
        v-model:page="appPage"
        v-model:pageSize="appPageSize"
        striped
        remote
        :scroll-x="1500"
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
        striped
        :scroll-x="1080"
        :loading="envLoading"
        :columns="envColumns"
        :data="envData"
        :row-key="(row: any) => row.slug"
      />
    </n-flex>
  </n-flex>
</template>
