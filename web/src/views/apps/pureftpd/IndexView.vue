<script setup lang="ts">
defineOptions({
  name: 'apps-pureftpd-index'
})

import { NButton, NDataTable, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import pureftpd from '@/api/apps/pureftpd'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import PureftpdConfigTuneView from './PureftpdConfigTuneView.vue'
import { generateRandomString } from '@/utils'

const { $gettext } = useGettext()
const currentTab = ref('status')
const port = ref(0)
const addUserModal = ref(false)
const changePasswordModal = ref(false)

const addUserModel = ref({
  username: '',
  password: generateRandomString(16),
  path: ''
})

const changePasswordModel = ref({
  username: '',
  password: generateRandomString(16)
})

const userColumns: any = [
  {
    title: $gettext('Username'),
    key: 'username',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Path'),
    key: 'path',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 240,
    hideInExcel: true,
    render(row: any) {
      return [
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
            }
          },
          {
            default: () => $gettext('Change Password')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDeleteUser(row.username)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete user %{ username }?', {
                username: row.username
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
  (page, pageSize) => pureftpd.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const getPort = async () => {
  port.value = await pureftpd.port()
}

const handleSavePort = async () => {
  useRequest(pureftpd.updatePort(port.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleAddUser = async () => {
  useRequest(
    pureftpd.add(addUserModel.value.username, addUserModel.value.password, addUserModel.value.path)
  ).onSuccess(() => {
    refresh()
    addUserModal.value = false
    addUserModel.value.username = ''
    addUserModel.value.password = generateRandomString(16)
    addUserModel.value.path = ''
    window.$message.success($gettext('Added successfully'))
  })
}

const handleChangePassword = async () => {
  useRequest(
    pureftpd.changePassword(changePasswordModel.value.username, changePasswordModel.value.password)
  ).onSuccess(() => {
    refresh()
    changePasswordModal.value = false
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleDeleteUser = async (username: string) => {
  useRequest(pureftpd.delete(username)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  refresh()
  getPort()
})
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="pure-ftpd" />
          <n-card :title="$gettext('Port Settings')">
            <n-flex>
              <n-input-number v-model:value="port" :min="1" :max="65535" />
              <n-button type="primary" @click="handleSavePort">
                {{ $gettext('Save') }}
              </n-button>
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
            striped
            remote
            :scroll-x="1000"
            :loading="loading"
            :columns="userColumns"
            :data="data"
            :row-key="(row: any) => row.username"
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
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <pureftpd-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Run Log')">
        <realtime-log service="pure-ftpd" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
  <n-modal v-model:show="addUserModal" :title="$gettext('Create User')">
    <n-card
      closable
      @close="() => (addUserModal = false)"
      :title="$gettext('Create User')"
      style="width: 60vw"
    >
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
      <n-button type="info" block @click="handleAddUser">{{ $gettext('Submit') }}</n-button>
    </n-card>
  </n-modal>
  <n-modal v-model:show="changePasswordModal">
    <n-card
      closable
      @close="() => (changePasswordModal = false)"
      :title="$gettext('Change Password')"
      style="width: 60vw"
    >
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
      <n-button type="info" block @click="handleChangePassword">{{ $gettext('Submit') }}</n-button>
    </n-card>
  </n-modal>
</template>
