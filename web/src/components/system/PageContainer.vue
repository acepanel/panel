<script lang="ts" setup>
import AppFooter from '@/components/common/AppFooter.vue'

import LoadingSkeleton from './LoadingSkeleton.vue'
import PageHeader from './PageHeader.vue'

interface Props {
  title?: string
  description?: string
  showFooter?: boolean
  flex?: boolean
  loading?: boolean
  bare?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: undefined,
  description: undefined,
  showFooter: false,
  flex: false,
  loading: false,
  bare: false,
})

const slots = defineSlots<{
  header?: () => any
  tabs?: () => any
  actions?: () => any
  default?: () => any
  footer?: () => any
}>()

const hasHeader = computed(() => !!(props.title || slots.header || slots.actions))
</script>

<template>
  <transition appear mode="out-in" name="fade-slide">
    <section class="cus-scroll-y page-container">
      <header v-if="hasHeader || slots.tabs">
        <slot v-if="slots.header" name="header" />
        <PageHeader v-else-if="title" :title="title" :description="description">
          <template v-if="slots.actions" #actions>
            <slot name="actions" />
          </template>
        </PageHeader>
        <n-card v-if="slots.tabs" size="small">
          <slot name="tabs" />
        </n-card>
      </header>

      <div
        v-if="bare"
        :class="flex ? 'flex flex-col flex-1 min-h-0 gap-1' : 'flex flex-col flex-1 gap-1'"
      >
        <LoadingSkeleton v-if="loading" type="card" />
        <slot v-else />
      </div>
      <n-card
        v-else
        :class="flex ? 'flex-1 min-h-0' : 'flex-1'"
        :content-class="flex ? 'flex flex-col min-h-0 h-full' : undefined"
      >
        <LoadingSkeleton v-if="loading" type="card" />
        <slot v-else />
      </n-card>

      <slot v-if="slots.footer" name="footer" />
      <AppFooter v-else-if="showFooter" class="mt-auto pt-5" />
    </section>
  </transition>
</template>
