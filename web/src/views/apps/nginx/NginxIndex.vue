<script setup lang="ts">
import type nginx from '@/api/apps/nginx'

import NginxConfigTuneView from './NginxConfigTuneView.vue'
import NginxStreamView from './NginxStreamView.vue'

const props = defineProps<{
  api: typeof nginx
  name: 'Nginx' | 'OpenResty'
}>()

const currentTab = ref('status')

const { data: errorLog } = useRequest(props.api.errorLog, {
  initialData: '',
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="nginx" show-reload />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <config-editor-pane
          :fetch="props.api.config"
          :save="props.api.saveConfig"
          :tip="
            $gettext(
              'This modifies the %{name} main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
              { name },
            )
          "
          lang="nginx"
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <nginx-config-tune-view :api="props.api" />
      </n-tab-pane>
      <n-tab-pane name="stream" :tab="$gettext('Stream')">
        <nginx-stream-view :api="props.api" />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="props.api.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <clearable-log service="nginx" />
      </n-tab-pane>
      <n-tab-pane name="error-log" :tab="$gettext('Error Logs')">
        <clearable-log :path="errorLog" :clear="props.api.clearErrorLog" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
