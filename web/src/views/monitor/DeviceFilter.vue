<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

// 磁盘、网卡的筛选：点名字切换显示，点“全部”一键全选或全不选，点名字旁的图标只看这一个
const props = defineProps<{
  options: string[]
}>()
const selected = defineModel<string[]>('value', { required: true })

const { $gettext } = useGettext()

const allSelected = computed(
  () => props.options.length > 0 && props.options.every((name) => selected.value.includes(name)),
)

const toggleAll = () => {
  selected.value = allSelected.value ? [] : [...props.options]
}

const toggle = (name: string) => {
  selected.value = selected.value.includes(name)
    ? selected.value.filter((item) => item !== name)
    : props.options.filter((item) => item === name || selected.value.includes(item))
}

const only = (name: string) => {
  selected.value = [name]
}

const chip = (active: boolean) =>
  active
    ? 'border-brand/40 bg-brand-subtle text-brand'
    : 'border-border-default text-text-tertiary hover:text-text-secondary'
</script>

<template>
  <div class="font-normal flex flex-wrap gap-1.5 items-center">
    <button
      type="button"
      class="text-xs px-2.5 border rounded-full h-6 transition-colors"
      :class="chip(allSelected)"
      @click="toggleAll"
    >
      {{ $gettext('All') }}
    </button>
    <div
      v-for="name in options"
      :key="name"
      class="group text-xs border rounded-full inline-flex h-6 transition-colors items-center"
      :class="chip(selected.includes(name))"
    >
      <button type="button" class="pl-2.5 pr-1 h-full max-w-48 truncate" @click="toggle(name)">
        {{ name }}
      </button>
      <button
        type="button"
        class="pl-0.5 pr-2 opacity-0 flex h-full items-center focus-visible:opacity-100 group-hover:opacity-100 lt-md:opacity-60"
        :title="$gettext('Show only this')"
        :aria-label="$gettext('Show only this')"
        @click="only(name)"
      >
        <the-icon icon="mdi:target" :size="12" />
      </button>
    </div>
  </div>
</template>
