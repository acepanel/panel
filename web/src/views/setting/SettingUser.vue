<script setup lang="ts">
import user from '@/api/panel/user'
import { formatDateTime } from '@/utils'
import PasskeyModal from '@/views/setting/PasskeyModal.vue'
import PasswordModal from '@/views/setting/PasswordModal.vue'
import TokenModal from '@/views/setting/TokenModal.vue'
import TwoFaModal from '@/views/setting/TwoFaModal.vue'
import { NButton, NDataTable, NInput, NPopconfirm, NSwitch } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const currentID = ref(0)
const passwordModal = ref(false)
const twoFaModal = ref(false)
const tokenModal = ref(false)
const passkeyModal = ref(false)

const columns: any = [
  {
    title: $gettext('Username'),
    key: 'username',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(NInput, {
        size: 'small',
        value: row.username,
        onBlur: () => handleUsername(row),
        onUpdateValue(v) {
          row.username = v
        }
      })
    }
  },
  {
    title: $gettext('Email'),
    key: 'email',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(NInput, {
        size: 'small',
        value: row.email,
        onBlur: () => handleEmail(row),
        onUpdateValue(v) {
          row.email = v
        }
      })
    }
  },
  {
    title: $gettext('2FA'),
    key: 'two_fa',
    width: 150,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.two_fa !== '',
        onUpdateValue: (v) => {
          if (v) {
            twoFaModal.value = true
            currentID.value = row.id
          } else {
            useRequest(user.updateTwoFA(row.id, '', '')).onSuccess(() => {
              window.$message.success($gettext('Disabled successfully'))
              refresh()
            })
          }
        }
      })
    }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 500,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              currentID.value = row.id
              tokenModal.value = true
            }
          },
          {
            default: () => $gettext('Access Tokens')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 15px;',
            onClick: () => {
              currentID.value = row.id
              passkeyModal.value = true
            }
          },
          {
            default: () => $gettext('Passkeys')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 15px;',
            onClick: () => {
              currentID.value = row.id
              passwordModal.value = true
            }
          },
          {
            default: () => $gettext('Change Password')
          }
        ),
        h(
          NPopconfirm,
          {
            style: 'margin-left: 15px;',
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete this user?')
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
  (page, pageSize) => user.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleUsername = (row: any) => {
  useRequest(user.updateUsername(row.id, row.username)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleEmail = (row: any) => {
  useRequest(user.updateEmail(row.id, row.email)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleDelete = (id: number) => {
  useRequest(user.delete(id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refresh()
  })
}

onMounted(() => {
  window.$bus.on('user:refresh', refresh)
})
</script>

<template>
  <n-flex vertical>
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
  </n-flex>
  <password-modal v-model:id="currentID" v-model:show="passwordModal" />
  <two-fa-modal v-model:id="currentID" v-model:show="twoFaModal" />
  <token-modal v-model:id="currentID" v-model:show="tokenModal" />
  <passkey-modal v-model:id="currentID" v-model:show="passkeyModal" />
</template>

<style scoped lang="scss"></style>
