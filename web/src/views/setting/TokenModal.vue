<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import { NAlert, NButton, NDataTable, NFlex, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

// 创建与编辑共用一个弹窗，editId 为 0 表示创建
const formModal = ref(false)
const editId = ref(0)
const defaultTokenModel = () => ({
  ips: [] as Array<string>,
  expired_at: new Date().getTime() + 31536000 * 1000, // 1 year
})
const model = ref(defaultTokenModel())

const handleOpenCreate = () => {
  editId.value = 0
  model.value = defaultTokenModel()
  formModal.value = true
}

const columns: any = [
  {
    title: $gettext('ID'),
    key: 'id',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true },
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
    title: $gettext('Expiration Time'),
    key: 'expired_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.expired_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 260,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              editId.value = row.id
              model.value = {
                ips: [...(row.ips || [])],
                expired_at: new Date(row.expired_at).getTime(),
              }
              formModal.value = true
            },
          },
          { default: () => $gettext('Modify') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this access token?'),
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

const { loading, data, page, pageSize, refresh, reload, pagination } = usePagedTable(
  (page, pageSize) => user.tokenList(id.value, page, pageSize),
  { immediate: false },
)

const handleDelete = async (id: number) => {
  await user.tokenDelete(id)
  window.$message.success($gettext('Deleted successfully'))
  refresh()
}

const handleCreate = async () => {
  const data = await user.tokenCreate(id.value, model.value.ips, model.value.expired_at)
  formModal.value = false
  window.$dialog.success({
    title: $gettext('Created successfully'),
    content: () => {
      return [
        h(
          NFlex,
          {
            vertical: true,
          },
          {
            default: () => [
              h(
                NAlert,
                {
                  type: 'warning',
                },
                {
                  default: () =>
                    $gettext(
                      'Token is only displayed once, please save it before closing the dialog.',
                    ),
                },
              ),
              h(NInput, {
                value: data.token,
                type: 'password',
                showPasswordOn: 'click',
                readonly: true,
              }),
            ],
          },
        ),
      ]
    },
    maskClosable: false,
    positiveText: $gettext('Copy and close'),
    onPositiveClick: () => {
      copy2clipboard(data.token)
        .then(() => {
          window.$message.success($gettext('Copied successfully'))
        })
        .catch(() => {
          window.$message.error($gettext('Copy failed'))
        })
    },
  })
  refresh()
}

const handleUpdate = async () => {
  await user.tokenUpdate(editId.value, model.value.ips, model.value.expired_at)
  window.$message.success($gettext('Updated successfully'))
  formModal.value = false
  refresh()
}

watch(
  () => show.value,
  (val) => {
    if (val) {
      reload()
    }
  },
  { immediate: true },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Access Tokens')">
    <n-flex vertical>
      <n-flex>
        <n-button type="primary" @click="handleOpenCreate">
          {{ $gettext('Create Access Token') }}
        </n-button>
      </n-flex>
      <n-data-table
        v-model:page="page"
        v-model:pageSize="pageSize"
        striped
        remote
        :scroll-x="800"
        :loading="loading"
        :columns="columns"
        :data="data"
        :row-key="(row: any) => row.id"
        :pagination="pagination"
      />
    </n-flex>
  </app-modal>
  <app-modal
    v-model:show="formModal"
    :title="editId ? $gettext('Modify Access Token') : $gettext('Create Access Token')"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('IP White List')">
          <ListInput
            v-model:value="model.ips"
            :placeholder="$gettext('127.0.0.1')"
            show-sort-button
          />
        </n-form-item>
        <n-form-item :label="$gettext('Expiration Time')">
          <n-date-picker
            v-model:value="model.expired_at"
            type="datetime"
            :placeholder="$gettext('Please select the expiration time')"
            w-full
          />
        </n-form-item>
      </n-form>
      <app-button type="primary" @click="editId ? handleUpdate() : handleCreate()">
        {{ editId ? $gettext('Update') : $gettext('Create') }}
      </app-button>
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
