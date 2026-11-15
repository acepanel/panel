<script setup lang="ts">
defineOptions({
  name: 'apps-redis-index',
})

import redis from '@/api/apps/redis'

import RedisIndex from './RedisIndex.vue'
</script>

<template>
  <redis-index :api="redis" name="Redis" service="redis" />
</template>
