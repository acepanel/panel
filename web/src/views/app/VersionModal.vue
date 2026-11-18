<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import CustomForm from '@/views/app/CustomForm.vue'
import type { App } from '@/views/app/types'

import app from '../../api/panel/app'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const operation = defineModel<string>('operation', { type: String, required: true })
const info = defineModel<App>('info', { type: Object, required: true })

const customFormRef = ref<{ save: () => Promise<any>; loading: boolean } | null>(null)

const model = ref({
  channel: null,
  version: '',
  log: '',
})

const options = computed(() => {
  return info.value.channels.map((channel) => {
    return {
      label: channel.name,
      value: channel.slug,
    }
  })
})

const handleSubmit = async () => {
  try {
    // 编译型应用先保存自定义参数再触发安装
    if (customFormRef.value) {
      await customFormRef.value.save()
    }
  } catch {
    return
  }
  await app.install(info.value.slug, model.value.channel)
  show.value = false
  model.value = {
    channel: null,
    version: '',
    log: '',
  }
  window.$message.success($gettext('Task submitted, please check the progress in background tasks'))
}

const handleChannelUpdate = (value: string) => {
  const channel = info.value.channels.find((channel) => channel.slug === value)
  if (channel) {
    model.value.version = channel.version
    model.value.log = channel.log
  }
}

const handleClose = () => {
  show.value = false
  model.value = {
    channel: null,
    version: '',
    log: '',
  }
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="operation + ' ' + info.name"
    @close="handleClose"
    @mask-click="handleClose"
  >
    <n-form :model="model">
      <n-form-item path="channel" :label="$gettext('Channel')">
        <n-select
          v-model:value="model.channel"
          :options="options"
          @update-value="handleChannelUpdate"
        />
      </n-form-item>
      <n-form-item path="channel" :label="$gettext('Version')">
        <n-input
          v-model:value="model.version"
          :placeholder="$gettext('Please select a channel')"
          readonly
          disabled
        />
      </n-form-item>
      <n-form-item v-if="model.log" path="log" :label="$gettext('Release Log')">
        <n-card embedded :bordered="false" content-style="padding: 12px;">
          <n-scrollbar style="max-height: 200px">
            <pre style="margin: 0; white-space: pre-wrap; word-break: break-word">{{
              model.log
            }}</pre>
          </n-scrollbar>
        </n-card>
      </n-form-item>
    </n-form>
    <n-collapse v-if="info.custom_supported" class="mb-16px">
      <n-collapse-item :title="$gettext('Custom Compile Params')" name="custom">
        <custom-form ref="customFormRef" :slug="info.slug" />
      </n-collapse-item>
    </n-collapse>
    <app-button
      type="info"
      block
      :disabled="model.channel == null || customFormRef?.loading"
      @click="handleSubmit"
    >
      {{ operation }}
    </app-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
