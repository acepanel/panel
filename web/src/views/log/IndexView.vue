<script setup lang="ts">
defineOptions({
  name: 'log-index',
})

import { useGettext } from 'vue3-gettext'

import DatabaseLog from './DatabaseLog.vue'
import HttpLog from './HttpLog.vue'
import OperationLog from './OperationLog.vue'
import SSHLog from './SSHLog.vue'

const { $gettext } = useGettext()

// 当前激活的标签
const activeTab = ref('operation')
</script>

<template>
  <PageContainer :show-footer="true" flex>
    <template #tabs>
      <n-tabs v-model:value="activeTab" animated>
        <n-tab name="operation" :tab="$gettext('Operation Log')" />
        <n-tab name="database" :tab="$gettext('Database Log')" />
        <n-tab name="http" :tab="$gettext('HTTP Log')" />
        <n-tab name="ssh" :tab="$gettext('SSH Log')" />
      </n-tabs>
    </template>
    <operation-log v-if="activeTab === 'operation'" />
    <database-log v-if="activeTab === 'database'" />
    <http-log v-if="activeTab === 'http'" />
    <SSHLog v-if="activeTab === 'ssh'" />
  </PageContainer>
</template>

<style scoped lang="scss"></style>
