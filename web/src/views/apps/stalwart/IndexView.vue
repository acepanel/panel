<script setup lang="ts">
defineOptions({
  name: 'apps-stalwart-index',
})

import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

import AccountView from './AccountView.vue'
import DomainView from './DomainView.vue'
import QueueView from './QueueView.vue'
import SettingView from './SettingView.vue'
import StatusView from './StatusView.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')

const status = ref({
  installed: false,
  initialized: false,
  running: false,
  hostname: '',
  domain_count: 0,
  http_port: 8080,
  log_path: '',
  proxy_website_id: 0,
  proxy_website_name: '',
  proxy_domains: [] as string[],
  proxy_missing_domains: [] as string[],
})

const refreshStatus = async () => {
  status.value = await stalwart.status()
}

onMounted(() => {
  refreshStatus()
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="stalwart" />
          <status-view :status="status" @refresh="refreshStatus" />
        </n-flex>
      </n-tab-pane>
      <template v-if="status.initialized">
        <n-tab-pane name="domains" :tab="$gettext('Domains')">
          <domain-view v-if="currentTab === 'domains'" />
        </n-tab-pane>
        <n-tab-pane name="accounts" :tab="$gettext('Mailboxes')">
          <account-view v-if="currentTab === 'accounts'" />
        </n-tab-pane>
        <n-tab-pane name="queue" :tab="$gettext('Mail Queue')">
          <queue-view v-if="currentTab === 'queue'" />
        </n-tab-pane>
        <n-tab-pane name="settings" :tab="$gettext('Settings')">
          <setting-view v-if="currentTab === 'settings'" @refresh="refreshStatus" />
        </n-tab-pane>
      </template>
      <n-tab-pane name="run-log" :tab="$gettext('Run Log')">
        <realtime-log v-if="status.log_path" :path="status.log_path" />
        <n-alert v-else type="info">
          {{ $gettext('The mail server has not written any log yet') }}
        </n-alert>
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
