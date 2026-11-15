<script lang="ts" setup>
interface Props {
  showFooter?: boolean
  flex?: boolean
  bare?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showFooter: false,
  flex: false,
  bare: false,
})

const slots = defineSlots<{
  tabs?: () => any
  default?: () => any
}>()

// 手机屏幕小，带页脚的页面让内容区至少撑满一屏，页脚滚动后才出现
// 用 flex-basis 而不是 min-height：显式 min-height 会取消 flex 子项按内容算的最小高度，内容会溢出卡片
const bodyClass = computed(() => {
  if (props.flex) return 'flex-1 min-h-0'
  return props.showFooter ? 'flex-1 lt-md:basis-full lt-md:shrink-0' : 'flex-1'
})
</script>

<template>
  <transition appear mode="out-in" name="fade-slide">
    <section class="cus-scroll-y page-container">
      <header v-if="slots.tabs">
        <n-card size="small">
          <slot name="tabs" />
        </n-card>
      </header>

      <div v-if="bare" class="flex flex-col gap-1" :class="bodyClass">
        <slot />
      </div>
      <n-card
        v-else
        :class="bodyClass"
        :content-class="flex ? 'flex flex-col min-h-0 h-full' : undefined"
      >
        <slot />
      </n-card>

      <!-- flex 页面内容区固定占满视口、没有页面滚动，手机上页脚无处可滚到，直接隐藏 -->
      <AppFooter v-if="showFooter" class="mt-auto pt-5" :class="{ 'lt-md:hidden': flex }" />
    </section>
  </transition>
</template>
