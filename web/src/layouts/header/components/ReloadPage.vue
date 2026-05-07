<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import { useTabStore } from '@/stores'

const { $gettext } = useGettext()
const tabStore = useTabStore()

const handleReloadPage = () => {
  tabStore.reloadTab(tabStore.active)
}
</script>

<template>
  <n-tooltip trigger="hover">
    <template #trigger>
      <n-icon mr-5 cursor-pointer size="20" @click="handleReloadPage">
        <i-mdi-refresh />
      </n-icon>
    </template>
    {{ $gettext('Refresh Tab') }}
  </n-tooltip>
</template>
