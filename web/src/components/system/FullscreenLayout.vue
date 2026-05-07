<script lang="ts" setup>
interface Props {
  title?: string
  toolbar?: boolean
  padding?: boolean
}

withDefaults(defineProps<Props>(), {
  title: undefined,
  toolbar: true,
  padding: false,
})
</script>

<template>
  <div class="bg-bg-base flex flex-col h-full">
    <header
      v-if="toolbar && (title || $slots.toolbar)"
      class="px-4 py-3 border-b border-border-subtle bg-bg-elevated flex flex-shrink-0 gap-3 items-center"
    >
      <h1 v-if="title" class="text-base text-text-primary font-semibold">{{ title }}</h1>
      <slot name="toolbar" />
    </header>
    <main class="flex flex-col flex-1 min-h-0" :class="{ 'p-1': padding }">
      <slot />
    </main>
  </div>
</template>
