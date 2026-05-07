<script setup lang="ts">
defineOptions({
  name: 'apps-mongodb-index',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import mongodb from '@/api/apps/mongodb'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

import MongoDBConfigTuneView from './MongoDBConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const setAdminPasswordLoading = ref(false)
const saveConfigLoading = ref(false)

const { data: adminPassword } = useRequest(mongodb.adminPassword, {
  initialData: '',
})
const { data: config, send: refreshConfig } = useRequest(mongodb.config, {
  initialData: '',
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
})

const { data: load } = useRequest(mongodb.load, {
  initialData: [],
})

const loadColumns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
]

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(mongodb.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleSetAdminPassword = () => {
  setAdminPasswordLoading.value = true
  useRequest(mongodb.setAdminPassword(adminPassword.value))
    .onSuccess(() => {
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => {
      setAdminPasswordLoading.value = false
    })
}

const handleCopyAdminPassword = () => {
  copy2clipboard(adminPassword.value).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="mongod" />
          <n-card :title="$gettext('Admin Password')">
            <n-flex vertical>
              <n-alert type="info">
                {{
                  $gettext(
                    'The MongoDB admin password is used to manage the database. Keep it safe!',
                  )
                }}
              </n-alert>
              <n-flex>
                <n-input-group>
                  <n-input v-model:value="adminPassword" type="password" show-password-on="click" />
                  <n-button type="primary" ghost @click="handleCopyAdminPassword">
                    {{ $gettext('Copy') }}
                  </n-button>
                </n-input-group>
                <n-button
                  type="primary"
                  :loading="setAdminPasswordLoading"
                  :disabled="setAdminPasswordLoading"
                  @click="handleSetAdminPassword"
                >
                  {{ $gettext('Save') }}
                </n-button>
              </n-flex>
            </n-flex>
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the MongoDB configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button
              type="primary"
              :loading="saveConfigLoading"
              :disabled="saveConfigLoading"
              @click="handleSaveConfig"
            >
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <mongo-d-b-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <n-data-table
          striped
          remote
          :scroll-x="400"
          :loading="false"
          :columns="loadColumns"
          :data="load"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="mongod" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
