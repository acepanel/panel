<script setup lang="ts">
defineOptions({
  name: 'apps-pureftpd-index',
})

import { NButton, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import pureftpd from '@/api/apps/pureftpd'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { generateRandomString } from '@/utils'

import PureftpdConfigTuneView from './PureftpdConfigTuneView.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const currentTab = ref('status')
const port = ref(0)
const addUserModal = ref(false)
const changePasswordModal = ref(false)

const addUserModel = ref({
  username: '',
  password: generateRandomString(16),
  path: '',
})

const changePasswordModel = ref({
  username: '',
  password: generateRandomString(16),
})

const userColumns: any = [
  {
    title: $gettext('Username'),
    key: 'username',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Path'),
    key: 'path',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 240,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            onClick: () => {
              changePasswordModel.value.username = row.username
              changePasswordModel.value.password = generateRandomString(16)
              changePasswordModal.value = true
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
                content: $gettext('Are you sure you want to delete user %{ username }?', {
                  username: row.username,
                }),
              })
              if (ok) await handleDeleteUser(row.username)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  pureftpd.list(page, pageSize),
)

const getPort = async () => {
  port.value = await pureftpd.port()
}

const handleSavePort = async () => {
  await pureftpd.updatePort(port.value)
  window.$message.success($gettext('Saved successfully'))
}

const handleAddUser = async () => {
  await pureftpd.add(
    addUserModel.value.username,
    addUserModel.value.password,
    addUserModel.value.path,
  )
  refresh()
  addUserModal.value = false
  addUserModel.value.username = ''
  addUserModel.value.password = generateRandomString(16)
  addUserModel.value.path = ''
  window.$message.success($gettext('Added successfully'))
}

const handleChangePassword = async () => {
  await pureftpd.changePassword(
    changePasswordModel.value.username,
    changePasswordModel.value.password,
  )
  refresh()
  changePasswordModal.value = false
  window.$message.success($gettext('Modified successfully'))
}

const handleDeleteUser = async (username: string) => {
  await pureftpd.delete(username)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

onMounted(() => {
  getPort()
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="pure-ftpd" />
          <n-card :title="$gettext('Port Settings')">
            <n-flex>
              <n-input-number v-model:value="port" :min="1" :max="65535" />
              <app-button type="primary" @click="handleSavePort">
                {{ $gettext('Save') }}
              </app-button>
            </n-flex>
            {{ $gettext('Modify Pure-Ftpd listening port') }}
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="users" :tab="$gettext('User Management')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="addUserModal = true">
              {{ $gettext('Add User') }}
            </n-button>
          </n-flex>
          <n-data-table
            v-model:page="page"
            v-model:pageSize="pageSize"
            striped
            remote
            :scroll-x="1000"
            :loading="loading"
            :columns="userColumns"
            :data="data"
            :row-key="(row: any) => row.username"
            :pagination="pagination"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <pureftpd-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Run Log')">
        <realtime-log service="pure-ftpd" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
  <app-modal v-model:show="addUserModal" :title="$gettext('Create User')">
    <n-form :model="addUserModel">
      <n-form-item path="username" :label="$gettext('Username')">
        <n-input
          v-model:value="addUserModel.username"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter username')"
        />
      </n-form-item>
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="addUserModel.password"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="
            $gettext('It is recommended to use the generator to generate a random password')
          "
        />
      </n-form-item>
      <n-form-item path="path" :label="$gettext('Directory')">
        <n-input
          v-model:value="addUserModel.path"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter the directory authorized to the user')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleAddUser">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <app-modal v-model:show="changePasswordModal" :title="$gettext('Change Password')">
    <n-form :model="changePasswordModel">
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="changePasswordModel.password"
          type="text"
          @keydown.enter.prevent
          :placeholder="
            $gettext('It is recommended to use the generator to generate a random password')
          "
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleChangePassword">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
