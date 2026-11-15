<script setup lang="ts">
import type { ProxyWebsiteForm } from './composables/useProxyWebsite'

const form = defineModel<ProxyWebsiteForm>('value', { required: true })
</script>

<template>
  <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
    <n-form-item-gi :span="16" :label="$gettext('Domain')">
      <ListInput v-model:value="form.domains" placeholder="example.com" :min="1" show-sort-button />
    </n-form-item-gi>
    <n-form-item-gi :span="8" :label="$gettext('Project Port')">
      <n-input-number
        v-model:value="form.port"
        :min="1"
        :max="65535"
        class="w-full"
        :placeholder="$gettext('e.g., 3000')"
      />
    </n-form-item-gi>
  </n-grid>
</template>
