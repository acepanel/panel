<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import task from '@/api/panel/task'
import { type QueueTask, useTaskQueueStore } from '@/stores'

const { $gettext, $ngettext } = useGettext()
const queue = useTaskQueueStore()

const title = computed(() => `${$gettext('Task Queue')} (${queue.tasks.length})`)

// 全部结束时队列里只剩失败的，留着看日志，否则关闭窗口
const settle = () => {
  if (queue.tasks.length) {
    const n = queue.tasks.length
    window.$message.error($ngettext('%{ n } task failed', '%{ n } tasks failed', n, { n }))
    return
  }
  const done = queue.finished
  queue.$reset()
  if (done) window.$message.success($gettext('All tasks completed'))
}

const poll = async () => {
  const ids = queue.active.map((t) => t.id)
  if (!ids.length) return
  const fresh: QueueTask[] | null = await task.query(ids).catch(() => null)
  if (!fresh) return
  queue.sync(ids, fresh)
  // 有任务结束就刷新，结果文件随之出现
  if (queue.active.length < ids.length) window.$bus.emit('file:refresh')
  if (!queue.active.length) settle()
}

const { pause, resume } = useTimeoutPoll(poll, 1000, { immediate: false, immediateCallback: true })
watch(
  () => queue.active.length > 0,
  (busy) => (busy ? resume() : pause()),
  { immediate: true },
)

// 只剩失败任务时关闭窗口就清空队列，还有任务在跑则只是隐藏
watch(
  () => queue.show,
  (show) => {
    if (!show && !queue.active.length) queue.$reset()
  },
)
</script>

<template>
  <task-window
    v-model:show="queue.show"
    v-model:minimized="queue.minimized"
    :tasks="queue.tasks"
    :title="title"
  />
</template>
