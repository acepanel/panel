<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import { formatBytes } from '@/utils/file'

import { useStatQuery } from './useStatQuery'

const { $gettext } = useGettext()

const { loading, items } = useStatQuery(({ start, end, sites }) =>
  website.statSites(start, end, sites),
)

const columns = computed(() => [
  { title: $gettext('Site'), key: 'site', ellipsis: { tooltip: true } },
  { title: 'PV', key: 'pv', sorter: (a: any, b: any) => a.pv - b.pv },
  { title: 'UV', key: 'uv', sorter: (a: any, b: any) => a.uv - b.uv },
  { title: 'IP', key: 'ip', sorter: (a: any, b: any) => a.ip - b.ip },
  {
    title: $gettext('Outbound'),
    key: 'bandwidth',
    render: (row: any) => formatBytes(row.bandwidth),
    sorter: (a: any, b: any) => a.bandwidth - b.bandwidth,
  },
  {
    title: $gettext('Inbound'),
    key: 'bandwidth_in',
    render: (row: any) => formatBytes(row.bandwidth_in),
    sorter: (a: any, b: any) => a.bandwidth_in - b.bandwidth_in,
  },
  {
    title: $gettext('Requests'),
    key: 'requests',
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
  { title: $gettext('Errors'), key: 'errors', sorter: (a: any, b: any) => a.errors - b.errors },
  {
    title: $gettext('Spiders'),
    key: 'spiders',
    sorter: (a: any, b: any) => a.spiders - b.spiders,
  },
])
</script>

<template>
  <n-spin :show="loading">
    <n-data-table :columns="columns" :data="items" :bordered="false" size="small" :scroll-x="900" />
  </n-spin>
</template>
