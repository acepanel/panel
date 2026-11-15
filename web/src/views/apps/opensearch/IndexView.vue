<script setup lang="ts">
defineOptions({
  name: 'apps-opensearch-index',
})

import opensearch from '@/api/apps/opensearch'
import ElasticSearchIndex from '@/views/apps/elasticsearch/ElasticSearchIndex.vue'
</script>

<template>
  <elastic-search-index :api="opensearch" name="OpenSearch" service="opensearch" />
</template>
