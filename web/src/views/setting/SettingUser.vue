<script setup lang="ts">
import { NButton, NDataTable, NFlex, NInput, NSwitch } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'
import PasskeyModal from '@/views/setting/PasskeyModal.vue'
import PasswordModal from '@/views/setting/PasswordModal.vue'
import TokenModal from '@/views/setting/TokenModal.vue'
import TwoFaModal from '@/views/setting/TwoFaModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const currentID = ref(0)
const passwordModal = ref(false)
const twoFaModal = ref(false)
const tokenModal = ref(false)
const passkeyModal = ref(false)
const switching = reactive(new Set<number>())
const saving = reactive(new Set<string>())

const columns: any = [
  {
    title: $gettext('Username'),
    key: 'username',
    minWidth: 100,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        value: row.username,
        disabled: saving.has(`${row.id}:username`),
        loading: saving.has(`${row.id}:username`) || undefined,
        onBlur: () => handleUsername(row),
        onUpdateValue(v) {
          row.username = v
        },
      })
    },
  },
  {
    title: $gettext('Email'),
    key: 'email',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        value: row.email,
        disabled: saving.has(`${row.id}:email`),
        loading: saving.has(`${row.id}:email`) || undefined,
        onBlur: () => handleEmail(row),
        onUpdateValue(v) {
          row.email = v
        },
      })
    },
  },
  {
    title: $gettext('2FA'),
    key: 'two_fa',
    width: 150,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.two_fa,
        loading: switching.has(row.id),
        onUpdateValue: (v) => {
          if (v) {
            twoFaModal.value = true
            currentID.value = row.id
          } else {
            switching.add(row.id)
            useRequest(user.updateTwoFA(row.id, '', ''))
              .onSuccess(() => {
                window.$message.success($gettext('Disabled successfully'))
                refresh()
              })
              .onComplete(() => switching.delete(row.id))
          }
        },
      })
    },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 500,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              currentID.value = row.id
              tokenModal.value = true
            },
          },
          { default: () => $gettext('Access Tokens') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              currentID.value = row.id
              passkeyModal.value = true
            },
          },
          { default: () => $gettext('Passkeys') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              currentID.value = row.id
              passwordModal.value = true
            },
          },
          { default: () => $gettext('Change Password') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this user?'),
              })
              if (ok) await handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  user.list(page, pageSize),
)

const handleUsername = async (row: any) => {
  const key = `${row.id}:username`
  if (saving.has(key)) return
  saving.add(key)
  try {
    await user.updateUsername(row.id, row.username)
    window.$message.success($gettext('Modified successfully'))
  } finally {
    saving.delete(key)
  }
}

const handleEmail = async (row: any) => {
  const key = `${row.id}:email`
  if (saving.has(key)) return
  saving.add(key)
  try {
    await user.updateEmail(row.id, row.email)
    window.$message.success($gettext('Modified successfully'))
  } finally {
    saving.delete(key)
  }
}

const handleDelete = async (id: number) => {
  await user.delete(id)
  window.$message.success($gettext('Deleted successfully'))
  refresh()
}

useBus('user:refresh', refresh)
</script>

<template>
  <n-flex vertical>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1200"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <password-modal v-model:id="currentID" v-model:show="passwordModal" />
  <two-fa-modal v-model:id="currentID" v-model:show="twoFaModal" />
  <token-modal v-model:id="currentID" v-model:show="tokenModal" />
  <passkey-modal v-model:id="currentID" v-model:show="passkeyModal" />
</template>

<style scoped lang="scss"></style>
