<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { formatBytes } from '@/utils/file'

const { $gettext } = useGettext()

defineProps<{
  progress: Map<string, any>
  status: string
  error: string
  percentage: number
}>()

defineEmits<{
  cancel: []
  retry: []
}>()
</script>

<template>
  <n-result v-if="error" status="error" :title="$gettext('Pull Failed')" :description="error">
    <template #footer>
      <n-flex justify="center">
        <n-button @click="$emit('cancel')">{{ $gettext('Cancel') }}</n-button>
        <n-button type="primary" @click="$emit('retry')">{{ $gettext('Retry') }}</n-button>
      </n-flex>
    </template>
  </n-result>

  <n-flex v-else vertical :size="16">
    <n-progress type="line" :percentage="percentage" :indicator-placement="'inside'" processing />

    <n-card size="small" :bordered="true" class="max-h-75 overflow-y-auto">
      <n-flex vertical :size="8">
        <div v-for="[id, layer] in progress" :key="id" class="p-1 px-2 rounded bg-bg-subtle">
          <n-flex justify="space-between" align="center">
            <n-text depth="3" class="text-xs font-mono">
              {{ id.substring(0, 12) }}
            </n-text>
            <n-text depth="2" class="text-xs">
              {{ layer.status }}
              <template v-if="layer.progressDetail?.total">
                {{ formatBytes(layer.progressDetail.current) }} /
                {{ formatBytes(layer.progressDetail.total) }}
              </template>
            </n-text>
          </n-flex>
        </div>
        <n-text v-if="progress.size === 0" depth="3">
          {{ status }}
        </n-text>
      </n-flex>
    </n-card>

    <n-flex justify="center">
      <n-button type="error" ghost @click="$emit('cancel')">
        {{ $gettext('Cancel') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>
