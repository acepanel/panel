<script setup lang="ts">
import type { TagProps } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import task from '@/api/panel/task'
import { useConfirm } from '@/components/system/composables/useConfirm'
import type { QueueTask, QueueTaskStatus } from '@/stores'
import { useThemeStore } from '@/stores'

const props = defineProps<{
  tasks: QueueTask[]
  title: string
  emptyText?: string
}>()

const show = defineModel<boolean>('show', { default: false })
const minimized = defineModel<boolean>('minimized', { default: false })

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const themeStore = useThemeStore()

const statusMeta = computed<Record<QueueTaskStatus, { label: string; type: TagProps['type'] }>>(
  () => ({
    waiting: { label: $gettext('Waiting'), type: 'default' },
    running: { label: $gettext('Running'), type: 'info' },
    finished: { label: $gettext('Completed'), type: 'success' },
    failed: { label: $gettext('Failed'), type: 'error' },
    canceled: { label: $gettext('Canceled'), type: 'warning' },
  }),
)

const isActive = (item: QueueTask) => item.status === 'waiting' || item.status === 'running'

const collapsed = ref(true)

// 没手动选过就跟随正在执行的任务
const selectedId = ref<number>()
const current = computed(
  () =>
    props.tasks.find((t) => t.id === selectedId.value) ??
    props.tasks.find((t) => t.status === 'running') ??
    props.tasks[0],
)

// 手机上展开的列表会挤掉大半日志区，选中后收起
const handleSelect = (id: number) => {
  selectedId.value = id
  if (themeStore.isMobile) collapsed.value = true
}

const handleCancel = async (item: QueueTask) => {
  const ok = await confirmAction({
    title: $gettext('Cancel Task'),
    content: $gettext('Are you sure you want to cancel task %{ name }?', { name: item.name }),
  })
  if (!ok) return
  await task.cancel(item.id)
  window.$message.success($gettext('Canceled successfully'))
}
</script>

<template>
  <DraggableWindow
    v-model:show="show"
    v-model:minimized="minimized"
    :title="title"
    :default-width="960"
    :default-height="560"
    :min-width="640"
    :min-height="360"
  >
    <template #icon>
      <i-mdi-sync v-if="tasks.some(isActive)" class="animate-spin" />
      <i-mdi-alert-circle-outline v-else-if="tasks.length" class="text-error" />
      <i-mdi-inbox-multiple-outline v-else />
    </template>
    <n-layout has-sider class="h-full !bg-transparent">
      <n-layout-sider
        v-if="tasks.length"
        v-model:collapsed="collapsed"
        :width="200"
        :collapsed-width="16"
        collapse-mode="width"
        show-trigger="arrow-circle"
        bordered
        :native-scrollbar="false"
        content-class="p-2"
        class="!bg-transparent"
      >
        <n-flex v-show="!collapsed" vertical :size="4">
          <div
            v-for="item in tasks"
            :key="item.id"
            class="hover-bg px-3 py-2 rounded cursor-pointer"
            :class="{ 'bg-bg-subtle': item.id === current?.id }"
            @click="handleSelect(item.id)"
          >
            <n-ellipsis class="w-full">{{ item.name }}</n-ellipsis>
            <n-flex align="center" justify="space-between" class="mt-1">
              <n-tag size="small" :bordered="false" :type="statusMeta[item.status].type">
                {{ statusMeta[item.status].label }}
              </n-tag>
              <app-button
                v-if="isActive(item)"
                size="tiny"
                quaternary
                type="error"
                @click.stop="handleCancel(item)"
              >
                {{ $gettext('Cancel') }}
              </app-button>
            </n-flex>
          </div>
        </n-flex>
      </n-layout-sider>
      <n-layout-content class="!bg-transparent" content-class="h-full p-3 flex">
        <realtime-log
          v-if="current?.log"
          :path="current.log"
          class="flex-1 min-w-0 !h-auto !min-h-0"
        />
        <n-empty
          v-else
          :description="current ? $gettext('Waiting') : emptyText"
          class="flex-1 justify-center"
        />
      </n-layout-content>
    </n-layout>
  </DraggableWindow>
</template>
