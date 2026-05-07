<script lang="ts" setup>
defineOptions({
  name: 'project-index',
})

import CreateModal from '@/views/project/CreateModal.vue'
import EditModal from '@/views/project/EditModal.vue'
import ListView from '@/views/project/ListView.vue'

const currentTab = ref('all')

const createModal = ref(false)
const editModal = ref(false)
const editId = ref(0)
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="all" :tab="$gettext('All')" />
        <n-tab name="general" :tab="$gettext('General')" />
        <n-tab name="go" :tab="$gettext('Go')" />
        <n-tab name="java" :tab="$gettext('Java')" />
        <n-tab name="nodejs" :tab="$gettext('Node.js')" />
        <n-tab name="php" :tab="$gettext('PHP')" />
        <n-tab name="python" :tab="$gettext('Python')" />
        <n-tab name="dotnet" :tab="$gettext('.NET')" />
      </n-tabs>
    </template>
    <list-view
      v-model:type="currentTab"
      v-model:createModal="createModal"
      v-model:editModal="editModal"
      v-model:editId="editId"
    />
    <create-modal v-model:show="createModal" v-model:type="currentTab" />
    <edit-modal v-model:show="editModal" v-model:editId="editId" />
  </PageContainer>
</template>
