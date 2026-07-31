<script setup lang="ts">
defineOptions({
  name: 'waf-index',
})

import { useRouter } from 'vue-router'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import AttackMapView from '@/views/waf/AttackMapView.vue'
import BindingView from '@/views/waf/BindingView.vue'
import DecisionView from '@/views/waf/DecisionView.vue'
import EventView from '@/views/waf/EventView.vue'
import PolicyView from '@/views/waf/PolicyView.vue'
import ReportView from '@/views/waf/ReportView.vue'

const { $gettext } = useGettext()
const router = useRouter()
const currentTab = ref('policy')

const ready = ref(false)
const installed = ref(false)

onMounted(() => {
  useRequest(app.isInstalled('acewaf')).onSuccess(({ data }: any) => {
    installed.value = !!data
    ready.value = true
  })
})

const goAppStore = () => {
  router.push({ name: 'app-index' })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-if="ready && installed" v-model:value="currentTab" animated>
        <n-tab name="policy" :tab="$gettext('Policies')" />
        <n-tab name="decision" :tab="$gettext('Allow/Deny List')" />
        <n-tab name="binding" :tab="$gettext('Website Binding')" />
        <n-tab name="attack-map" :tab="$gettext('Attack Map')" />
        <n-tab name="report" :tab="$gettext('Dashboard')" />
        <n-tab name="event" :tab="$gettext('Event Logs')" />
      </n-tabs>
    </template>

    <!-- 未安装 acewaf:提示安装组件 + 重装带 WAF 模块的 nginx -->
    <n-result
      v-if="ready && !installed"
      status="info"
      :title="$gettext('WAF is not installed')"
      class="mt-20"
    >
      <template #footer>
        <n-flex vertical align="center" :size="16">
          <n-text depth="3">
            {{
              $gettext(
                'WAF requires the acewaf component. Please install it from the App Store, and install or reinstall nginx with the WAF module enabled.',
              )
            }}
          </n-text>
          <n-button type="primary" @click="goAppStore">
            {{ $gettext('Go to App Store') }}
          </n-button>
        </n-flex>
      </template>
    </n-result>

    <!-- 已安装:功能页 -->
    <template v-else-if="ready">
      <policy-view v-if="currentTab === 'policy'" />
      <decision-view v-if="currentTab === 'decision'" />
      <binding-view v-if="currentTab === 'binding'" />
      <attack-map-view v-if="currentTab === 'attack-map'" />
      <report-view v-if="currentTab === 'report'" />
      <event-view v-if="currentTab === 'event'" />
    </template>
  </PageContainer>
</template>

<style scoped lang="scss"></style>
