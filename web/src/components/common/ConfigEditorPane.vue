<script setup lang="ts">
import type { Method } from 'alova'
import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  fetch: () => Method
  save: (content: string) => Method
  tip?: string
  lang?: string
  height?: string
}>()

const emit = defineEmits<{
  saved: []
}>()

const { $gettext } = useGettext()

const { data: content } = useRequest(props.fetch, {
  initialData: '',
})
const { loading, send } = useRequest((value: string) => props.save(value), {
  immediate: false,
}).onSuccess(() => {
  window.$message.success($gettext('Saved successfully'))
  emit('saved')
})
</script>

<template>
  <n-flex vertical>
    <n-alert v-if="tip" type="warning">
      {{ tip }}
    </n-alert>
    <common-editor v-model:value="content" :lang="lang" :height="height" />
    <n-flex>
      <n-button type="primary" :loading="loading" :disabled="loading" @click="send(content)">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>
