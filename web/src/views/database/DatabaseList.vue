<script setup lang="ts">
import { NButton, NInput, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import DeleteConfirm from '@/components/common/DeleteConfirm.vue'

const { $gettext } = useGettext()

const columns: any = [
  {
    title: $gettext('Type'),
    key: 'type',
    width: 150,
    render(row: any) {
      return h(
        NTag,
        { type: 'info' },
        {
          default: () => {
            switch (row.type) {
              case 'mysql':
                return 'MySQL'
              case 'postgresql':
                return 'PostgreSQL'
              default:
                return row.type
            }
          }
        }
      )
    }
  },
  {
    title: $gettext('Database Name'),
    key: 'name',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Server'),
    key: 'server',
    width: 150
  },
  {
    title: $gettext('Encoding'),
    key: 'encoding',
    width: 150,
    render(row: any) {
      return h(NTag, null, {
        default: () => row.encoding
      })
    }
  },
  {
    title: $gettext('Comment'),
    key: 'comment',
    minWidth: 250,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        class: 'w-full',
        value: row.comment,
        // MySQL 不支持数据库备注
        disabled: row.type === 'mysql',
        placeholder:
          row.type === 'mysql' ? $gettext('MySQL does not support database comments') : undefined,
        onBlur: () => handleComment(row),
        onUpdateValue(v) {
          row.comment = v
        }
      })
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          DeleteConfirm,
          {
            onPositiveClick: () => handleDelete(row.server_id, row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete this database?')
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px;'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => database.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
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
      pageCount: pageCount,
      pageSize: pageSize,
      itemCount: total,
      showQuickJumper: true,
      showSizePicker: true,
      pageSizes: [20, 50, 100, 200]
    }"
  />
</template>

<style scoped lang="scss"></style>
