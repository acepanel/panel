<script setup lang="ts">
defineOptions({
  name: 'apps-mariadb-index'
})

import mariadb from '@/api/apps/mariadb'
import MysqlIndex from '@/views/apps/mysql/MysqlIndex.vue'
</script>

<template>
  <mysql-index :api="mariadb" name="MariaDB" />
</template>
