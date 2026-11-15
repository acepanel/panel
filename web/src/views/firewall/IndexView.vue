<script setup lang="ts">
defineOptions({
  name: 'safe-index',
})

import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import { useBus } from '@/components/system/composables/useBus'
import ForwardView from '@/views/firewall/ForwardView.vue'
import IpRuleView from '@/views/firewall/IpRuleView.vue'
import RuleView from '@/views/firewall/RuleView.vue'
import ScanView from '@/views/firewall/ScanView.vue'
import SettingView from '@/views/firewall/SettingView.vue'
import TamperView from '@/views/tamper/TamperView.vue'

const { $gettext } = useGettext()
const currentTab = ref('rule')

// 规则类标签页只在系统防火墙运行时展示，状态未知前不渲染以免标签闪动
// n-tabs 对 v-if 动态增删子 tab 不会重算指示条位置，用 key 强制重挂
const ruleTabs = ['rule', 'ip-rule', 'forward']
const firewallRunning = ref(false)
const firewallName = ref('')
const ready = ref(false)

const { send: refreshStatus } = useRequest(firewall.status)
  .onSuccess(({ data }) => {
    firewallRunning.value = data.running
    firewallName.value = data.name
    if (!data.running && ruleTabs.includes(currentTab.value)) currentTab.value = 'scan'
  })
  .onComplete(() => {
    ready.value = true
  })

useBus('firewall:refresh', refreshStatus)
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-if="ready" :key="String(firewallRunning)" v-model:value="currentTab" animated>
        <n-tab v-if="firewallRunning" name="rule" :tab="$gettext('Port Rules')" />
        <n-tab v-if="firewallRunning" name="ip-rule" :tab="$gettext('IP Rules')" />
        <n-tab v-if="firewallRunning" name="forward" :tab="$gettext('Port Forwarding')" />
        <n-tab name="scan" :tab="$gettext('Scan Awareness')" />
        <n-tab name="tamper" :tab="$gettext('Tamper Protection')" />
        <n-tab name="setting" :tab="$gettext('Settings')" />
      </n-tabs>
    </template>
    <n-flex v-if="ready" vertical :size="20">
      <template v-if="currentTab !== 'tamper'">
        <n-alert v-if="firewallName" type="info" :bordered="false">
          {{ $gettext('Current firewall: %{ name }', { name: firewallName }) }}
        </n-alert>
        <n-alert v-else type="warning" :bordered="false">
          {{ $gettext('No supported firewall detected, please install firewalld or ufw') }}
        </n-alert>
      </template>
      <rule-view v-if="currentTab === 'rule'" />
      <ip-rule-view v-if="currentTab === 'ip-rule'" />
      <forward-view v-if="currentTab === 'forward'" />
      <scan-view v-if="currentTab === 'scan'" />
      <tamper-view v-if="currentTab === 'tamper'" />
      <setting-view v-if="currentTab === 'setting'" />
    </n-flex>
  </PageContainer>
</template>

<style scoped lang="scss"></style>
