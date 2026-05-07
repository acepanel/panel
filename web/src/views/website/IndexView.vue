<script lang="ts" setup>
defineOptions({
  name: 'website-index',
})

import BulkCreateModal from '@/views/website/BulkCreateModal.vue'
import CreateModal from '@/views/website/CreateModal.vue'
import EditModal from '@/views/website/EditModal.vue'
import ListView from '@/views/website/ListView.vue'
import SettingView from '@/views/website/SettingView.vue'
import StatsView from '@/views/website/StatsView.vue'

const currentTab = ref('all')

const createModal = ref(false)
const bulkCreateModal = ref(false)
const editModal = ref(false)
const editId = ref(0)
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="all" :tab="$gettext('All')" />
        <n-tab name="proxy" :tab="$gettext('Reverse Proxy')" />
        <n-tab name="php" :tab="$gettext('PHP')" />
        <n-tab name="static" :tab="$gettext('Pure Static')" />
        <n-tab name="stats" :tab="$gettext('Stats')" />
        <n-tab name="setting" :tab="$gettext('Settings')" />
      </n-tabs>
    </template>
    <list-view
      v-if="currentTab != 'setting' && currentTab != 'stats'"
      v-model:type="currentTab"
      v-model:create-modal="createModal"
      v-model:bulk-create-modal="bulkCreateModal"
      v-model:edit-modal="editModal"
      v-model:edit-id="editId"
    />
    <stats-view v-if="currentTab === 'stats'" />
    <setting-view v-if="currentTab === 'setting'" />
    <create-modal v-model:show="createModal" v-model:type="currentTab" />
    <bulk-create-modal v-model:show="bulkCreateModal" v-model:type="currentTab" />
    <edit-modal v-model:show="editModal" v-model:edit-id="editId" />
  </PageContainer>
</template>
