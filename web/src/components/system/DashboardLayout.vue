<script lang="ts" setup>
import PageContainer from './PageContainer.vue'
import PageHeader from './PageHeader.vue'

interface Props {
  title?: string
  description?: string
  maxWidth?: number | string
  cols?: number
  gap?: number
}

const props = withDefaults(defineProps<Props>(), {
  title: undefined,
  description: undefined,
  maxWidth: 1440,
  cols: 12,
  gap: 16,
})

const containerStyle = computed(() => ({
  maxWidth: typeof props.maxWidth === 'number' ? `${props.maxWidth}px` : props.maxWidth,
  margin: '0 auto',
}))
</script>

<template>
  <PageContainer bare>
    <template v-if="title || $slots.actions" #header>
      <PageHeader :title="title ?? ''" :description="description">
        <template v-if="$slots.actions" #actions>
          <slot name="actions" />
        </template>
      </PageHeader>
    </template>
    <div class="w-full" :style="containerStyle">
      <n-grid :cols="cols" :x-gap="gap" :y-gap="gap" responsive="screen" item-responsive>
        <slot />
      </n-grid>
    </div>
  </PageContainer>
</template>
