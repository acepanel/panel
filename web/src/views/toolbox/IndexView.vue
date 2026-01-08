<script setup lang="ts">
defineOptions({
  name: 'toolbox-index'
})

import BenchmarkView from '@/views/toolbox/BenchmarkView.vue'
import DiskView from '@/views/toolbox/DiskView.vue'
import ProcessView from '@/views/toolbox/ProcessView.vue'
import SshView from '@/views/toolbox/SshView.vue'
import SystemView from '@/views/toolbox/SystemView.vue'
import WebHookView from '@/views/toolbox/WebHookView.vue'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const current = ref('process')
</script>

<template>
  <common-page show-header show-footer>
    <template #tabbar>
      <n-tabs v-model:value="current" animated>
        <n-tab name="process" :tab="$gettext('Process')" />
        <n-tab name="system" :tab="$gettext('System')" />
        <n-tab name="ssh" tab="SSH" />
        <n-tab name="disk" :tab="$gettext('Disk')" />
        <n-tab name="webhook" :tab="$gettext('WebHook')" />
        <n-tab name="benchmark" :tab="$gettext('Benchmark')" />
      </n-tabs>
    </template>
    <n-flex vertical>
      <process-view v-if="current === 'process'" />
      <system-view v-if="current === 'system'" />
      <ssh-view v-if="current === 'ssh'" />
      <disk-view v-if="current === 'disk'" />
      <web-hook-view v-if="current === 'webhook'" />
      <benchmark-view v-if="current === 'benchmark'" />
    </n-flex>
  </common-page>
</template>

<style scoped lang="scss"></style>
