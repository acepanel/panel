<script setup lang="ts">
defineOptions({
  name: 'monitor-index',
})

import { useGettext } from 'vue3-gettext'

import AlertView from '@/views/monitor/AlertView.vue'
import SettingView from '@/views/monitor/SettingView.vue'
import SystemView from '@/views/monitor/SystemView.vue'

const { $gettext } = useGettext()
const currentTab = ref('system')
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="system" :tab="$gettext('System')" />
        <n-tab name="alert" :tab="$gettext('Alerts')" />
        <n-tab name="setting" :tab="$gettext('Settings')" />
      </n-tabs>
    </template>
    <system-view v-if="currentTab === 'system'" />
    <alert-view v-if="currentTab === 'alert'" />
    <setting-view v-if="currentTab === 'setting'" />
  </PageContainer>
</template>

<style scoped lang="scss"></style>
