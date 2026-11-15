<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import task from '@/api/panel/task'
import type { QueueTask } from '@/stores'

const { $gettext } = useGettext()

const { data } = useAutoRequest(() => task.status(), {
  initialData: { task: false, tasks: [] as QueueTask[] },
  pollingTime: 3000,
})

const show = ref(false)
const minimized = ref(false)
const title = computed(() => `${$gettext('Panel Tasks')} (${data.value.tasks.length})`)

const open = () => {
  show.value = true
  minimized.value = false
}
</script>

<template>
  <n-tooltip trigger="hover">
    <template #trigger>
      <n-icon cursor-pointer size="20" @click="open">
        <i-mdi-sync v-if="data.task" class="animate-spin" />
        <i-mdi-inbox-multiple-outline v-else />
      </n-icon>
    </template>
    {{ data.task ? $gettext('Tasks Running') : $gettext('Panel Tasks') }}
  </n-tooltip>
  <task-window
    v-model:show="show"
    v-model:minimized="minimized"
    :tasks="data.tasks"
    :title="title"
    :empty-text="$gettext('No running tasks')"
  />
</template>
