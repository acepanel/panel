<script setup lang="ts">
defineOptions({
  name: 'apps-valkey-index',
})

import valkey from '@/api/apps/valkey'
import RedisIndex from '@/views/apps/redis/RedisIndex.vue'
</script>

<template>
  <redis-index :api="valkey" name="Valkey" service="valkey" />
</template>
