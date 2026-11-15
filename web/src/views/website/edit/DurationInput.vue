<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

type Unit = 's' | 'm' | 'h'

// Go time.Duration 在 JSON 中以纳秒表示
const SECOND = 1000000000
const factors: Record<Unit, number> = { s: SECOND, m: 60 * SECOND, h: 3600 * SECOND }

const value = defineModel<number>('value', { required: true })
defineProps<{
  min?: number
  max?: number
  placeholder?: string
}>()

const { $gettext } = useGettext()

const unitOptions = [
  { label: $gettext('Seconds'), value: 's' },
  { label: $gettext('Minutes'), value: 'm' },
  { label: $gettext('Hours'), value: 'h' },
]

// 初始单位取能整除的最大单位，之后切换单位只改显示，不改纳秒值
const inferUnit = (ns: number): Unit => {
  if (!(ns > 0)) return 's'
  if (ns % factors.h === 0) return 'h'
  if (ns % factors.m === 0) return 'm'
  return 's'
}
const unit = ref(inferUnit(value.value))

const display = computed(() => (value.value > 0 ? value.value / factors[unit.value] : null))

const update = (v: number | null) => {
  value.value = v && v > 0 ? v * factors[unit.value] : 0
}
</script>

<template>
  <n-input-group>
    <n-input-number
      :value="display"
      :min="min"
      :max="max"
      :placeholder="placeholder"
      flex-1
      @update:value="update"
    />
    <n-select v-model:value="unit" :options="unitOptions" class="w-25" />
  </n-input-group>
</template>
