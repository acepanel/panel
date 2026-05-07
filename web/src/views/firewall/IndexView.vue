<script setup lang="ts">
defineOptions({
  name: 'firewall-index',
})

import { useGettext } from 'vue3-gettext'

import ForwardView from '@/views/firewall/ForwardView.vue'
import IpRuleView from '@/views/firewall/IpRuleView.vue'
import RuleView from '@/views/firewall/RuleView.vue'
import ScanView from '@/views/firewall/ScanView.vue'
import SettingView from '@/views/firewall/SettingView.vue'

const { $gettext } = useGettext()
const currentTab = ref('rule')
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="rule" :tab="$gettext('Port Rules')" />
        <n-tab name="ip-rule" :tab="$gettext('IP Rules')" />
        <n-tab name="forward" :tab="$gettext('Port Forwarding')" />
        <n-tab name="scan" :tab="$gettext('Scan Awareness')" />
        <n-tab name="setting" :tab="$gettext('Settings')" />
      </n-tabs>
    </template>
    <rule-view v-if="currentTab === 'rule'" />
    <ip-rule-view v-if="currentTab === 'ip-rule'" />
    <forward-view v-if="currentTab === 'forward'" />
    <scan-view v-if="currentTab === 'scan'" />
    <setting-view v-if="currentTab === 'setting'" />
  </PageContainer>
</template>

<style scoped lang="scss"></style>
