<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })

const mime = ref('')
const content = ref('')
const img = computed(() => {
  return `data:${mime.value};base64,${content.value}`
})

watch(
  () => path.value,
  () => {
    content.value = ''
    useRequest(file.content(encodeURIComponent(path.value))).onSuccess(({ data }) => {
      mime.value = data.mime
      content.value = data.content
    })
  },
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Preview - ') + path"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-image width="100%" :src="img" preview-disabled :show-toolbar="false">
      <template #placeholder>
        <n-spin />
      </template>
    </n-image>
  </n-modal>
</template>

<style scoped lang="scss"></style>
