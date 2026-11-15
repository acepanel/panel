<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import systemctl from '@/api/panel/systemctl'

const { $gettext } = useGettext()
const props = defineProps({
  service: {
    type: String,
    required: true,
  },
  title: {
    type: String,
    required: false,
  },
  showReload: {
    type: Boolean,
    required: false,
  },
})

const fetchingStatus = ref(true)
const fetchingIsEnabled = ref(true)
const status = ref(false)
const isEnabled = ref(false)

const statusStr = computed(() => {
  if (fetchingStatus.value) return $gettext('Loading...')
  return status.value ? $gettext('Running') : $gettext('Stopped')
})

const fetchStatus = async () => {
  fetchingStatus.value = true
  try {
    status.value = await systemctl.status(props.service)
  } finally {
    fetchingStatus.value = false
  }
}

const fetchIsEnabled = async () => {
  fetchingIsEnabled.value = true
  try {
    isEnabled.value = await systemctl.isEnabled(props.service)
  } finally {
    fetchingIsEnabled.value = false
  }
}

const act = (method: (service: string) => any, loadingText: string, okText: string) => {
  const messageReactive = window.$message.loading(loadingText, {
    duration: 0,
  })
  fetchingStatus.value = true
  useRequest(method(props.service))
    .onSuccess(() => {
      window.$message.success(okText)
    })
    .onComplete(() => {
      messageReactive?.destroy()
      fetchStatus()
    })
}

const handleStart = () =>
  act(systemctl.start, $gettext('Starting...'), $gettext('Started successfully'))
const handleStop = () =>
  act(systemctl.stop, $gettext('Stopping...'), $gettext('Stopped successfully'))
const handleRestart = () =>
  act(systemctl.restart, $gettext('Restarting...'), $gettext('Restarted successfully'))
const handleReload = () =>
  act(systemctl.reload, $gettext('Reloading...'), $gettext('Reloaded successfully'))

const handleIsEnabled = async () => {
  const messageReactive = window.$message.loading($gettext('Setting autostart...'), {
    duration: 0,
  })
  fetchingIsEnabled.value = true
  if (isEnabled.value) {
    useRequest(systemctl.enable(props.service))
      .onSuccess(() => {
        window.$message.success($gettext('Autostart enabled successfully'))
      })
      .onComplete(() => {
        messageReactive?.destroy()
        fetchIsEnabled()
      })
  } else {
    useRequest(systemctl.disable(props.service))
      .onSuccess(() => {
        window.$message.success($gettext('Autostart disabled successfully'))
      })
      .onComplete(() => {
        messageReactive?.destroy()
        fetchIsEnabled()
      })
  }
}

onMounted(() => {
  fetchStatus()
  fetchIsEnabled()
})
</script>

<template>
  <n-card :title="props.title ?? $gettext('Running Status')">
    <template #header-extra>
      <n-switch
        :loading="fetchingIsEnabled"
        v-model:value="isEnabled"
        @update:value="handleIsEnabled"
      >
        <template #checked> {{ $gettext('Autostart On') }} </template>
        <template #unchecked> {{ $gettext('Autostart Off') }} </template>
      </n-switch>
    </template>
    <n-flex vertical>
      <n-alert :type="fetchingStatus ? 'info' : status ? 'success' : 'error'">
        {{ statusStr }}
      </n-alert>
      <n-flex>
        <n-button
          type="success"
          :loading="fetchingStatus"
          :disabled="fetchingStatus || status"
          @click="handleStart"
        >
          {{ $gettext('Start') }}
        </n-button>
        <ConfirmDialog
          type="danger"
          :content="
            $gettext('Are you sure you want to stop %{ service }?', { service: props.service })
          "
          @confirm="handleStop"
        >
          <template #trigger>
            <n-button type="error" :loading="fetchingStatus" :disabled="fetchingStatus || !status">
              {{ $gettext('Stop') }}
            </n-button>
          </template>
        </ConfirmDialog>
        <n-button
          type="warning"
          :loading="fetchingStatus"
          :disabled="fetchingStatus || !status"
          @click="handleRestart"
        >
          {{ $gettext('Restart') }}
        </n-button>
        <n-button
          v-if="showReload"
          type="primary"
          :loading="fetchingStatus"
          :disabled="fetchingStatus || !status"
          @click="handleReload"
        >
          {{ $gettext('Reload') }}
        </n-button>
      </n-flex>
    </n-flex>
  </n-card>
</template>

<style scoped lang="scss"></style>
