<script setup lang="ts">
import { NButton, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import systemctl from '@/api/panel/systemctl'

const { $gettext } = useGettext()
const props = defineProps({
  service: {
    type: String,
    required: true
  },
  showReload: {
    type: Boolean,
    required: false
  }
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
  status.value = await systemctl.status(props.service)
  fetchingStatus.value = false
}

const fetchIsEnabled = async () => {
  fetchingIsEnabled.value = true
  isEnabled.value = await systemctl.isEnabled(props.service)
  fetchingIsEnabled.value = false
}

const handleStart = () => {
  const messageReactive = window.$message.loading($gettext('Starting...'), {
    duration: 0
  })
  fetchingStatus.value = true
  useRequest(systemctl.start(props.service))
    .onSuccess(() => {
      window.$message.success($gettext('Started successfully'))
      fetchStatus()
    })
    .onComplete(() => {
      messageReactive?.destroy()
    })
}

const handleStop = () => {
  const messageReactive = window.$message.loading($gettext('Stopping...'), {
    duration: 0
  })
  fetchingStatus.value = true
  useRequest(systemctl.stop(props.service))
    .onSuccess(() => {
      window.$message.success($gettext('Stopped successfully'))
      fetchStatus()
    })
    .onComplete(() => {
      messageReactive?.destroy()
    })
}

const handleRestart = () => {
  const messageReactive = window.$message.loading($gettext('Restarting...'), {
    duration: 0
  })
  fetchingStatus.value = true
  useRequest(systemctl.restart(props.service))
    .onSuccess(() => {
      window.$message.success($gettext('Restarted successfully'))
      fetchStatus()
    })
    .onComplete(() => {
      messageReactive?.destroy()
    })
}

const handleReload = () => {
  const messageReactive = window.$message.loading($gettext('Reloading...'), {
    duration: 0
  })
  fetchingStatus.value = true
  useRequest(systemctl.reload(props.service))
    .onSuccess(() => {
      window.$message.success($gettext('Reloaded successfully'))
      fetchStatus()
    })
    .onComplete(() => {
      messageReactive?.destroy()
    })
}

const handleIsEnabled = async () => {
  const messageReactive = window.$message.loading($gettext('Setting autostart...'), {
    duration: 0
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
  <n-card :title="$gettext('Running Status')">
    <template #header-extra>
      <n-switch
        :disabled="fetchingIsEnabled"
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
        <n-button type="success" :loading="fetchingStatus" :disabled="fetchingStatus || status" @click="handleStart">
          {{ $gettext('Start') }}
        </n-button>
        <n-popconfirm @positive-click="handleStop">
          <template #trigger>
            <n-button type="error" :loading="fetchingStatus" :disabled="fetchingStatus || !status">
              {{ $gettext('Stop') }}
            </n-button>
          </template>
          {{ $gettext('Are you sure you want to stop %{ service }?', { service: props.service }) }}
        </n-popconfirm>
        <n-button type="warning" :loading="fetchingStatus" :disabled="fetchingStatus || !status" @click="handleRestart">
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
