<script setup lang="ts">
defineOptions({
  name: 'apps-elasticsearch-index',
})

import elasticsearch from '@/api/apps/elasticsearch'

import ElasticSearchIndex from './ElasticSearchIndex.vue'
</script>

<template>
  <elastic-search-index :api="elasticsearch" name="ElasticSearch" service="elasticsearch" />
</template>
