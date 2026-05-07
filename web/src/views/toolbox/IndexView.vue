<script setup lang="ts">
defineOptions({
  name: 'toolbox-index',
})

import { useGettext } from 'vue3-gettext'

import BenchmarkView from '@/views/toolbox/BenchmarkView.vue'
import DiskView from '@/views/toolbox/DiskView.vue'
import LogView from '@/views/toolbox/LogView.vue'
import MigrationView from '@/views/toolbox/MigrationView.vue'
import NetworkView from '@/views/toolbox/NetworkView.vue'
import PanelView from '@/views/toolbox/PanelView.vue'
import ProcessView from '@/views/toolbox/ProcessView.vue'
import SshView from '@/views/toolbox/SshView.vue'
import SystemView from '@/views/toolbox/SystemView.vue'
import WebHookView from '@/views/toolbox/WebHookView.vue'

const { $gettext } = useGettext()
const current = ref('process')
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="current" animated>
        <n-tab name="process" :tab="$gettext('Process')" />
        <n-tab name="network" :tab="$gettext('Network')" />
        <n-tab name="system" :tab="$gettext('System')" />
        <n-tab name="ssh" tab="SSH" />
        <n-tab name="disk" :tab="$gettext('Disk')" />
        <n-tab name="log" :tab="$gettext('Log Clean')" />
        <n-tab name="webhook" :tab="$gettext('WebHook')" />
        <n-tab name="benchmark" :tab="$gettext('Benchmark')" />
        <n-tab name="migration" :tab="$gettext('Migration')" />
        <n-tab name="panel" :tab="$gettext('Panel')" />
      </n-tabs>
    </template>
    <n-flex vertical>
      <process-view v-if="current === 'process'" />
      <network-view v-if="current === 'network'" />
      <system-view v-if="current === 'system'" />
      <ssh-view v-if="current === 'ssh'" />
      <disk-view v-if="current === 'disk'" />
      <log-view v-if="current === 'log'" />
      <web-hook-view v-if="current === 'webhook'" />
      <benchmark-view v-if="current === 'benchmark'" />
      <migration-view v-if="current === 'migration'" />
      <panel-view v-if="current === 'panel'" />
    </n-flex>
  </PageContainer>
</template>

<style scoped lang="scss"></style>
