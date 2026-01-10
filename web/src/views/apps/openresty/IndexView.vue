<script setup lang="ts">
defineOptions({
  name: 'apps-openresty-index'
})

import openresty from '@/api/apps/openresty'
import NginxIndex from '@/views/apps/nginx/NginxIndex.vue'
</script>

<template>
  <nginx-index :api="openresty" service="openresty" />
</template>
