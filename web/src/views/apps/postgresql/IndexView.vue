<script setup lang="ts">
defineOptions({
  name: 'apps-postgresql-index'
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import PostgresqlConfigTuneView from './PostgresqlConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')

const { data: postgresPassword } = useRequest(postgresql.postgresPassword, {
  initialData: ''
})
const { data: log } = useRequest(postgresql.log, {
  initialData: ''
})
const { data: config, send: refreshConfig } = useRequest(postgresql.config, {
  initialData: ''
})
const { data: userConfig, send: refreshUserConfig } = useRequest(postgresql.userConfig, {
  initialData: ''
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  } else if (val === 'user-config') {
    refreshUserConfig()
  }
})
const { data: load } = useRequest(postgresql.load, {
  initialData: []
})

const loadColumns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    minWidth: 200,
    ellipsis: { tooltip: true }
  }
]

const handleSaveConfig = async () => {
  await postgresql.saveConfig(config.value)
  window.$message.success($gettext('Saved successfully'))
}

const handleSaveUserConfig = async () => {
  await postgresql.saveUserConfig(userConfig.value)
  window.$message.success($gettext('Saved successfully'))
}

const handleClearLog = async () => {
  await postgresql.clearLog()
  window.$message.success($gettext('Cleared successfully'))
}

const handleSetPostgresPassword = () => {
  useRequest(postgresql.setPostgresPassword(postgresPassword.value)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleCopyPostgresPassword = () => {
  copy2clipboard(postgresPassword.value).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="postgresql" show-reload />
          <n-card :title="$gettext('Super Password')">
            <n-flex vertical>
              <n-alert type="info">
                {{
                  $gettext(
                    'The "postgres" superuser password is used to manage the database system. Keep it safe!'
                  )
                }}
              </n-alert>
              <n-flex>
                <n-input-group>
                  <n-input
                    v-model:value="postgresPassword"
                    type="password"
                    show-password-on="click"
                  />
                  <n-button type="primary" ghost @click="handleCopyPostgresPassword">
                    {{ $gettext('Copy') }}
                  </n-button>
                </n-input-group>
                <n-button type="primary" @click="handleSetPostgresPassword">
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
                'This modifies the PostgreSQL main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <postgresql-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="user-config" :tab="$gettext('User Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the PostgreSQL user configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="userConfig" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveUserConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
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
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="handleClearLog">
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log service="postgresql" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="slow-log" :tab="$gettext('Slow Logs')">
        <realtime-log :path="log" />
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>
