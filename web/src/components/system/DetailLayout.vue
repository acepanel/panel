<script lang="ts" setup>
import PageContainer from './PageContainer.vue'
import PageHeader from './PageHeader.vue'

interface TabItem {
  key: string
  label: string
  icon?: string
  disabled?: boolean
}

interface Props {
  title?: string
  description?: string
  back?: boolean | (() => void)
  tabs?: TabItem[]
  modelValue?: string
  tabsPlacement?: 'top' | 'left' | 'auto'
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: undefined,
  description: undefined,
  back: false,
  tabs: () => [],
  modelValue: undefined,
  tabsPlacement: 'auto',
  loading: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
}>()

const placement = computed<'top' | 'left'>(() => {
  if (props.tabsPlacement !== 'auto') return props.tabsPlacement
  return props.tabs.length > 5 ? 'left' : 'top'
})

const currentTab = computed({
  get: () => props.modelValue ?? props.tabs[0]?.key,
  set: (v: string) => emit('update:modelValue', v),
})
</script>

<template>
  <PageContainer :loading="loading" bare flex>
    <template v-if="title || $slots.actions" #header>
      <PageHeader :title="title ?? ''" :description="description" :back="back">
        <template v-if="$slots.actions" #actions>
          <slot name="actions" />
        </template>
        <template v-if="$slots.extra" #extra>
          <slot name="extra" />
        </template>
      </PageHeader>
    </template>

    <n-tabs
      v-if="tabs.length"
      v-model:value="currentTab"
      :placement="placement"
      animated
      type="line"
      :tab-style="placement === 'left' ? { padding: '6px 12px' } : undefined"
      class="flex-1 min-h-0"
    >
      <n-tab-pane
        v-for="tab in tabs"
        :key="tab.key"
        :name="tab.key"
        :tab="tab.label"
        :disabled="tab.disabled"
        display-directive="show:lazy"
      >
        <slot :name="tab.key" :tab="tab" />
      </n-tab-pane>
    </n-tabs>
    <slot v-else />
  </PageContainer>
</template>
