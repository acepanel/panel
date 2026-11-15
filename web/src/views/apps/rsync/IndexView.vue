<script setup lang="ts">
defineOptions({
  name: 'apps-rsync-index',
})

import { NButton, NFlex, NSwitch } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import rsync from '@/api/apps/rsync'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { generateRandomString } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const currentTab = ref('status')
const modelAddLoading = ref(false)

const addModuleModal = ref(false)
const addModuleModel = ref({
  name: '',
  path: '/opt/ace',
  comment: '',
  auth_user: '',
  secret: generateRandomString(16),
  hosts_allow: '0.0.0.0/0',
  read_only: false,
})

const editModuleModal = ref(false)
const editModuleModel = ref({
  name: '',
  path: '',
  comment: '',
  auth_user: '',
  secret: '',
  hosts_allow: '',
  read_only: false,
})

const processColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Directory'),
    key: 'path',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('User'),
    key: 'auth_user',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Host'),
    key: 'hosts_allow',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Read Only'),
    key: 'read_only',
    width: 100,
    render(row: any) {
      return h(NSwitch, { size: 'small', rubberBand: false, value: row.read_only, disabled: true })
    },
  },
  { title: $gettext('Comment'), key: 'comment', resizable: true, ellipsis: { tooltip: true } },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleModelEdit(row),
          },
          { default: () => $gettext('Configure') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete module %{ name }?', {
                  name: row.name,
                }),
              })
              if (ok) handleModelDelete(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  rsync.modules(page, pageSize),
)

const handleModelAdd = async () => {
  modelAddLoading.value = true
  useRequest(rsync.addModule(addModuleModel.value))
    .onSuccess(() => {
      refresh()
      addModuleModal.value = false
      addModuleModel.value = {
        name: '',
        path: '/opt/ace',
        comment: '',
        auth_user: '',
        secret: generateRandomString(16),
        hosts_allow: '0.0.0.0/0',
        read_only: false,
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
  editModuleModel.value.read_only = row.read_only
}

const handleSaveModuleConfig = async () => {
  useRequest(rsync.updateModule(editModuleModel.value.name, editModuleModel.value)).onSuccess(
    () => {
      editModuleModal.value = false
      refresh()
      window.$message.success($gettext('Saved successfully'))
    },
  )
}
</script>

<template>
  <PageContainer :show-footer="true">
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
            v-model:page="page"
            v-model:pageSize="pageSize"
            striped
            remote
            :scroll-x="1200"
            :loading="loading"
            :columns="processColumns"
            :data="data"
            :row-key="(row: any) => row.name"
            :pagination="pagination"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="rsync.config"
          :save="rsync.saveConfig"
          :tip="
            $gettext(
              'This modifies the Rsync main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
          @saved="refresh"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="rsyncd" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
  <app-modal v-model:show="addModuleModal" :title="$gettext('Add Module')">
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
      <n-form-item path="read_only" :label="$gettext('Read Only')">
        <n-switch v-model:value="addModuleModel.read_only" />
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
    <n-button
      type="info"
      block
      :loading="modelAddLoading"
      :disabled="modelAddLoading"
      @click="handleModelAdd"
    >
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
  <app-modal v-model:show="editModuleModal" :title="$gettext('Module Configuration')">
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
      <n-form-item path="read_only" :label="$gettext('Read Only')">
        <n-switch v-model:value="editModuleModel.read_only" />
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
    <n-button type="info" block @click="handleSaveModuleConfig">{{ $gettext('Save') }}</n-button>
  </app-modal>
</template>
