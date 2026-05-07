<script setup lang="ts">
defineOptions({
  name: 'apps-clickhouse-index',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import clickhouse from '@/api/apps/clickhouse'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

import ClickHouseConfigTuneView from './ClickHouseConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const setDefaultPasswordLoading = ref(false)
const saveConfigLoading = ref(false)

const { data: defaultPassword } = useRequest(clickhouse.defaultPassword, {
  initialData: '',
})
const { data: config, send: refreshConfig } = useRequest(clickhouse.config, {
  initialData: '',
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  }
})

const { data: load } = useRequest(clickhouse.load, {
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
  useRequest(clickhouse.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleSetDefaultPassword = () => {
  setDefaultPasswordLoading.value = true
  useRequest(clickhouse.setDefaultPassword(defaultPassword.value))
    .onSuccess(() => {
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => {
      setDefaultPasswordLoading.value = false
    })
}

const handleCopyDefaultPassword = () => {
  copy2clipboard(defaultPassword.value).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="clickhouse-server" />
          <n-card :title="$gettext('Default User Password')">
            <n-flex vertical>
              <n-alert type="info">
                {{
                  $gettext(
                    'The ClickHouse default user password is used to access the database. Keep it safe!',
                  )
                }}
              </n-alert>
              <n-flex>
                <n-input-group>
                  <n-input
                    v-model:value="defaultPassword"
                    type="password"
                    show-password-on="click"
                  />
                  <n-button type="primary" ghost @click="handleCopyDefaultPassword">
                    {{ $gettext('Copy') }}
                  </n-button>
                </n-input-group>
                <n-button
                  type="primary"
                  :loading="setDefaultPasswordLoading"
                  :disabled="setDefaultPasswordLoading"
                  @click="handleSetDefaultPassword"
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
                'This modifies the ClickHouse configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
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
        <click-house-config-tune-view />
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
        <realtime-log service="clickhouse-server" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
