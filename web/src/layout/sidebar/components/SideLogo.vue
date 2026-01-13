<script lang="ts" setup>
import logoImg from '@/assets/images/logo.png'
import { useThemeStore } from '@/store'

const themeStore = useThemeStore()
const router = useRouter()
const logo = computed(() => themeStore.logo || logoImg)

const toHome = () => {
  router.push({ name: 'home-index' })
}
</script>

<template>
  <div class="f-c-c h-60 cursor-pointer" @click="toHome">
    <n-image :src="logo" preview-disabled class="h-36" />
    <h2
      v-show="!themeStore.sider.collapsed"
      class="text-18 font-bold ml-10 flex-shrink-0 max-w-140"
    >
      {{ themeStore.name }}
    </h2>
  </div>
</template>
