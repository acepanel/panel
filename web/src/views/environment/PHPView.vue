<script setup lang="ts">
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import openlitespeed from '@/api/apps/openlitespeed'
import php from '@/api/panel/environment/php'
import file from '@/api/panel/file'
import home from '@/api/panel/home'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatBytes } from '@/utils/file'
import PhpConfigTuneView from '@/views/environment/PhpConfigTuneView.vue'

const { confirmAction } = useConfirm()

const route = useRoute()
const slug = Number(route.params.slug)

const { $gettext } = useGettext()

const currentTab = ref('status')

// 走 LSAPI 时 PHP 由 OpenLiteSpeed 自己拉起
const lsapiTakeover = ref(false)
useRequest(home.installedEnvironment()).onSuccess(async ({ data }: any) => {
  if (data?.webserver !== 'openlitespeed') return
  const list = await openlitespeed.php()
  lsapiTakeover.value = (list ?? []).some((item: any) => item.version === slug && item.lsapi)
})

// phpinfo 相关状态
const showPHPInfoModal = ref(false)
const phpinfoContent = ref('')
const phpinfoLoading = ref(false)

const { data: config, send: refreshConfig } = useRequest(php.config(slug), {
  initialData: '',
})
const { data: fpmConfig, send: refreshFpmConfig } = useRequest(php.fpmConfig(slug), {
  initialData: '',
})

watch(currentTab, (val) => {
  if (val === 'config') {
    refreshConfig()
  } else if (val === 'fpm-config') {
    refreshFpmConfig()
  }
})
const logRef = ref<{ clear: () => void } | null>(null)
const slowLogRef = ref<{ clear: () => void } | null>(null)

const { data: log } = useRequest(php.log(slug), {
  initialData: '',
})
const { data: slowLog } = useRequest(php.slowLog(slug), {
  initialData: '',
})
const { data: load } = useRequest(php.load(slug), {
  initialData: [],
})
const { data: modules } = useRequest(php.modules(slug), {
  initialData: [],
})
const { data: processes, send: refreshProcesses } = useRequest(php.processes(slug), {
  initialData: [],
})
const { data: opcache, send: refreshOpcache } = useRequest(php.opcache(slug), {
  initialData: { enabled: true },
})
const { data: composer, send: refreshComposer } = useRequest(php.composer(slug), {
  initialData: { installed: true, version: '', mirror: '' },
})

const composerMirror = ref('')
watch(
  () => composer.value?.mirror,
  (val) => {
    composerMirror.value = val ?? ''
  },
)

const composerMirrorOptions = computed(() => [
  { label: $gettext('Official'), value: '' },
  { label: $gettext('Aliyun Mirror'), value: 'https://mirrors.aliyun.com/composer/' },
  { label: $gettext('Tencent Mirror'), value: 'https://mirrors.tencent.com/composer/' },
])

const processColumns: any = [
  { title: 'PID', key: 'pid', width: 90 },
  { title: $gettext('State'), key: 'state', width: 130, ellipsis: { tooltip: true } },
  {
    title: $gettext('Requests'),
    key: 'requests',
    width: 100,
    sorter: (a: any, b: any) => a.requests - b.requests,
  },
  { title: $gettext('Method'), key: 'method', width: 90 },
  { title: 'URI', key: 'uri', minWidth: 250, ellipsis: { tooltip: true } },
  {
    title: $gettext('Duration (ms)'),
    key: 'request_duration',
    width: 150,
    sorter: (a: any, b: any) => a.request_duration - b.request_duration,
    render: (row: any) => Math.round(row.request_duration / 1000),
  },
  {
    title: $gettext('Memory'),
    key: 'last_request_memory',
    width: 110,
    sorter: (a: any, b: any) => a.last_request_memory - b.last_request_memory,
    render: (row: any) => formatBytes(row.last_request_memory),
  },
  { title: $gettext('Script'), key: 'script', minWidth: 250, ellipsis: { tooltip: true } },
]

const moduleColumns: any = [
  {
    title: $gettext('Module Name'),
    key: 'name',
    minWidth: 250,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    resizable: true,
    minWidth: 250,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 240,
    hideInExcel: true,
    render(row: any) {
      return h(
        NButton,
        row.installed
          ? {
              size: 'small',
              type: 'error',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'warning',
                  title: $gettext('Confirm Uninstall'),
                  content: $gettext('Are you sure you want to uninstall %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) handleUninstallModule(row.slug)
              },
            }
          : {
              size: 'small',
              type: 'info',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'info',
                  title: $gettext('Confirm Install'),
                  content: $gettext('Are you sure you want to install %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) handleInstallModule(row.slug)
              },
            },
        { default: () => (row.installed ? $gettext('Delete') : $gettext('Install')) },
      )
    },
  },
]

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
  useRequest(file.truncate(log.value)).onSuccess(() => {
    logRef.value?.clear()
    window.$message.success($gettext('Cleared successfully'))
  })
}

const handleClearSlowLog = async () => {
  useRequest(file.truncate(slowLog.value)).onSuccess(() => {
    slowLogRef.value?.clear()
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

const handleResetOpcache = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Confirm Reset'),
    content: $gettext(
      'Resetting will clear all cached scripts, performance may fluctuate briefly. Are you sure?',
    ),
  })
  if (!ok) return
  useRequest(php.resetOpcache(slug)).onSuccess(() => {
    window.$message.success($gettext('Reset successfully'))
    refreshOpcache()
  })
}

const handleInstallComposer = async () => {
  useRequest(php.installComposer(slug)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
  })
}

const handleSaveComposerMirror = async () => {
  useRequest(php.setComposerMirror(slug, composerMirror.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
    refreshComposer()
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <!-- 限制按钮区宽度，窄屏时按钮换行，避免把标题挤成一字一行 -->
          <n-card header-extra-class="max-w-[65%]">
            <template #header> PHP {{ slug }} </template>
            <template #header-extra>
              <n-flex>
                <n-button type="info" @click="handleSetCli">
                  {{ $gettext('Set as CLI Default Version') }}
                </n-button>
                <n-button type="primary" @click="handlePHPInfo">
                  {{ $gettext('View PHPInfo') }}
                </n-button>
              </n-flex>
            </template>
          </n-card>
          <n-alert v-if="lsapiTakeover" type="info" :show-icon="true">
            {{
              $gettext(
                'PHP requests are handled by OpenLiteSpeed over LSAPI and do not go through php-fpm, so its stopped state does not affect websites.',
              )
            }}
          </n-alert>
          <service-status :service="`php-fpm-${slug}`" show-reload />
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
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <php-config-tune-view :slug="slug" />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the PHP %{ version } main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                { version: slug },
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
                { version: slug },
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
        <n-flex vertical>
          <n-alert v-if="lsapiTakeover" type="info" :show-icon="true">
            {{
              $gettext(
                'Processes are managed by OpenLiteSpeed under LSAPI, so php-fpm metrics do not apply here. Live status is on the OpenLiteSpeed app page.',
              )
            }}
          </n-alert>
          <n-data-table
            striped
            remote
            :scroll-x="400"
            :loading="false"
            :columns="loadColumns"
            :data="load"
          />
          <n-card :title="$gettext('FPM Processes')">
            <template #header-extra>
              <n-button size="small" type="primary" @click="() => refreshProcesses()">
                {{ $gettext('Refresh') }}
              </n-button>
            </template>
            <n-data-table
              striped
              :columns="processColumns"
              :data="processes"
              :scroll-x="1170"
              max-height="50vh"
            />
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="opcache" tab="OPcache">
        <n-flex vertical>
          <n-alert v-if="!opcache.enabled" type="info">
            {{ opcache.message }}
          </n-alert>
          <template v-else>
            <n-flex>
              <n-button type="primary" @click="() => refreshOpcache()">
                {{ $gettext('Refresh') }}
              </n-button>
              <n-button type="warning" @click="handleResetOpcache">
                {{ $gettext('Reset OPcache') }}
              </n-button>
            </n-flex>
            <n-card :title="$gettext('Cache Statistics')">
              <n-flex>
                <n-statistic :label="$gettext('Hit Rate')" :value="`${opcache.hit_rate}%`" />
                <n-statistic class="md:ml-40" :label="$gettext('Hits')" :value="opcache.hits" />
                <n-statistic class="md:ml-40" :label="$gettext('Misses')" :value="opcache.misses" />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('Cached Scripts')"
                  :value="opcache.cached_scripts"
                />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('Cached Keys')"
                  :value="`${opcache.cached_keys} / ${opcache.max_cached_keys}`"
                />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('OOM Restarts')"
                  :value="opcache.oom_restarts"
                />
              </n-flex>
            </n-card>
            <n-card :title="$gettext('Memory')">
              <n-flex>
                <n-statistic :label="$gettext('Used')" :value="opcache.memory_used" />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('Free')"
                  :value="opcache.memory_free"
                />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('Wasted')"
                  :value="`${opcache.memory_wasted} (${opcache.wasted_percent}%)`"
                />
              </n-flex>
            </n-card>
            <n-card title="JIT">
              <n-flex v-if="opcache.jit_enabled">
                <n-statistic :label="$gettext('Buffer Size')" :value="opcache.jit_buffer_size" />
                <n-statistic
                  class="md:ml-40"
                  :label="$gettext('Buffer Free')"
                  :value="opcache.jit_buffer_free"
                />
              </n-flex>
              <n-text v-else depth="3">{{ $gettext('JIT is not enabled') }}</n-text>
            </n-card>
          </template>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="composer" tab="Composer">
        <n-flex vertical>
          <template v-if="!composer.installed">
            <n-alert type="info">
              {{ $gettext('Composer is not installed.') }}
            </n-alert>
            <n-flex>
              <n-button type="primary" @click="handleInstallComposer">
                {{ $gettext('Install') }}
              </n-button>
            </n-flex>
          </template>
          <template v-else>
            <n-card :title="$gettext('Composer')">
              <template #header-extra>
                <n-button size="small" type="primary" @click="handleInstallComposer">
                  {{ $gettext('Update') }}
                </n-button>
              </template>
              <n-descriptions label-placement="left" :column="1">
                <n-descriptions-item :label="$gettext('Version')">
                  {{ composer.version || '-' }}
                </n-descriptions-item>
              </n-descriptions>
            </n-card>
            <n-card :title="$gettext('Mirror')">
              <n-flex vertical>
                <n-alert type="info">
                  {{
                    $gettext(
                      'The mirror is a global setting shared by all PHP versions. Use a mirror to speed up package downloads in mainland China.',
                    )
                  }}
                </n-alert>
                <n-flex>
                  <n-select
                    v-model:value="composerMirror"
                    :options="composerMirrorOptions"
                    class="md:!w-80"
                  />
                  <n-button type="primary" @click="handleSaveComposerMirror">
                    {{ $gettext('Save') }}
                  </n-button>
                </n-flex>
              </n-flex>
            </n-card>
          </template>
        </n-flex>
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
          <realtime-log ref="logRef" :path="log" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="slow-log" :tab="$gettext('Slow Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="handleClearSlowLog">
              {{ $gettext('Clear Slow Log') }}
            </n-button>
          </n-flex>
          <realtime-log ref="slowLogRef" :path="slowLog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>

    <!-- PHPInfo 弹窗 -->
    <app-modal
      v-model:show="showPHPInfoModal"
      size="full"
      :title="$gettext('PHPInfo') + ' - PHP ' + slug"
      :mask-closable="true"
    >
      <n-spin :show="phpinfoLoading">
        <n-scrollbar :style="{ maxHeight: '70vh' }">
          <div class="phpinfo-content" v-html="phpinfoContent"></div>
        </n-scrollbar>
      </n-spin>
    </app-modal>
  </PageContainer>
</template>

<style scoped lang="scss">
.phpinfo-content {
  :deep(table) {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 10px;
  }

  // phpinfo 自带样式把分隔线定为 934px、标签列定为 300px，窄屏会横向溢出或把值列挤到只剩几个字宽
  :deep(hr) {
    max-width: 100%;
  }

  @media (max-width: 767px) {
    :deep(td.e) {
      width: 40%;
    }
  }
}
</style>
