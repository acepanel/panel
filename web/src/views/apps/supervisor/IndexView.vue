<script setup lang="ts">
defineOptions({
  name: 'apps-supervisor-index'
})

import { NButton, NDataTable, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import supervisor from '@/api/apps/supervisor'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const saveConfigLoading = ref(false)
const clearLogLoading = ref(false)
const createProcessLoading = ref(false)
const processLog = ref('')

const { data: serviceName } = useRequest(supervisor.service, {
  initialData: ''
}).onSuccess(() => {
  refresh()
  config.value = supervisor.config()
})

const { data: config } = useRequest(supervisor.config, {
  initialData: ''
})

const createProcessModal = ref(false)
const createProcessModel = ref({
  name: '',
  user: 'www',
  path: '',
  command: '',
  num: 1
})

const editProcessModal = ref(false)
const editProcessModel = ref({
  process: '',
  config: ''
})

const processLogModal = ref(false)

const processColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Status'),
    key: 'status',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: 'PID',
    key: 'pid',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Uptime'),
    key: 'uptime',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 500,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            secondary: true,
            onClick: () => handleShowProcessLog(row)
          },
          {
            default: () => $gettext('Logs')
          }
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            style: 'margin-left: 15px',
            onClick: () => handleEditProcess(row.name)
          },
          {
            default: () => $gettext('Configure')
          }
        ),
        row.status != 'RUNNING'
          ? h(
              NButton,
              {
                size: 'small',
                type: 'primary',
                secondary: true,
                style: 'margin-left: 15px',
                onClick: () => handleProcessStart(row.name)
              },
              {
                default: () => $gettext('Start')
              }
            )
          : null,
        row.status == 'RUNNING'
          ? h(
              NPopconfirm,
              {
                onPositiveClick: () => handleProcessStop(row.name)
              },
              {
                default: () => {
                  return $gettext('Are you sure you want to stop process %{ name }?', {
                    name: row.name
                  })
                },
                trigger: () => {
                  return h(
                    NButton,
                    {
                      size: 'small',
                      type: 'warning',
                      style: 'margin-left: 15px'
                    },
                    {
                      default: () => $gettext('Stop')
                    }
                  )
                }
              }
            )
          : null,
        row.status == 'RUNNING'
          ? h(
              NPopconfirm,
              {
                onPositiveClick: () => handleProcessRestart(row.name)
              },
              {
                default: () => {
                  return $gettext('Are you sure you want to restart process %{ name }?', {
                    name: row.name
                  })
                },
                trigger: () => {
                  return h(
                    NButton,
                    {
                      size: 'small',
                      type: 'primary',
                      style: 'margin-left: 15px'
                    },
                    {
                      default: () => $gettext('Restart')
                    }
                  )
                }
              }
            )
          : null,
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleProcessDelete(row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete process %{ name }?', {
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
  (page, pageSize) => supervisor.processes(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(supervisor.saveConfig(config.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const handleClearLog = () => {
  clearLogLoading.value = true
  useRequest(supervisor.clearLog())
    .onSuccess(() => {
      window.$message.success($gettext('Cleared successfully'))
    })
    .onComplete(() => {
      clearLogLoading.value = false
    })
}

const handleCreateProcess = () => {
  createProcessLoading.value = true
  useRequest(supervisor.createProcess(createProcessModel.value))
    .onSuccess(() => {
      refresh()
      createProcessModal.value = false
      window.$message.success($gettext('Added successfully'))
    })
    .onComplete(() => {
      createProcessLoading.value = false
    })
}

const handleProcessStart = (name: string) => {
  useRequest(supervisor.startProcess(name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Started successfully'))
  })
}

const handleProcessStop = (name: string) => {
  useRequest(supervisor.stopProcess(name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Stopped successfully'))
  })
}

const handleProcessRestart = (name: string) => {
  useRequest(supervisor.restartProcess(name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Restarted successfully'))
  })
}

const handleProcessDelete = (name: string) => {
  useRequest(supervisor.deleteProcess(name)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleShowProcessLog = async (row: any) => {
  processLog.value = await supervisor.processLog(row.name)
  processLogModal.value = true
}

const handleEditProcess = async (name: string) => {
  await getProcessConfig(name)
  editProcessModal.value = true
}

const getProcessConfig = async (name: string) => {
  editProcessModel.value.process = name
  editProcessModel.value.config = await supervisor.processConfig(name)
}

const handleSaveProcessConfig = () => {
  useRequest(
    supervisor.saveProcessConfig(editProcessModel.value.process, editProcessModel.value.config)
  ).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const timer: any = null

onUnmounted(() => {
  clearInterval(timer)
})
</script>

<template>
  <common-page show-footer>
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
                'This modifies the Supervisor main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
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
        <realtime-log service="supervisor" />
      </n-tab-pane>
      <n-tab-pane name="log" :tab="$gettext('Daemon Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" :loading="clearLogLoading" :disabled="clearLogLoading" @click="handleClearLog">
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log path="/var/log/supervisor/supervisord.log" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </common-page>
  <n-modal
    v-model:show="createProcessModal"
    preset="card"
    :title="$gettext('Add Process')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="createProcessModal = false"
  >
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
    <n-button type="info" block :loading="createProcessLoading" :disabled="createProcessLoading" @click="handleCreateProcess">{{ $gettext('Submit') }}</n-button>
  </n-modal>
  <realtime-log-modal v-model:show="processLogModal" :path="processLog" />
  <n-modal
    v-model:show="editProcessModal"
    preset="card"
    :title="$gettext('Process Configuration')"
    style="width: 80vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="handleSaveProcessConfig"
  >
    <common-editor v-model:value="editProcessModel.config" height="60vh" />
  </n-modal>
</template>
