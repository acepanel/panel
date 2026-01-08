<script lang="ts" setup>
interface Props {
  showFooter?: boolean
}

withDefaults(defineProps<Props>(), {
  showFooter: false
})
</script>

<template>
  <transition appear mode="out-in" name="fade-slide">
    <section class="cus-scroll-y p-15 bg-[#f5f6fb] flex-col wh-full" dark:bg-hex-121212>
      <slot />
      <app-footer v-if="showFooter" mt-auto pt-20 />
    </section>
  </transition>
</template>
