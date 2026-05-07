<script setup lang="ts">
import { NButton, NInput, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import { useConfirm } from '@/components/system/composables/useConfirm'

const props = defineProps<{
  type: string
}>()

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const hasEncoding = computed(() => ['mysql', 'postgresql'].includes(props.type))
const hasComment = computed(() => ['postgresql'].includes(props.type))

const columns: any = computed(() => {
  const cols: any[] = [
    {
      title: $gettext('Database Name'),
      key: 'name',
      minWidth: 100,
      resizable: true,
      ellipsis: { tooltip: true },
    },
    {
      title: $gettext('Server'),
      key: 'server',
      width: 150,
    },
  ]

  if (hasEncoding.value) {
    cols.push({
      title: $gettext('Encoding'),
      key: 'encoding',
      width: 150,
      render(row: any) {
        return h(NTag, null, {
          default: () => row.encoding,
        })
      },
    })
  }

  if (hasComment.value) {
    cols.push({
      title: $gettext('Comment'),
      key: 'comment',
      minWidth: 250,
      resizable: true,
      render(row: any) {
        return h(NInput, {
          size: 'small',
          class: 'w-full',
          value: row.comment,
          onBlur: () => handleComment(row),
          onUpdateValue(v: string) {
            row.comment = v
          },
        })
      },
    })
  }

  cols.push({
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
              content: $gettext('Are you sure you want to delete this database?'),
            })
            if (ok) handleDelete(row.server_id, row.name)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  })

  return cols
})

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => database.list(page, pageSize, props.type),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

const handleDelete = (serverID: number, name: string) => {
  useRequest(database.delete(serverID, name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleComment = (row: any) => {
  useRequest(database.comment(row.server_id, row.name, row.comment)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

onMounted(() => {
  window.$bus.on('database:refresh', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('database:refresh')
})
</script>

<template>
  <n-data-table
    striped
    remote
    :scroll-x="1000"
    :loading="loading"
    :columns="columns"
    :data="data"
    :row-key="(row: any) => row.name"
    v-model:page="page"
    v-model:pageSize="pageSize"
    :pagination="{
      page: page,
      pageSize: pageSize,
      itemCount: total,
      showQuickJumper: true,
      showSizePicker: true,
      pageSizes: [20, 50, 100, 200],
    }"
  />
</template>

<style scoped lang="scss"></style>
