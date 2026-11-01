<script setup lang="ts">
defineOptions({
  name: 'apps-openlitespeed-index',
})

import { NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import openlitespeed from '@/api/apps/openlitespeed'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import { useFormLayout } from '@/components/system/composables/useFormLayout'

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)
const currentTab = ref('status')
const saveConfigLoading = ref(false)
const clearErrorLogLoading = ref(false)

const { data: config } = useRequest(openlitespeed.config, {
  initialData: '',
})
const { data: errorLog } = useRequest(openlitespeed.errorLog, {
  initialData: '',
})
const { data: load } = useRequest(openlitespeed.load, {
  initialData: [],
})
const { data: phpList, send: fetchPHP } = useRequest(openlitespeed.php, {
  initialData: [],
})
const { data: realIP } = useRequest(openlitespeed.realIP, {
  initialData: { enabled: false, trusted: [] },
})
const realIPLoading = ref(false)

// 可信代理列表，多行文本与数组双向转换
const realIPTrusted = computed({
  get: () => (realIP.value.trusted ?? []).join('\n'),
  set: (value: string) => {
    realIP.value.trusted = value
      .split('\n')
      .map((line: string) => line.trim())
      .filter((line: string) => line !== '')
  },
})

const handleSaveRealIP = () => {
  realIPLoading.value = true
  useRequest(openlitespeed.setRealIP(realIP.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      realIPLoading.value = false
    })
}

const columns: any = [
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
    ellipsis: { tooltip: true },
  },
]

const phpColumns: any = [
  {
    title: $gettext('PHP Version'),
    key: 'version',
    minWidth: 120,
    render(row: any) {
      return 'PHP ' + String(row.version).replace(/(\d)(\d)/, '$1.$2')
    },
  },
  {
    title: 'LSPHP',
    key: 'lsphp',
    minWidth: 120,
    render(row: any) {
      return h(
        NTag,
        { type: row.lsphp ? 'success' : 'warning', size: 'small' },
        { default: () => (row.lsphp ? $gettext('Available') : $gettext('Not Available')) },
      )
    },
  },
  {
    title: $gettext('Protocol'),
    key: 'lsapi',
    minWidth: 200,
    render(row: any) {
      return h(
        NSwitch,
        {
          value: row.lsapi,
          disabled: !row.lsphp,
          loading: switching.has(row.version),
          checkedValue: true,
          uncheckedValue: false,
          onUpdateValue: (value: boolean) => handleSetPHP(row.version, value),
        },
        {
          checked: () => 'LSAPI',
          unchecked: () => 'FastCGI',
        },
      )
    },
  },
]

const switching = reactive(new Set<number>())

const handleSetPHP = (version: number, lsapi: boolean) => {
  switching.add(version)
  useRequest(openlitespeed.setPHP(version, lsapi))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      fetchPHP()
    })
    .onComplete(() => switching.delete(version))
}

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(openlitespeed.saveConfig(config.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}

const errorLogRef = ref<{ clear: () => void } | null>(null)

const handleClearErrorLog = () => {
  clearErrorLogLoading.value = true
  useRequest(openlitespeed.clearErrorLog())
    .onSuccess(() => {
      errorLogRef.value?.clear()
      window.$message.success($gettext('Cleared successfully'))
    })
    .onComplete(() => {
      clearErrorLogLoading.value = false
    })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="openlitespeed" show-reload />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the %{name} main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
                { name: 'OpenLiteSpeed' },
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" lang="plaintext" height="60vh" />
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
      <n-tab-pane name="php" :tab="$gettext('PHP Protocol')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'LSAPI is the native protocol of OpenLiteSpeed and performs better than FastCGI. It requires the LSPHP binary shipped with the panel PHP; if it is not available, reinstall the PHP version first.',
              )
            }}
          </n-alert>
          <n-data-table striped remote :scroll-x="400" :columns="phpColumns" :data="phpList" />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="realip" :tab="$gettext('Real IP')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'OpenLiteSpeed reads the client IP from the X-Forwarded-For header at the server level only, so this setting applies to all websites. Fill in the trusted proxy IPs (e.g., CDN or Frp); leave it empty to trust every source [insecure].',
              )
            }}
          </n-alert>
          <n-form v-bind="formLayout">
            <n-form-item :label="$gettext('Enable')">
              <n-switch v-model:value="realIP.enabled" />
            </n-form-item>
            <n-form-item v-if="realIP.enabled" :label="$gettext('IP Sources')">
              <n-input
                v-model:value="realIPTrusted"
                type="textarea"
                :placeholder="$gettext('One per line, e.g., 127.0.0.1 or 10.0.0.0/8')"
                :autosize="{ minRows: 3, maxRows: 10 }"
              />
            </n-form-item>
          </n-form>
          <n-flex>
            <n-button
              type="primary"
              :loading="realIPLoading"
              :disabled="realIPLoading"
              @click="handleSaveRealIP"
            >
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <n-data-table striped remote :loading="false" :columns="columns" :data="load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="openlitespeed" />
      </n-tab-pane>
      <n-tab-pane name="error-log" :tab="$gettext('Error Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button
              type="primary"
              :loading="clearErrorLogLoading"
              :disabled="clearErrorLogLoading"
              @click="handleClearErrorLog"
            >
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log ref="errorLogRef" :path="errorLog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
