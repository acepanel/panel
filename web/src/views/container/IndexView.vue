<script setup lang="ts">
defineOptions({
  name: 'container-index',
})

import { useGettext } from 'vue3-gettext'

import ComposeView from '@/views/container/ComposeView.vue'
import ContainerView from '@/views/container/ContainerView.vue'
import ImageView from '@/views/container/ImageView.vue'
import NetworkView from '@/views/container/NetworkView.vue'
import VolumeView from '@/views/container/VolumeView.vue'

const { $gettext } = useGettext()
const current = ref('container')
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs v-model:value="current" animated>
        <n-tab name="container" :tab="$gettext('Containers')" />
        <n-tab name="compose" :tab="$gettext('Compose')" />
        <n-tab name="image" :tab="$gettext('Images')" />
        <n-tab name="network" :tab="$gettext('Networks')" />
        <n-tab name="volume" :tab="$gettext('Volumes')" />
      </n-tabs>
    </template>
    <container-view v-if="current === 'container'" />
    <compose-view v-else-if="current === 'compose'" />
    <image-view v-else-if="current === 'image'" />
    <network-view v-else-if="current === 'network'" />
    <volume-view v-else-if="current === 'volume'" />
  </PageContainer>
</template>
