<script setup lang="ts">
defineOptions({
  name: 'apps-nginx-index'
})

import nginx from '@/api/apps/nginx'
import NginxIndex from './NginxIndex.vue'
</script>

<template>
  <nginx-index :api="nginx" service="nginx" />
</template>
