<script setup lang="ts">
defineOptions({
  name: 'apps-mysql-index'
})

import mysql from '@/api/apps/mysql'
import MysqlIndex from './MysqlIndex.vue'
</script>

<template>
  <mysql-index :api="mysql" name="MySQL" />
</template>
