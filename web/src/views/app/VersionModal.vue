<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import type { App } from '@/views/app/types'

import app from '../../api/panel/app'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const operation = defineModel<string>('operation', { type: String, required: true })
const info = defineModel<App>('info', { type: Object, required: true })

const doSubmit = ref(false)

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

const handleSubmit = () => {
  doSubmit.value = true
  useRequest(app.install(info.value.slug, model.value.channel))
    .onSuccess(() => {
      show.value = false
      model.value = {
        channel: null,
        version: '',
        log: '',
      }
      window.$message.success(
        $gettext('Task submitted, please check the progress in background tasks'),
      )
    })
    .onComplete(() => {
      doSubmit.value = false
    })
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
  <n-modal
    v-model:show="show"
    preset="card"
    :title="operation + ' ' + info.name"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
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
    <n-button
      type="info"
      block
      :loading="doSubmit"
      :disabled="model.channel == null || doSubmit"
      @click="handleSubmit"
    >
      {{ operation }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
