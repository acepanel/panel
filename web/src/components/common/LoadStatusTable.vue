<script setup lang="ts">
import type { Method } from 'alova'
import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  fetch: () => Method
}>()

const { $gettext } = useGettext()

const { data, loading } = useRequest(props.fetch, {
  initialData: [],
})

const columns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    ellipsis: { tooltip: true },
  },
]
</script>

<template>
  <n-data-table striped :loading="loading" :columns="columns" :data="data" />
</template>
