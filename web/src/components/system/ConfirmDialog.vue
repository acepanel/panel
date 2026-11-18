<script lang="ts" setup>
import type { PopoverPlacement } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { isThenable } from '@/utils'

interface Props {
  type?: 'normal' | 'danger' | 'delete'
  countdown?: number
  title?: string
  content?: string
  positiveText?: string
  negativeText?: string
  placement?: PopoverPlacement
  trigger?: 'click' | 'hover' | 'manual'
  showIcon?: boolean
  width?: number | string
  // 声明成属性而不是事件，才能拿到处理函数返回的 Promise
  onConfirm?: () => unknown
}

const props = withDefaults(defineProps<Props>(), {
  type: 'normal',
  countdown: undefined,
  title: undefined,
  content: '',
  positiveText: undefined,
  negativeText: undefined,
  placement: 'top',
  trigger: 'click',
  showIcon: false,
  width: 280,
  onConfirm: undefined,
})

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const { $gettext } = useGettext()

const totalCountdown = computed(() => {
  if (typeof props.countdown === 'number') return props.countdown
  return 0
})

const remain = ref(0)
let timer: ReturnType<typeof setInterval> | null = null
const isDisabled = computed(() => remain.value > 0)

const positiveButtonType = computed(() => {
  if (props.type === 'delete' || props.type === 'danger') return 'error'
  return 'primary'
})

const computedPositiveText = computed(() => {
  const base =
    props.positiveText ?? (props.type === 'delete' ? $gettext('Delete') : $gettext('Confirm'))
  return remain.value > 0 ? `${base} (${remain.value}s)` : base
})

const computedNegativeText = computed(() => props.negativeText ?? $gettext('Cancel'))

const stop = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

const start = () => {
  stop()
  remain.value = totalCountdown.value
  if (totalCountdown.value <= 0) return
  timer = setInterval(() => {
    remain.value -= 1
    if (remain.value <= 0) stop()
  }, 1000)
}

const onUpdateShow = (show: boolean) => {
  if (show) start()
  else {
    stop()
    remain.value = 0
  }
}

// 确认后的操作未结束时，作为触发器的 AppButton 保持加载态
const pending = ref(false)
provide('confirmPending', pending)

// 不把 Promise 返回给 n-popconfirm，否则气泡会等操作结束才关闭
const onPositive = () => {
  if (isDisabled.value) return false
  const result = props.onConfirm?.()
  if (!isThenable(result)) return
  // 推迟到下一个宏任务，已经结束的 Promise 不会让触发按钮闪一下
  const timer = setTimeout(() => {
    pending.value = true
  })
  void Promise.resolve(result).finally(() => {
    clearTimeout(timer)
    pending.value = false
  })
}

const onNegative = () => {
  emit('cancel')
}

onUnmounted(stop)
</script>

<template>
  <n-popconfirm
    :show-icon="showIcon"
    :placement="placement"
    :trigger="trigger"
    :positive-button-props="{ type: positiveButtonType, disabled: isDisabled }"
    :positive-text="computedPositiveText"
    :negative-text="computedNegativeText"
    @update:show="onUpdateShow"
    @positive-click="onPositive"
    @negative-click="onNegative"
  >
    <template #default>
      <div :style="{ maxWidth: typeof width === 'number' ? `${width}px` : width }">
        <p v-if="title" class="text-text-primary font-medium mb-1">{{ title }}</p>
        <p class="text-sm text-text-secondary">
          <slot>{{ content }}</slot>
        </p>
      </div>
    </template>
    <template #trigger>
      <slot name="trigger" />
    </template>
  </n-popconfirm>
</template>
