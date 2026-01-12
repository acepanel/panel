<script setup lang="ts">
defineOptions({
  name: 'log-index'
})

import { useGettext } from 'vue3-gettext'

import DatabaseLog from './DatabaseLog.vue'
import HttpLog from './HttpLog.vue'
import OperationLog from './OperationLog.vue'

const { $gettext } = useGettext()

// 当前激活的标签
const activeTab = ref('operation')
</script>

<template>
  <common-page show-header show-footer>
    <n-tabs v-model:value="activeTab" type="line" animated>
      <n-tab-pane name="operation" :tab="$gettext('Operation Log')">
        <div class="pt-4">
          <OperationLog />
        </div>
      </n-tab-pane>
      <n-tab-pane name="database" :tab="$gettext('Database Log')">
        <div class="pt-4">
          <DatabaseLog />
        </div>
      </n-tab-pane>
      <n-tab-pane name="http" :tab="$gettext('HTTP Log')">
        <div class="pt-4">
          <HttpLog />
        </div>
      </n-tab-pane>
    </n-tabs>
  </common-page>
</template>

<style scoped lang="scss"></style>
