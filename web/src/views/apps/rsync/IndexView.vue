<script setup lang="ts">
defineOptions({
  name: 'apps-rsync-index'
})

import { NButton, NDataTable, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import rsync from '@/api/apps/rsync'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import { generateRandomString } from '@/utils'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveConfigLoading = ref(false)
const modelAddLoading = ref(false)
const config = ref('')

const addModuleModal = ref(false)
const addModuleModel = ref({
  name: '',
  path: '/www',
  comment: '',
  auth_user: '',
  secret: generateRandomString(16),
  hosts_allow: '0.0.0.0/0'
})

const editModuleModal = ref(false)
const editModuleModel = ref({
  name: '',
  path: '',
  comment: '',
  auth_user: '',
  secret: '',
  hosts_allow: ''
})

const processColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Directory'),
    key: 'path',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('User'),
    key: 'auth_user',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Host'),
    key: 'hosts_allow',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  { title: $gettext('Comment'), key: 'comment', resizable: true, ellipsis: { tooltip: true } },
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
            type: 'info',
            onClick: () => handleModelEdit(row)
          },
          {
            default: () => $gettext('Configure')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleModelDelete(row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete module %{ name }?', {
                name: row.name
              })
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px'
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
  (page, pageSize) => rsync.modules(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const getConfig = async () => {
  config.value = await rsync.config()
}

const handleSaveConfig = async () => {
  saveConfigLoading.value = true
  useRequest(rsync.saveConfig(config.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleModelAdd = async () => {
  modelAddLoading.value = true
  useRequest(rsync.addModule(addModuleModel.value))
    .onSuccess(() => {
      refresh()
      getConfig()
      addModuleModal.value = false
      addModuleModel.value = {
        name: '',
        path: '/www',
        comment: '',
        auth_user: '',
        secret: generateRandomString(16),
        hosts_allow: '0.0.0.0/0'
      }
      window.$message.success($gettext('Added successfully'))
    })
    .onComplete(() => {
      modelAddLoading.value = false
    })
}

const handleModelDelete = async (name: string) => {
  useRequest(rsync.deleteModule(name)).onSuccess(() => {
    refresh()
    getConfig()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleModelEdit = async (row: any) => {
  editModuleModal.value = true
  editModuleModel.value.name = row.name
  editModuleModel.value.path = row.path
  editModuleModel.value.comment = row.comment
  editModuleModel.value.auth_user = row.auth_user
  editModuleModel.value.secret = row.secret
  editModuleModel.value.hosts_allow = row.hosts_allow
}

const handleSaveModuleConfig = async () => {
  useRequest(rsync.updateModule(editModuleModel.value.name, editModuleModel.value)).onSuccess(
    () => {
      refresh()
      getConfig()
      window.$message.success($gettext('Saved successfully'))
    }
  )
}

onMounted(() => {
  refresh()
  getConfig()
})
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="rsyncd" />
      </n-tab-pane>
      <n-tab-pane name="modules" :tab="$gettext('Module Management')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="addModuleModal = true">
              {{ $gettext('Add Module') }}
            </n-button>
          </n-flex>
          <n-data-table
            striped
            remote
            :scroll-x="1000"
            :loading="loading"
            :columns="processColumns"
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
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the Rsync main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" :loading="saveConfigLoading" :disabled="saveConfigLoading" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="rsyncd" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
  <n-modal
    v-model:show="addModuleModal"
    preset="card"
    :title="$gettext('Add Module')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="addModuleModal = false"
  >
    <n-form :model="addModuleModel">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="addModuleModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Name cannot contain Chinese characters')"
        />
      </n-form-item>
      <n-form-item path="path" :label="$gettext('Directory')">
        <n-input
          v-model:value="addModuleModel.path"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Please enter absolute path')"
        />
      </n-form-item>
      <n-form-item path="auth_user" :label="$gettext('User')">
        <n-input
          v-model:value="addModuleModel.auth_user"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter module username')"
        />
      </n-form-item>
      <n-form-item path="secret" :label="$gettext('Password')">
        <n-input
          v-model:value="addModuleModel.secret"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter module password')"
        />
      </n-form-item>
      <n-form-item path="hosts_allow" :label="$gettext('Host')">
        <n-input
          v-model:value="addModuleModel.hosts_allow"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter allowed hosts, separate multiple hosts with spaces')"
        />
      </n-form-item>
      <n-form-item path="comment" :label="$gettext('Comment')">
        <n-input
          v-model:value="addModuleModel.comment"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter comments')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="modelAddLoading" :disabled="modelAddLoading" @click="handleModelAdd">{{ $gettext('Submit') }}</n-button>
  </n-modal>
  <n-modal
    v-model:show="editModuleModal"
    preset="card"
    :title="$gettext('Module Configuration')"
    style="width: 80vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="handleSaveModuleConfig"
  >
    <n-form :model="editModuleModel">
      <n-form-item path="path" :label="$gettext('Directory')">
        <n-input
          v-model:value="editModuleModel.path"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Please enter absolute path')"
        />
      </n-form-item>
      <n-form-item path="auth_user" :label="$gettext('User')">
        <n-input
          v-model:value="editModuleModel.auth_user"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter module username')"
        />
      </n-form-item>
      <n-form-item path="secret" :label="$gettext('Password')">
        <n-input
          v-model:value="editModuleModel.secret"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter module password')"
        />
      </n-form-item>
      <n-form-item path="hosts_allow" :label="$gettext('Host')">
        <n-input
          v-model:value="editModuleModel.hosts_allow"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter allowed hosts, separate multiple hosts with spaces')"
        />
      </n-form-item>
      <n-form-item path="comment" :label="$gettext('Comment')">
        <n-input
          v-model:value="editModuleModel.comment"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter comments')"
        />
      </n-form-item>
    </n-form>
  </n-modal>
</template>
