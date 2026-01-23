<script setup lang="ts">
import { NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const props = defineProps({
  // 确认按钮文本
  positiveText: {
    type: String,
    default: ''
  },
  // 取消按钮文本
  negativeText: {
    type: String,
    default: ''
  },
  // 是否显示图标
  showIcon: {
    type: Boolean,
    default: false
  },
  // 倒计时秒数
  countdown: {
    type: Number,
    default: 5
  }
})

const emit = defineEmits<{
  (e: 'positiveClick'): void
  (e: 'negativeClick'): void
}>()

// 倒计时计数器
const countdownValue = ref(0)
// 定时器 ID
let timer: ReturnType<typeof setInterval> | null = null

// 计算按钮禁用状态
const isDisabled = computed(() => countdownValue.value > 0)

// 计算确认按钮文本
const computedPositiveText = computed(() => {
  const text = props.positiveText || $gettext('Confirm')
  if (countdownValue.value > 0) {
    return `${text} (${countdownValue.value}s)`
  }
  return text
})

// 计算取消按钮文本
const computedNegativeText = computed(() => {
  return props.negativeText || $gettext('Cancel')
})

// 开始倒计时
const startCountdown = () => {
  countdownValue.value = props.countdown
  timer = setInterval(() => {
    countdownValue.value--
    if (countdownValue.value <= 0) {
      stopCountdown()
    }
  }, 1000)
}

// 停止倒计时
const stopCountdown = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

// 重置倒计时
const resetCountdown = () => {
  stopCountdown()
  countdownValue.value = 0
}

// 处理显示状态变化
const handleUpdateShow = (show: boolean) => {
  if (show) {
    startCountdown()
  } else {
    resetCountdown()
  }
}

// 处理确认点击
const handlePositiveClick = () => {
  if (!isDisabled.value) {
    emit('positiveClick')
  }
  return !isDisabled.value
}

// 组件卸载时清理定时器
onUnmounted(() => {
  stopCountdown()
})
</script>

<template>
  <n-popconfirm
    :show-icon="showIcon"
    :positive-button-props="{ disabled: isDisabled }"
    :positive-text="computedPositiveText"
    :negative-text="computedNegativeText"
    @update:show="handleUpdateShow"
    @positive-click="handlePositiveClick"
    @negative-click="emit('negativeClick')"
  >
    <template #default>
      <slot></slot>
    </template>
    <template #trigger>
      <slot name="trigger"></slot>
    </template>
  </n-popconfirm>
</template>
