<script setup lang="ts">
defineOptions({
  name: 'apps-percona-index'
})

import percona from '@/api/apps/percona'
import MysqlIndex from '@/views/apps/mysql/MysqlIndex.vue'
</script>

<template>
  <mysql-index :api="percona" name="Percona" />
</template>
