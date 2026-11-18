<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable, NFlex, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import webhook from '@/api/panel/webhook'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const defaultModel = () => ({
  name: '',
  script: '#!/bin/bash\n\n',
  raw: false,
  user: 'root',
  status: true,
})

// 创建与编辑共用一个弹窗，editId 为 0 表示创建
const formModal = ref(false)
const editId = ref(0)
const model = ref(defaultModel())

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: 'Key',
    key: 'key',
    width: 300,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false,
        },
        {
          default: () => row.key,
        },
      )
    },
  },
  {
    title: $gettext('Run As User'),
    key: 'user',
    width: 120,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.user || 'root'
    },
  },
  {
    title: $gettext('Raw Output'),
    key: 'raw',
    width: 120,
    resizable: true,
    render(row: any) {
      return h(
        NTag,
        {
          type: row.raw ? 'success' : 'default',
          size: 'small',
        },
        {
          default: () => (row.raw ? $gettext('Yes') : $gettext('No')),
        },
      )
    },
  },
  {
    title: $gettext('Enabled'),
    key: 'status',
    width: 100,
    resizable: true,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.status,
        loading: switching.has(row.id),
        onUpdateValue: () => handleStatusChange(row),
      })
    },
  },
  {
    title: $gettext('Call Count'),
    key: 'call_count',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Last Call'),
    key: 'last_call_at',
    width: 180,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): string {
      if (!row.last_call_at || row.last_call_at === '0001-01-01T00:00:00Z') {
        return '-'
      }
      return formatDateTime(row.last_call_at)
    },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 180,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 280,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => handleCopyUrl(row),
          },
          { default: () => $gettext('Copy URL') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => handleEdit(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this WebHook?'),
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
  webhook.list(page, pageSize),
)

const switching = reactive(new Set<number>())

const handleStatusChange = (row: any) => {
  switching.add(row.id)
  useRequest(
    webhook.update(row.id, {
      name: row.name,
      script: row.script,
      raw: row.raw,
      user: row.user,
      status: !row.status,
    }),
  )
    .onSuccess(() => {
      row.status = !row.status
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => switching.delete(row.id))
}

const handleCopyUrl = (row: any) => {
  const url = `${window.location.origin}/webhook/${row.key}`
  copy2clipboard(url).then(() => {
    window.$message.success($gettext('URL copied to clipboard'))
  })
}

const handleCreate = () => {
  editId.value = 0
  model.value = defaultModel()
  formModal.value = true
}

const handleEdit = (row: any) => {
  editId.value = row.id
  model.value = {
    name: row.name,
    script: row.script,
    raw: row.raw,
    user: row.user || 'root',
    status: row.status,
  }
  formModal.value = true
}

const handleDelete = async (id: number) => {
  await webhook.delete(id)
  window.$message.success($gettext('Deleted successfully'))
  refresh()
}

const submit = async () => {
  const { name, script, raw, user, status } = model.value
  const req = editId.value
    ? webhook.update(editId.value, { name, script, raw, user, status })
    : webhook.create({ name, script, raw, user })
  await req
  formModal.value = false
  window.$message.success(
    editId.value ? $gettext('Modified successfully') : $gettext('Created successfully'),
  )
  refresh()
}

// 校验不通过时同步返回，避免提交按钮闪一下加载图标
const handleSubmit = () => {
  if (!model.value.name) {
    window.$message.warning($gettext('Please enter a name'))
    return
  }
  if (!model.value.script) {
    window.$message.warning($gettext('Please enter a script'))
    return
  }
  return submit()
}
</script>

<template>
  <n-flex vertical>
    <n-flex>
      <n-button type="primary" @click="handleCreate">
        {{ $gettext('Create WebHook') }}
      </n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1600"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>

  <app-modal
    v-model:show="formModal"
    :title="editId ? $gettext('Edit WebHook') : $gettext('Create WebHook')"
  >
    <n-form :model="model">
      <n-form-item :label="$gettext('Name')">
        <n-input v-model:value="model.name" :placeholder="$gettext('Enter WebHook name')" />
      </n-form-item>
      <n-form-item :label="$gettext('User')">
        <n-input
          v-model:value="model.user"
          :placeholder="$gettext('User to run the script (default: root)')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Raw Output')">
        <n-switch v-model:value="model.raw" />
        <span class="text-text-tertiary ml-2">
          {{ $gettext('Return script output as raw text instead of JSON') }}
        </span>
      </n-form-item>
      <n-form-item v-if="editId" :label="$gettext('Enabled')">
        <n-switch v-model:value="model.status" />
      </n-form-item>
      <n-form-item :label="$gettext('Script')">
        <common-editor v-model:value="model.script" lang="shell" height="40vh" />
      </n-form-item>
    </n-form>
    <app-button type="info" @click="handleSubmit" block>
      {{ editId ? $gettext('Save') : $gettext('Create') }}
    </app-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
