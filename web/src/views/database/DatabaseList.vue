<script setup lang="ts">
import { NInput, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

const props = defineProps<{
  type: string
  keyword: string
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
        const busy = saving.has(`${row.server_id}:${row.name}`)
        return h(NInput, {
          size: 'small',
          class: 'w-full',
          value: row.comment,
          disabled: busy,
          loading: busy || undefined,
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
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              content: $gettext('Are you sure you want to delete this database?'),
              countdown: 5,
            })
            if (ok) await handleDelete(row.server_id, row.name)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  })

  return cols
})

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable(
  (page, pageSize) => database.list(page, pageSize, props.type, props.keyword),
  {
    watchingStates: [() => props.keyword],
    debounce: [300],
  },
)

const handleDelete = async (serverID: number, name: string) => {
  await database.delete(serverID, name)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const saving = reactive(new Set<string>())

const handleComment = async (row: any) => {
  const key = `${row.server_id}:${row.name}`
  if (saving.has(key)) return
  saving.add(key)
  try {
    await database.comment(row.server_id, row.name, row.comment)
    window.$message.success($gettext('Modified successfully'))
  } finally {
    saving.delete(key)
  }
}

useBus('database:refresh', refresh)
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
    :pagination="pagination"
  />
</template>

<style scoped lang="scss"></style>
