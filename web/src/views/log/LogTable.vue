<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import log from '@/api/panel/log'

import CleanPopover from './CleanPopover.vue'

const props = defineProps<{
  type: 'app' | 'db' | 'http'
  columns: DataTableColumns<any>
}>()

const { $gettext } = useGettext()

const limit = ref(200)
const selectedDate = ref('')
const selectedLevel = ref('')
const keyword = ref('')

const levelOptions = [
  { label: $gettext('All'), value: '' },
  { label: 'DEBUG', value: 'DEBUG' },
  { label: 'INFO', value: 'INFO' },
  { label: 'WARN', value: 'WARN' },
  { label: 'ERROR', value: 'ERROR' },
]

const { data: dates, send: refreshDates } = useRequest(() => log.dates(props.type), {
  initialData: [],
})

const dateOptions = computed(() => [
  { label: $gettext('Today'), value: '' },
  ...(dates.value ?? []).map((date: string) => ({ label: date, value: date })),
])

const {
  loading,
  data,
  send: refresh,
} = useRequest(
  () => log.list(props.type, limit.value, selectedDate.value, selectedLevel.value, keyword.value),
  {
    initialData: [],
  },
)

// 清理后选中的日期可能已被删掉，回到今天
const handleCleaned = () => {
  selectedDate.value = ''
  refreshDates()
  refresh()
}
</script>

<template>
  <n-flex vertical class="h-full">
    <n-flex align="center">
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Date') }}:</span>
        <n-select
          v-model:value="selectedDate"
          :options="dateOptions"
          class="w-37"
          @update:value="refresh()"
        />
      </div>
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Level') }}:</span>
        <n-select
          v-model:value="selectedLevel"
          :options="levelOptions"
          class="w-25"
          @update:value="refresh()"
        />
      </div>
      <div class="flex gap-3 items-center">
        <span>{{ $gettext('Show entries') }}:</span>
        <n-select
          v-model:value="limit"
          :options="[
            { label: '100', value: 100 },
            { label: '200', value: 200 },
            { label: '500', value: 500 },
            { label: '1000', value: 1000 },
          ]"
          class="w-25"
          @update:value="refresh()"
        />
      </div>
      <n-input
        v-model:value="keyword"
        :placeholder="$gettext('Search')"
        clearable
        class="md:!w-60"
        @keyup.enter="refresh()"
        @clear="refresh()"
      />
      <n-button type="primary" @click="refresh()">
        {{ $gettext('Refresh') }}
      </n-button>
      <clean-popover :type="type" @cleaned="handleCleaned" />
    </n-flex>
    <n-data-table
      class="flex-1 min-h-0"
      :columns="columns"
      :data="data"
      :loading="loading"
      :bordered="false"
      flex-height
      :scroll-x="800"
      virtual-scroll
    />
  </n-flex>
</template>
