<script setup lang="ts">
defineOptions({
  name: 'apps-postgresql-index',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import file from '@/api/panel/file'
import systemctl from '@/api/panel/systemctl'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

import PostgresqlConfigTuneView from './PostgresqlConfigTuneView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const setPostgresPasswordLoading = ref(false)
const saveConfigLoading = ref(false)
const saveUserConfigLoading = ref(false)
const clearLogLoading = ref(false)
const clearAppLogLoading = ref(false)
const runLogRef = ref<{ clear: () => void } | null>(null)
const appLogRef = ref<{ clear: () => void } | null>(null)

const { data: postgresPassword } = useRequest(postgresql.postgresPassword, {
  initialData: '',
})
const selectedLog = ref('')
const { data: logList } = useRequest(postgresql.log, {
  initialData: [],
}).onSuccess(({ data }) => {
  if (!data?.length) {
    selectedLog.value = ''
    return
  }
  // log_filename='postgresql-%a.log' 按 weekday 缩写循环；优先选中今天对应的文件
  const weekdays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
  const today = `postgresql-${weekdays[new Date().getDay()]}.log`
  selectedLog.value = data.find((p: string) => p.endsWith(today)) ?? data[data.length - 1]
})

const logOptions = computed(() =>
  (logList.value as string[]).map((p) => ({
    label: p.split('/').pop() ?? p,
    value: p,
  })),
)
const { data: config, send: refreshConfig } = useRequest(postgresql.config, {
  initialData: '',
})
const { data: userConfig, send: refreshUserConfig } = useRequest(postgresql.userConfig, {
  initialData: '',
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  } else if (val === 'user-config') {
    refreshUserConfig()
  }
})
const { data: load } = useRequest(postgresql.load, {
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
  useRequest(postgresql.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleSaveUserConfig = () => {
  saveUserConfigLoading.value = true
  useRequest(postgresql.saveUserConfig(userConfig.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveUserConfigLoading.value = false
    })
}

const handleClearLog = () => {
  clearLogLoading.value = true
  useRequest(systemctl.clearLog('postgresql'))
    .onSuccess(() => {
      runLogRef.value?.clear()
      window.$message.success($gettext('Cleared successfully'))
    })
    .onComplete(() => {
      clearLogLoading.value = false
    })
}

const handleClearAppLog = () => {
  if (!selectedLog.value) return
  clearAppLogLoading.value = true
  useRequest(file.truncate(selectedLog.value))
    .onSuccess(() => {
      appLogRef.value?.clear()
      window.$message.success($gettext('Cleared successfully'))
    })
    .onComplete(() => {
      clearAppLogLoading.value = false
    })
}

const handleSetPostgresPassword = () => {
  setPostgresPasswordLoading.value = true
  useRequest(postgresql.setPostgresPassword(postgresPassword.value))
    .onSuccess(() => {
      window.$message.success($gettext('Modified successfully'))
    })
    .onComplete(() => {
      setPostgresPasswordLoading.value = false
    })
}

const handleCopyPostgresPassword = () => {
  copy2clipboard(postgresPassword.value).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="postgresql" show-reload />
          <n-card :title="$gettext('Super Password')">
            <n-flex vertical>
              <n-alert type="info">
                {{
                  $gettext(
                    'The "postgres" superuser password is used to manage the database system. Keep it safe!',
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
                <n-button
                  type="primary"
                  :loading="setPostgresPasswordLoading"
                  :disabled="setPostgresPasswordLoading"
                  @click="handleSetPostgresPassword"
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
                'This modifies the PostgreSQL main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
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
        <postgresql-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="user-config" :tab="$gettext('User Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the PostgreSQL user configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              )
            }}
          </n-alert>
          <common-editor v-model:value="userConfig" height="60vh" />
          <n-flex>
            <n-button
              type="primary"
              :loading="saveUserConfigLoading"
              :disabled="saveUserConfigLoading"
              @click="handleSaveUserConfig"
            >
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
            <n-button
              type="primary"
              :loading="clearLogLoading"
              :disabled="clearLogLoading"
              @click="handleClearLog"
            >
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log ref="runLogRef" service="postgresql" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="app-log" :tab="$gettext('Application Logs')">
        <n-flex vertical>
          <n-flex>
            <n-select v-model:value="selectedLog" :options="logOptions" class="w-60" />
            <n-button
              type="primary"
              :loading="clearAppLogLoading"
              :disabled="clearAppLogLoading || !selectedLog"
              @click="handleClearAppLog"
            >
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log ref="appLogRef" :path="selectedLog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
