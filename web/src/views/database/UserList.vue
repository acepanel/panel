<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NFlex, NInput, NInputGroup, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import DeleteConfirm from '@/components/common/DeleteConfirm.vue'
import { formatDateTime } from '@/utils'
import UpdateUserModal from '@/views/database/UpdateUserModal.vue'

const { $gettext } = useGettext()
const updateModal = ref(false)
const updateID = ref(0)

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
            switch (row.server.type) {
              case 'mysql':
                return 'MySQL'
              case 'postgresql':
                return 'PostgreSQL'
              default:
                return row.server.type
            }
          }
        }
      )
    }
  },
  {
    title: $gettext('Username'),
    key: 'username',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.username || $gettext('None')
    }
  },
  {
    title: $gettext('Password'),
    key: 'password',
    width: 250,
    render(row: any) {
      return h(NInputGroup, null, {
        default: () => [
          h(NInput, {
            value: row.password,
            type: 'password',
            showPasswordOn: 'click',
            readonly: true,
            placeholder: $gettext('Not saved')
          }),
          h(
            NButton,
            {
              type: 'primary',
              ghost: true,
              onClick: () => {
                copy2clipboard(row.password).then(() => {
                  window.$message.success($gettext('Copied successfully'))
                })
              }
            },
            { default: () => $gettext('Copy') }
          )
        ]
      })
    }
  },
  {
    title: $gettext('Host'),
    key: 'host',
    width: 150,
    render(row: any) {
      return h(NTag, null, {
        default: () => row.host || $gettext('None')
      })
    }
  },
  {
    title: $gettext('Server'),
    key: 'server',
    width: 150,
    render(row: any) {
      return row.server.name
    }
  },
  {
    title: $gettext('Privileges'),
    key: 'privileges',
    width: 200,
    render(row: any) {
      return h(NFlex, null, {
        default: () =>
          row.privileges.map((privilege: string) =>
            h(NTag, null, {
              default: () => privilege
            })
          )
      })
    }
  },
  {
    title: $gettext('Comment'),
    key: 'remark',
    minWidth: 250,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        class: 'w-full',
        value: row.remark,
        onBlur: () => handleRemark(row),
        onUpdateValue(v) {
          row.remark = v
        }
      })
    }
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        { type: row.status === 'valid' ? 'success' : 'error' },
        { default: () => (row.status === 'valid' ? $gettext('Valid') : $gettext('Invalid')) }
      )
    }
  },
  {
    title: $gettext('Update Date'),
    key: 'updated_at',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.updated_at)
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
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              updateID.value = row.id
              updateModal.value = true
            }
          },
          {
            default: () => $gettext('Modify')
          }
        ),
        h(
          DeleteConfirm,
          {
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete the user?')
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
  (page, pageSize) => database.userList(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleDelete = (id: number) => {
  useRequest(database.userDelete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleRemark = (row: any) => {
  useRequest(database.userRemark(row.id, row.remark)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

onMounted(() => {
  window.$bus.on('database-user:refresh', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('database-user:refresh')
})
</script>

<template>
  <n-data-table
    striped
    remote
    :scroll-x="1800"
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
  <update-user-modal v-model:show="updateModal" v-model:id="updateID" />
</template>

<style scoped lang="scss"></style>
