<script lang="ts" setup>
defineOptions({
  name: 'app-index',
})

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

const handleUpdateCache = async () => {
  await app.updateCache()
  window.$message.success($gettext('Cache updated successfully'))
  tabStore.reloadTab(tabStore.active)
}
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="currentTab" animated>
        <n-tab name="installed" :tab="$gettext('Installed')" />
        <n-tab name="app" :tab="$gettext('Native App')" />
        <n-tab name="environment" :tab="$gettext('Operating Environment')" />
        <n-tab name="template" :tab="$gettext('Container Template')" />
        <template #suffix>
          <app-button type="primary" @click="handleUpdateCache">
            {{ $gettext('Update Cache') }}
          </app-button>
        </template>
      </n-tabs>
    </template>
    <installed-view v-if="currentTab === 'installed'" />
    <app-view v-if="currentTab === 'app'" />
    <environment-view v-if="currentTab === 'environment'" />
    <template-view v-if="currentTab === 'template'" />
  </PageContainer>
</template>
