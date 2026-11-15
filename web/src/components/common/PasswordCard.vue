<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { Method } from 'alova'
import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  title: string
  tip: string
  fetch: () => Method
  save: (password: string) => Method
}>()

const { $gettext } = useGettext()

const { data: password } = useRequest(props.fetch, {
  initialData: '',
})
const { loading, send } = useRequest((value: string) => props.save(value), {
  immediate: false,
}).onSuccess(() => {
  window.$message.success($gettext('Modified successfully'))
})

const handleCopy = () => {
  copy2clipboard(password.value).then(() => {
    window.$message.success($gettext('Copied successfully'))
  })
}
</script>

<template>
  <n-card :title="title">
    <n-flex vertical>
      <n-alert type="info">
        {{ tip }}
      </n-alert>
      <n-flex>
        <n-input-group>
          <n-input v-model:value="password" type="password" show-password-on="click" />
          <n-button type="primary" ghost @click="handleCopy">
            {{ $gettext('Copy') }}
          </n-button>
        </n-input-group>
        <n-button type="primary" :loading="loading" :disabled="loading" @click="send(password)">
          {{ $gettext('Save') }}
        </n-button>
      </n-flex>
    </n-flex>
  </n-card>
</template>
