<script setup lang="ts">
defineOptions({
  name: 'apps-supervisor-index',
})

import { NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import supervisor from '@/api/apps/supervisor'
import file from '@/api/panel/file'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()
const currentTab = ref('status')
const processLog = ref('')
const processLogModalRef = ref<{ clear: () => void } | null>(null)
const daemonLogPath = '/var/log/supervisor/supervisord.log'

const { data: serviceName } = useRequest(supervisor.service, {
  initialData: '',
})

const createProcessModal = ref(false)
const createProcessModel = ref({
  name: '',
  user: 'www',
  path: '',
  command: '',
  num: 1,
})

const editProcessModal = ref(false)
const editProcessTab = ref('setting')
const editProcessModel = ref({
  process: '',
  config: '',
})

const settingModel = ref<Record<string, string>>({
  command: '',
  directory: '',
  user: '',
  numprocs: '',
  priority: '',
  autostart: '',
  autorestart: '',
  startsecs: '',
  startretries: '',
  stopwaitsecs: '',
  stopasgroup: '',
  killasgroup: '',
  redirect_stderr: '',
  stdout_logfile: '',
  stdout_logfile_maxbytes: '',
  stdout_logfile_backups: '',
  environment: '',
})

const boolOptions = [
  { label: $gettext('Yes'), value: 'true' },
  { label: $gettext('No'), value: 'false' },
]

const autoRestartOptions = [
  { label: $gettext('Always'), value: 'true' },
  { label: $gettext('Never'), value: 'false' },
  { label: $gettext('On Unexpected Exit'), value: 'unexpected' },
]

const processLogModal = ref(false)

const processColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: 'PID',
    key: 'pid',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Uptime'),
    key: 'uptime',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 500,
    hideInExcel: true,
    render(row: any) {
      const items: any[] = [
        h(
          AppButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => handleShowProcessLog(row),
          },
          { default: () => $gettext('Logs') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleEditProcess(row.name),
          },
          { default: () => $gettext('Configure') },
        ),
      ]
      if (row.status != 'RUNNING') {
        items.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'primary',
              secondary: true,
              onClick: () => handleProcessStart(row.name),
            },
            { default: () => $gettext('Start') },
          ),
        )
      } else {
        items.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'warning',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'warning',
                  title: $gettext('Confirm'),
                  content: $gettext('Are you sure you want to stop process %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) await handleProcessStop(row.name)
              },
            },
            { default: () => $gettext('Stop') },
          ),
          h(
            AppButton,
            {
              size: 'small',
              type: 'primary',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'warning',
                  title: $gettext('Confirm'),
                  content: $gettext('Are you sure you want to restart process %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) await handleProcessRestart(row.name)
              },
            },
            { default: () => $gettext('Restart') },
          ),
        )
      }
      items.push(
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete process %{ name }?', {
                  name: row.name,
                }),
              })
              if (ok) await handleProcessDelete(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      )
      return h(NFlex, { size: 'small', align: 'center' }, () => items)
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  supervisor.processes(page, pageSize),
)

const clearDaemonLog = () => file.truncate(daemonLogPath)

const handleCreateProcess = async () => {
  await supervisor.createProcess(createProcessModel.value)
  refresh()
  createProcessModal.value = false
  window.$message.success($gettext('Added successfully'))
}

const handleProcessStart = async (name: string) => {
  await supervisor.startProcess(name)
  refresh()
  window.$message.success($gettext('Started successfully'))
}

const handleProcessStop = async (name: string) => {
  await supervisor.stopProcess(name)
  refresh()
  window.$message.success($gettext('Stopped successfully'))
}

const handleProcessRestart = async (name: string) => {
  await supervisor.restartProcess(name)
  refresh()
  window.$message.success($gettext('Restarted successfully'))
}

const handleProcessDelete = async (name: string) => {
  await supervisor.deleteProcess(name)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const handleShowProcessLog = async (row: any) => {
  processLog.value = await supervisor.processLog(row.name)
  processLogModal.value = true
}

const handleClearProcessLog = async () => {
  await file.truncate(processLog.value)
  processLogModalRef.value?.clear()
  window.$message.success($gettext('Cleared successfully'))
}

const handleEditProcess = async (name: string) => {
  editProcessTab.value = 'setting'
  await getProcessConfig(name)
  editProcessModal.value = true
}

const getProcessConfig = async (name: string) => {
  const [config, setting] = await Promise.all([
    supervisor.processConfig(name),
    supervisor.processSetting(name),
  ])
  editProcessModel.value.process = name
  editProcessModel.value.config = config
  settingModel.value = { ...settingModel.value, ...setting }
}

const handleSaveProcessSetting = async () => {
  await supervisor.saveProcessSetting(editProcessModel.value.process, settingModel.value)
  editProcessModal.value = false
  refresh()
  window.$message.success($gettext('Saved successfully'))
}

const handleSaveProcessConfig = async () => {
  await supervisor.saveProcessConfig(editProcessModel.value.process, editProcessModel.value.config)
  editProcessModal.value = false
  refresh()
  window.$message.success($gettext('Saved successfully'))
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status v-if="serviceName != ''" :service="serviceName" />
      </n-tab-pane>
      <n-tab-pane name="processes" :tab="$gettext('Process Management')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="createProcessModal = true">
              {{ $gettext('Add Process') }}
            </n-button>
          </n-flex>
          <n-data-table
            v-model:page="page"
            v-model:pageSize="pageSize"
            striped
            remote
            :scroll-x="1100"
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
          :fetch="supervisor.config"
          :save="supervisor.saveConfig"
          :tip="
            $gettext(
              'This modifies the Supervisor main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
          @saved="refresh"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log :service="serviceName" />
      </n-tab-pane>
      <n-tab-pane name="log" :tab="$gettext('Daemon Logs')">
        <clearable-log :path="daemonLogPath" :clear="clearDaemonLog" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
  <app-modal v-model:show="createProcessModal" :title="$gettext('Add Process')">
    <n-form :model="createProcessModel">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="createProcessModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Name cannot contain Chinese characters')"
        />
      </n-form-item>
      <n-form-item path="command" :label="$gettext('Start Command')">
        <n-input
          v-model:value="createProcessModel.command"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Please enter absolute path for files in start command')"
        />
      </n-form-item>
      <n-form-item path="path" :label="$gettext('Working Directory')">
        <n-input
          v-model:value="createProcessModel.path"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Please enter absolute path for working directory')"
        />
      </n-form-item>
      <n-form-item path="user" :label="$gettext('Run As User')">
        <n-input
          v-model:value="createProcessModel.user"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Usually www is sufficient')"
        />
      </n-form-item>
      <n-form-item path="num" :label="$gettext('Number of Processes')">
        <n-input-number v-model:value="createProcessModel.num" :min="1" />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleCreateProcess">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <realtime-log-modal
    ref="processLogModalRef"
    v-model:show="processLogModal"
    :path="processLog"
    clearable
    @clear="handleClearProcessLog"
  />
  <app-modal v-model:show="editProcessModal" size="lg" :title="$gettext('Process Configuration')">
    <n-tabs v-model:value="editProcessTab" type="line" animated>
      <n-tab-pane name="setting" :tab="$gettext('Parameter Settings')">
        <n-flex vertical>
          <n-alert type="info">
            {{ $gettext('Leave a field empty to comment out the option and use the default.') }}
          </n-alert>
          <n-form :model="settingModel">
            <n-form-item :label="$gettext('Start Command (command)')">
              <n-input v-model:value="settingModel.command" @keydown.enter.prevent />
            </n-form-item>
            <n-form-item :label="$gettext('Working Directory (directory)')">
              <n-input v-model:value="settingModel.directory" @keydown.enter.prevent />
            </n-form-item>
            <n-form-item :label="$gettext('Run As User (user)')">
              <n-input v-model:value="settingModel.user" @keydown.enter.prevent />
            </n-form-item>
            <n-form-item :label="$gettext('Number of Processes (numprocs)')">
              <n-input v-model:value="settingModel.numprocs" placeholder="1" />
            </n-form-item>
            <n-form-item :label="$gettext('Priority (priority)')">
              <n-input v-model:value="settingModel.priority" placeholder="999" />
            </n-form-item>
            <n-form-item :label="$gettext('Start On Boot (autostart)')">
              <n-select v-model:value="settingModel.autostart" :options="boolOptions" clearable />
            </n-form-item>
            <n-form-item :label="$gettext('Auto Restart (autorestart)')">
              <n-select
                v-model:value="settingModel.autorestart"
                :options="autoRestartOptions"
                clearable
              />
            </n-form-item>
            <n-form-item :label="$gettext('Seconds Before Considered Started (startsecs)')">
              <n-input v-model:value="settingModel.startsecs" placeholder="1" />
            </n-form-item>
            <n-form-item :label="$gettext('Start Retries (startretries)')">
              <n-input v-model:value="settingModel.startretries" placeholder="3" />
            </n-form-item>
            <n-form-item :label="$gettext('Seconds To Wait On Stop (stopwaitsecs)')">
              <n-input v-model:value="settingModel.stopwaitsecs" placeholder="10" />
            </n-form-item>
            <n-form-item :label="$gettext('Stop Whole Process Group (stopasgroup)')">
              <n-select v-model:value="settingModel.stopasgroup" :options="boolOptions" clearable />
            </n-form-item>
            <n-form-item :label="$gettext('Kill Whole Process Group (killasgroup)')">
              <n-select v-model:value="settingModel.killasgroup" :options="boolOptions" clearable />
            </n-form-item>
            <n-form-item :label="$gettext('Merge Stderr Into Stdout (redirect_stderr)')">
              <n-select
                v-model:value="settingModel.redirect_stderr"
                :options="boolOptions"
                clearable
              />
            </n-form-item>
            <n-form-item :label="$gettext('Log File (stdout_logfile)')">
              <n-input v-model:value="settingModel.stdout_logfile" @keydown.enter.prevent />
            </n-form-item>
            <n-form-item :label="$gettext('Log Rotate Size (stdout_logfile_maxbytes)')">
              <n-input v-model:value="settingModel.stdout_logfile_maxbytes" placeholder="2MB" />
            </n-form-item>
            <n-form-item :label="$gettext('Log Backups (stdout_logfile_backups)')">
              <n-input v-model:value="settingModel.stdout_logfile_backups" placeholder="10" />
            </n-form-item>
            <n-form-item :label="$gettext('Environment Variables (environment)')">
              <n-input
                v-model:value="settingModel.environment"
                :placeholder="$gettext('e.g. KEY=&quot;value&quot;,FOO=&quot;bar&quot;')"
              />
            </n-form-item>
          </n-form>
          <app-button type="info" block @click="handleSaveProcessSetting">
            {{ $gettext('Save') }}
          </app-button>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Raw Configuration')">
        <n-flex vertical>
          <common-editor v-model:value="editProcessModel.config" height="60vh" />
          <app-button type="info" block @click="handleSaveProcessConfig">
            {{ $gettext('Save') }}
          </app-button>
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </app-modal>
</template>
