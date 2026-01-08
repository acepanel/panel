<script setup lang="ts">
import { NButton, NDataTable, NInput, NPopconfirm, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import webhook from '@/api/panel/webhook'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()

// 创建弹窗
const createModal = ref(false)
const createModel = ref({
  name: '',
  script: '#!/bin/bash\n\n',
  raw: false,
  user: 'root'
})

// 编辑弹窗
const editModal = ref(false)
const editModel = ref({
  id: 0,
  name: '',
  script: '',
  raw: false,
  user: '',
  status: true
})

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: 'Key',
    key: 'key',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false
        },
        {
          default: () => row.key
        }
      )
    }
  },
  {
    title: $gettext('Run As User'),
    key: 'user',
    width: 120,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.user || 'root'
    }
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
          size: 'small'
        },
        {
          default: () => (row.raw ? $gettext('Yes') : $gettext('No'))
        }
      )
    }
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
        onUpdateValue: () => handleStatusChange(row)
      })
    }
  },
  {
    title: $gettext('Call Count'),
    key: 'call_count',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true }
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
    }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 180,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): string {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 280,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => handleCopyUrl(row)
          },
          {
            default: () => $gettext('Copy URL')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 10px;',
            onClick: () => handleEdit(row)
          },
          {
            default: () => $gettext('Edit')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete this WebHook?')
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 10px;'
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
  (page, pageSize) => webhook.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleStatusChange = (row: any) => {
  useRequest(
    webhook.update(row.id, {
      name: row.name,
      script: row.script,
      raw: row.raw,
      user: row.user,
      status: !row.status
    })
  ).onSuccess(() => {
    row.status = !row.status
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleCopyUrl = (row: any) => {
  const url = `${window.location.origin}/webhook/${row.key}`
  navigator.clipboard.writeText(url).then(() => {
    window.$message.success($gettext('URL copied to clipboard'))
  })
}

const handleEdit = (row: any) => {
  editModel.value = {
    id: row.id,
    name: row.name,
    script: row.script,
    raw: row.raw,
    user: row.user || 'root',
    status: row.status
  }
  editModal.value = true
}

const handleDelete = (id: number) => {
  useRequest(webhook.delete(id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refresh()
  })
}

const handleCreate = () => {
  if (!createModel.value.name) {
    window.$message.warning($gettext('Please enter a name'))
    return
  }
  if (!createModel.value.script) {
    window.$message.warning($gettext('Please enter a script'))
    return
  }
  useRequest(webhook.create(createModel.value)).onSuccess(() => {
    createModal.value = false
    createModel.value = {
      name: '',
      script: '#!/bin/bash\n\n',
      raw: false,
      user: 'root'
    }
    window.$message.success($gettext('Created successfully'))
    refresh()
  })
}

const handleUpdate = () => {
  if (!editModel.value.name) {
    window.$message.warning($gettext('Please enter a name'))
    return
  }
  if (!editModel.value.script) {
    window.$message.warning($gettext('Please enter a script'))
    return
  }
  useRequest(
    webhook.update(editModel.value.id, {
      name: editModel.value.name,
      script: editModel.value.script,
      raw: editModel.value.raw,
      user: editModel.value.user,
      status: editModel.value.status
    })
  ).onSuccess(() => {
    editModal.value = false
    window.$message.success($gettext('Modified successfully'))
    refresh()
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex justify="end">
      <n-button type="primary" @click="createModal = true">
        {{ $gettext('Create WebHook') }}
      </n-button>
    </n-flex>
    <n-data-table
      striped
      remote
      :scroll-x="1400"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
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

  <!-- 创建弹窗 -->
  <n-modal
    v-model:show="createModal"
    preset="card"
    :title="$gettext('Create WebHook')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form :model="createModel">
      <n-form-item :label="$gettext('Name')">
        <n-input v-model:value="createModel.name" :placeholder="$gettext('Enter WebHook name')" />
      </n-form-item>
      <n-form-item :label="$gettext('User')">
        <n-input
          v-model:value="createModel.user"
          :placeholder="$gettext('User to run the script (default: root)')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Raw Output')">
        <n-switch v-model:value="createModel.raw" />
        <span   text-gray ml-10 >
          {{ $gettext('Return script output as raw text instead of JSON') }}
        </span>
      </n-form-item>
      <n-form-item :label="$gettext('Script')">
        <common-editor v-model:value="createModel.script" lang="sh" height="40vh" />
      </n-form-item>
    </n-form>
    <n-button type="info" @click="handleCreate" block>
      {{ $gettext('Create') }}
    </n-button>
  </n-modal>

  <!-- 编辑弹窗 -->
  <n-modal
    v-model:show="editModal"
    preset="card"
    :title="$gettext('Edit WebHook')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form :model="editModel">
      <n-form-item :label="$gettext('Name')">
        <n-input v-model:value="editModel.name" :placeholder="$gettext('Enter WebHook name')" />
      </n-form-item>
      <n-form-item :label="$gettext('User')">
        <n-input
          v-model:value="editModel.user"
          :placeholder="$gettext('User to run the script (default: root)')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Raw Output')">
        <n-switch v-model:value="editModel.raw" />
        <span   text-gray ml-10 >
          {{ $gettext('Return script output as raw text instead of JSON') }}
        </span>
      </n-form-item>
      <n-form-item :label="$gettext('Enabled')">
        <n-switch v-model:value="editModel.status" />
      </n-form-item>
      <n-form-item :label="$gettext('Script')">
        <common-editor v-model:value="editModel.script" lang="sh" height="40vh" />
      </n-form-item>
    </n-form>
    <n-button type="info" @click="handleUpdate" block>
      {{ $gettext('Save') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
