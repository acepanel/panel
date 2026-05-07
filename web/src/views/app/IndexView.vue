<script lang="ts" setup>
defineOptions({
  name: 'app-index',
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import { useTabStore } from '@/stores'
import AppView from '@/views/app/AppView.vue'
import EnvironmentView from '@/views/app/EnvironmentView.vue'
import InstalledView from '@/views/app/InstalledView.vue'
import TemplateView from '@/views/app/TemplateView.vue'

const { $gettext } = useGettext()
const tabStore = useTabStore()

const currentTab = ref('installed')
const updateCacheLoading = ref(false)

const handleUpdateCache = () => {
  updateCacheLoading.value = true
  useRequest(app.updateCache())
    .onSuccess(() => {
      window.$message.success($gettext('Cache updated successfully'))
      tabStore.reloadTab(tabStore.active)
    })
    .onComplete(() => {
      updateCacheLoading.value = false
    })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <div class="flex items-center justify-between">
        <n-tabs v-model:value="currentTab" animated class="flex-1">
          <n-tab name="installed" :tab="$gettext('Installed')" />
          <n-tab name="app" :tab="$gettext('Native App')" />
          <n-tab name="environment" :tab="$gettext('Operating Environment')" />
          <n-tab name="template" :tab="$gettext('Container Template')" />
        </n-tabs>
        <n-button
          v-if="currentTab != 'installed'"
          type="primary"
          :loading="updateCacheLoading"
          :disabled="updateCacheLoading"
          @click="handleUpdateCache"
        >
          {{ $gettext('Update Cache') }}
        </n-button>
      </div>
    </template>
    <installed-view v-if="currentTab === 'installed'" />
    <app-view v-if="currentTab === 'app'" />
    <environment-view v-if="currentTab === 'environment'" />
    <template-view v-if="currentTab === 'template'" />
  </PageContainer>
</template>
