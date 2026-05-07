<script lang="ts" setup>
import logoImg from '@/assets/images/logo.svg'
import { useThemeStore } from '@/stores'

const themeStore = useThemeStore()
const router = useRouter()
const logo = computed(() => themeStore.logo || logoImg)

const toHome = () => {
  router.push({ name: 'home-index' })
}
</script>

<template>
  <div class="px-3 flex gap-2 h-15 cursor-pointer items-center justify-center" @click="toHome">
    <n-image :src="logo" preview-disabled class="flex-shrink-0 h-9" />
    <n-tooltip v-if="!themeStore.sider.collapsed" trigger="hover" :delay="500">
      <template #trigger>
        <h2 class="text-lg text-text-primary font-bold min-w-0 truncate">
          {{ themeStore.name }}
        </h2>
      </template>
      {{ themeStore.name }}
    </n-tooltip>
  </div>
</template>
