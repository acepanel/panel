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
  <common-page show-header show-footer flex>
    <template #tabbar>
      <n-tabs v-model:value="activeTab" animated>
        <n-tab name="operation" :tab="$gettext('Operation Log')" />
        <n-tab name="database" :tab="$gettext('Database Log')" />
        <n-tab name="http" :tab="$gettext('HTTP Log')" />
      </n-tabs>
    </template>
    <operation-log v-if="activeTab === 'operation'" />
    <database-log v-if="activeTab === 'database'" />
    <http-log v-if="activeTab === 'http'" />
  </common-page>
</template>

<style scoped lang="scss"></style>
