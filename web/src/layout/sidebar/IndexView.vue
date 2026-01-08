<script lang="ts" setup>
import SideSetting from '@/layout/sidebar/components/SideSetting.vue'
import SideLogo from './components/SideLogo.vue'
import SideMenu from './components/SideMenu.vue'
</script>

<template>
  <div class="flex flex-col h-screen">
    <side-logo class="flex-shrink-0" />
    <side-menu class="flex-shrink-0 flex-grow-1" />
    <side-setting class="flex-shrink-0" />
  </div>
</template>
