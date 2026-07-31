<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'
import { formatDateTime } from '@/utils'
import ExclusionModal from '@/views/waf/ExclusionModal.vue'

const { $gettext } = useGettext()

// 过滤条件
const filterClientIP = ref('')
const filterJA4 = ref('')
const filterAction = ref<string | null>(null)
const filterPolicyId = ref<number | null>(null)

const exclusionShow = ref(false)
const exclusionEvent = ref<any>(null)

const actionFilterOptions = computed(() => [
  { label: $gettext('All'), value: '' },
  { label: $gettext('Block'), value: 'block' },
  { label: $gettext('Challenge'), value: 'challenge' },
  { label: $gettext('Log'), value: 'log' },
  { label: $gettext('Ban'), value: 'ban' },
])

const actionTag = (action: string) => {
  switch (action) {
    case 'block':
    case 'ban':
      return 'error'
    case 'challenge':
      return 'warning'
    default:
      return 'default'
  }
}

const columns: any = [
  {
    title: $gettext('Time'),
    key: 'ts',
    width: 170,
    render: (row: any) => (row.ts ? formatDateTime(new Date(row.ts * 1000)) : '-'),
  },
  {
    title: $gettext('Client IP'),
    key: 'client_ip',
    width: 140,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('JA4'),
    key: 'ja4',
    width: 280,
    ellipsis: { tooltip: true },
    render: (row: any) => row.ja4 || '-',
  },
  {
    title: $gettext('Action'),
    key: 'action',
    width: 100,
    render: (row: any) =>
      h(NTag, { type: actionTag(row.action), size: 'small' }, { default: () => row.action || '-' }),
  },
  {
    title: $gettext('Severity'),
    key: 'severity',
    width: 90,
  },
  {
    title: $gettext('Host'),
    key: 'host',
    minWidth: 140,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Method'),
    key: 'method',
    width: 90,
  },
  {
    title: 'URI',
    key: 'uri',
    minWidth: 180,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Rule'),
    key: 'rule',
    minWidth: 140,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Country'),
    key: 'country',
    width: 90,
    render: (row: any) => row.country || '-',
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 130,
    fixed: 'right',
    render(row: any) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'primary',
          secondary: true,
          onClick: () => {
            exclusionEvent.value = row
            exclusionShow.value = true
          },
        },
        { default: () => $gettext('Add to Allowlist') }
      )
    },
  },
]

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) =>
    waf.events({
      page,
      limit: pageSize,
      client_ip: filterClientIP.value || undefined,
      ja4: filterJA4.value || undefined,
      action: filterAction.value || undefined,
      policy_id: filterPolicyId.value || undefined,
    }),
  {
    initialData: { total: 0, items: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  }
)

const handleSearch = () => {
  page.value = 1
  refresh()
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex align="center" :wrap="true">
      <n-input
        v-model:value="filterClientIP"
        :placeholder="$gettext('Client IP')"
        clearable
        class="w-48"
        @keydown.enter="handleSearch"
      />
      <n-input
        v-model:value="filterJA4"
        :placeholder="$gettext('JA4')"
        clearable
        class="w-72"
        @keydown.enter="handleSearch"
      />
      <n-select
        v-model:value="filterAction"
        :options="actionFilterOptions"
        :placeholder="$gettext('Action')"
        clearable
        class="w-40"
      />
      <n-input-number
        v-model:value="filterPolicyId"
        :placeholder="$gettext('Policy ID')"
        :min="1"
        class="w-40"
      />
      <n-button type="primary" @click="handleSearch">{{ $gettext('Search') }}</n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1680"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
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
  <exclusion-modal v-model:show="exclusionShow" :event="exclusionEvent" />
</template>

<style scoped lang="scss"></style>
