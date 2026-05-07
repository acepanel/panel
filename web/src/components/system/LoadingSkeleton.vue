<script lang="ts" setup>
interface Props {
  type?: 'list' | 'form' | 'card' | 'detail'
  rows?: number
  loading?: boolean
}

withDefaults(defineProps<Props>(), {
  type: 'card',
  rows: 4,
  loading: true,
})
</script>

<template>
  <div v-if="loading">
    <template v-if="type === 'list'">
      <n-skeleton :repeat="rows" height="44px" :sharp="false" class="mb-2" />
    </template>
    <template v-else-if="type === 'form'">
      <div class="flex flex-col gap-1">
        <div v-for="i in rows" :key="i" class="flex flex-col gap-2">
          <n-skeleton width="80px" height="14px" />
          <n-skeleton height="32px" :sharp="false" />
        </div>
      </div>
    </template>
    <template v-else-if="type === 'detail'">
      <div class="flex flex-col gap-1">
        <n-skeleton width="40%" height="22px" />
        <n-skeleton text :repeat="3" />
        <n-skeleton height="180px" :sharp="false" />
      </div>
    </template>
    <template v-else>
      <div class="flex flex-col gap-3">
        <n-skeleton width="60%" height="20px" />
        <n-skeleton text :repeat="rows" />
      </div>
    </template>
  </div>
  <slot v-else />
</template>
