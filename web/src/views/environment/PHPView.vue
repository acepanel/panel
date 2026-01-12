<script setup lang="ts">
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import { NButton, NDataTable, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import php from '@/api/panel/environment/php'

const route = useRoute()
const slug = Number(route.params.slug)

const { $gettext } = useGettext()

const currentTab = ref('status')

// phpinfo 相关状态
const showPHPInfoModal = ref(false)
const phpinfoContent = ref('')
const phpinfoLoading = ref(false)

const { data: config } = useRequest(php.config(slug), {
  initialData: ''
})
const { data: fpmConfig } = useRequest(php.fpmConfig(slug), {
  initialData: ''
})
const { data: log } = useRequest(php.log(slug), {
  initialData: ''
})
const { data: slowLog } = useRequest(php.slowLog(slug), {
  initialData: ''
})
const { data: load } = useRequest(php.load(slug), {
  initialData: []
})
const { data: modules } = useRequest(php.modules(slug), {
  initialData: []
})

const moduleColumns: any = [
  {
    title: $gettext('Module Name'),
    key: 'name',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Description'),
    key: 'description',
    resizable: true,
    minWidth: 250,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 240,
    hideInExcel: true,
    render(row: any) {
      return [
        !row.installed
          ? h(
              NPopconfirm,
              {
                onPositiveClick: () => handleInstallModule(row.slug)
              },
              {
                default: () => {
                  return $gettext('Are you sure you want to install %{ name }?', { name: row.name })
                },
                trigger: () => {
                  return h(
                    NButton,
                    {
                      size: 'small',
                      type: 'info'
                    },
                    {
                      default: () => $gettext('Install')
                    }
                  )
                }
              }
            )
          : null,
        row.installed
          ? h(
              NPopconfirm,
              {
                onPositiveClick: () => handleUninstallModule(row.slug)
              },
              {
                default: () => {
                  return $gettext('Are you sure you want to uninstall %{ name }?', {
                    name: row.name
                  })
                },
                trigger: () => {
                  return h(
                    NButton,
                    {
                      size: 'small',
                      type: 'error'
                    },
                    {
                      default: () => $gettext('Delete')
                    }
                  )
                }
              }
            )
          : null
      ]
    }
  }
]

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

const handleSetCli = async () => {
  useRequest(php.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}

const handlePHPInfo = async () => {
  phpinfoLoading.value = true
  showPHPInfoModal.value = true
  useRequest(php.phpinfo(slug))
    .onSuccess((res) => {
      phpinfoContent.value = res.data
    })
    .onComplete(() => {
      phpinfoLoading.value = false
    })
}

const handleSaveConfig = async () => {
  useRequest(php.saveConfig(slug, config.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleSaveFPMConfig = async () => {
  useRequest(php.saveFPMConfig(slug, fpmConfig.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleClearLog = async () => {
  useRequest(php.clearLog(slug)).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
  })
}

const handleClearSlowLog = async () => {
  useRequest(php.clearSlowLog(slug)).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
  })
}

const handleInstallModule = async (module: string) => {
  useRequest(php.installModule(slug, module)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
  })
}

const handleUninstallModule = async (module: string) => {
  useRequest(php.uninstallModule(slug, module)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
  })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <n-card> PHP {{ slug }} </n-card>
          <service-status :service="`php-fpm-${slug}`" show-reload />
          <n-flex>
            <n-button type="info" @click="handleSetCli">
              {{ $gettext('Set as CLI Default Version') }}
            </n-button>
            <n-button type="primary" @click="handlePHPInfo">
              {{ $gettext('View PHPInfo') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="modules" :tab="$gettext('Module Management')">
        <n-flex vertical>
          <n-data-table
            striped
            remote
            :scroll-x="1000"
            :loading="false"
            :columns="moduleColumns"
            :data="modules"
            :row-key="(row: any) => row.slug"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the PHP %{ version } main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                { version: slug }
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
      <n-tab-pane name="fpm-config" :tab="$gettext('FPM Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the PHP %{ version } FPM configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                { version: slug }
              )
            }}
          </n-alert>
          <common-editor v-model:value="fpmConfig" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveFPMConfig">
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
        <realtime-log :service="'php-fpm-' + slug" />
      </n-tab-pane>
      <n-tab-pane name="log" :tab="$gettext('Error Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="handleClearLog">
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log :path="log" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="slow-log" :tab="$gettext('Slow Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="handleClearSlowLog">
              {{ $gettext('Clear Slow Log') }}
            </n-button>
          </n-flex>
          <realtime-log :path="slowLog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>

    <!-- PHPInfo 弹窗 -->
    <n-modal
      v-model:show="showPHPInfoModal"
      preset="card"
      :title="$gettext('PHPInfo') + ' - PHP ' + slug"
      style="width: 90%; max-width: 1200px"
      :mask-closable="true"
    >
      <n-spin :show="phpinfoLoading">
        <n-scrollbar style="max-height: 70vh">
          <div class="phpinfo-content" v-html="phpinfoContent"></div>
        </n-scrollbar>
      </n-spin>
    </n-modal>
  </common-page>
</template>

<style scoped lang="scss">
.phpinfo-content {
  :deep(table) {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 10px;
  }
  :deep(td),
  :deep(th) {
    border: 1px solid #ccc;
    padding: 4px 8px;
    text-align: left;
    vertical-align: top;
  }
  :deep(th) {
    background-color: #f0f0f0;
  }
  :deep(h1) {
    font-size: 24px;
    margin: 10px 0;
  }
  :deep(h2) {
    font-size: 18px;
    margin: 10px 0;
    background-color: #9999cc;
    color: white;
    padding: 5px;
  }
  :deep(hr) {
    display: none;
  }
}
</style>
