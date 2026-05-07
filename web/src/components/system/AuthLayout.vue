<script lang="ts" setup>
interface Props {
  title?: string
  subtitle?: string
  logo?: string
  showFooter?: boolean
}

withDefaults(defineProps<Props>(), {
  title: undefined,
  subtitle: undefined,
  logo: undefined,
  showFooter: true,
})
</script>

<template>
  <div class="bg-bg-base flex h-full w-full">
    <aside
      class="p-3 bg-brand-subtle flex-col w-1/2 hidden justify-between relative overflow-hidden lg:flex"
    >
      <slot name="hero">
        <div class="relative z-10">
          <img v-if="logo" :src="logo" alt="logo" class="h-10" />
          <h2 class="text-3xl text-text-primary font-semibold mt-2">{{ title }}</h2>
          <p v-if="subtitle" class="text-base text-text-secondary mt-3">{{ subtitle }}</p>
        </div>
      </slot>
      <div class="text-xs text-text-tertiary relative z-10">
        <slot name="hero-footer" />
      </div>
    </aside>

    <main class="p-6 flex flex-col flex-1 items-center justify-center">
      <div class="max-w-md w-full">
        <slot />
      </div>
      <footer v-if="showFooter && $slots.footer" class="text-xs text-text-tertiary mt-6">
        <slot name="footer" />
      </footer>
    </main>
  </div>
</template>
