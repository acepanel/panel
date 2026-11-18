<script setup lang="ts">
import type { Ref } from 'vue'

import { isThenable } from '@/utils'

defineOptions({ inheritAttrs: false })

type ClickHandler = (e: MouseEvent) => unknown

const props = defineProps<{
  loading?: boolean
  // 作为弹层触发器时，弹层会把自己的监听合并进来，成为数组
  onClick?: ClickHandler | ClickHandler[]
}>()

// 作为 ConfirmDialog 的触发按钮时，跟随确认后的操作进入加载态
const confirming = inject<Ref<boolean>>('confirmPending', ref(false))
const pending = ref(false)
const busy = computed(() => props.loading || pending.value || confirming.value)

// naive 在加载中不调用 onClick，调用方写的 .stop 随之失效，这里统一不让点击冒泡到可点击的行
const handleClickCapture = (e: MouseEvent) => {
  if (busy.value) e.stopPropagation()
}

// 加载态推迟到下一个宏任务：同步的处理函数、async 函数里直接返回的校验分支都不会让加载图标闪一下
let running = false
const handleClick = async (e: MouseEvent) => {
  if (running) return
  const results = [props.onClick].flat().map((handler) => handler?.(e))
  if (!results.some(isThenable)) return
  running = true
  const timer = setTimeout(() => {
    pending.value = true
  })
  try {
    await Promise.all(results)
  } finally {
    clearTimeout(timer)
    running = false
    pending.value = false
  }
}
</script>

<template>
  <n-button
    v-bind="$attrs"
    :loading="busy"
    @click="handleClick"
    @click.capture="handleClickCapture"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </n-button>
</template>
