<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatDateTime } from '@/utils'
import DecisionModal from '@/views/waf/DecisionModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const modalShow = ref(false)

type TagType = 'default' | 'error' | 'info' | 'success' | 'warning' | 'primary'
const typeTag = (type: string): { type: TagType; label: string } => {
  switch (type) {
    case 'ban':
      return { type: 'error', label: $gettext('Ban') }
    case 'allow':
      return { type: 'success', label: $gettext('Allow') }
    case 'captcha':
      return { type: 'warning', label: $gettext('Captcha') }
    default:
      return { type: 'default', label: type }
  }
}

const columns: any = [
  { title: 'ID', key: 'id', width: 80 },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 110,
    render(row: any) {
      const t = typeTag(row.type)
      return h(NTag, { type: t.type }, { default: () => t.label })
    },
  },
  {
    title: $gettext('Scope'),
    key: 'scope',
    width: 100,
    render: (row: any) =>
      String(row.value).includes('/') ? $gettext('Range') : $gettext('Single IP'),
  },
  {
    title: $gettext('Value'),
    key: 'value',
    minWidth: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Origin'),
    key: 'origin',
    width: 110,
    render: (row: any) => h(NTag, { size: 'small' }, { default: () => row.origin || '-' }),
  },
  {
    title: $gettext('Expires At'),
    key: 'until',
    width: 180,
    render: (row: any) =>
      row.until && row.until > 0 ? formatDateTime(new Date(row.until * 1000)) : $gettext('Never'),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 110,
    align: 'center',
    render(row: any) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              content: $gettext('Are you sure you want to delete this entry?'),
            })
            if (ok) handleDelete(row.id)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  },
]

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => waf.decisions(page, pageSize),
  {
    initialData: { total: 0, items: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

watch(modalShow, (v) => {
  if (!v) refresh()
})

const handleDelete = (id: number) => {
  useRequest(waf.deleteDecision(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex items-center>
      <n-button type="primary" @click="modalShow = true">
        {{ $gettext('Add Entry') }}
      </n-button>
      <n-button @click="refresh">{{ $gettext('Refresh') }}</n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="900"
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
  <decision-modal v-model:show="modalShow" />
</template>

<style scoped lang="scss"></style>
