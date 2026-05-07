<script setup lang="ts">
import { NButton, NTimeline, NTimelineItem } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import ws from '@/api/ws'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

const props = defineProps({
  mode: {
    type: String as () => 'obtain' | 'renew',
    default: 'obtain',
  },
})

const model = ref({
  type: 'auto',
})

const loading = ref(false)
const progressLogs = ref<string[]>([])
const errorMsg = ref('')
const isComplete = ref(false)
let currentWs: WebSocket | null = null

const options = [
  { label: $gettext('Automatic'), value: 'auto' },
  { label: $gettext('Self-signed'), value: 'self-signed' },
]

const resetState = () => {
  progressLogs.value = []
  errorMsg.value = ''
  isComplete.value = false
  if (currentWs) {
    currentWs.close()
    currentWs = null
  }
}

const handleWsObtainOrRenew = () => {
  loading.value = true
  resetState()

  const wsFactory = props.mode === 'renew' ? ws.certRenew : ws.certObtain
  wsFactory(id.value)
    .then((socket) => {
      currentWs = socket
      socket.onmessage = (event) => {
        let data
        try {
          data = JSON.parse(event.data)
        } catch {
          return
        }
        if (data.status === 'success') {
          isComplete.value = true
          loading.value = false
          window.$bus.emit('cert:refresh-cert')
          window.$bus.emit('cert:refresh-async')
          window.$message.success(
            props.mode === 'renew'
              ? $gettext('Renewal successful')
              : $gettext('Issuance successful'),
          )
        } else if (data.status === 'error') {
          errorMsg.value = data.msg
          loading.value = false
        } else if (data.status === 'progress') {
          progressLogs.value.push(data.msg)
        }
      }
      socket.onclose = () => {
        if (!isComplete.value && !errorMsg.value) {
          loading.value = false
        }
        currentWs = null
      }
      socket.onerror = () => {
        errorMsg.value = $gettext('WebSocket connection failed')
        loading.value = false
        currentWs = null
      }
    })
    .catch(() => {
      errorMsg.value = $gettext('WebSocket connection failed')
      loading.value = false
    })
}

const handleSubmit = () => {
  if (props.mode === 'renew') {
    handleWsObtainOrRenew()
    return
  }

  if (model.value.type === 'auto') {
    handleWsObtainOrRenew()
  } else {
    loading.value = true
    useRequest(cert.obtainSelfSigned(id.value))
      .onSuccess(() => {
        window.$bus.emit('cert:refresh-cert')
        window.$bus.emit('cert:refresh-async')
        show.value = false
        window.$message.success($gettext('Issuance successful'))
      })
      .onComplete(() => {
        loading.value = false
      })
  }
}

const handleClose = () => {
  resetState()
  model.value.type = 'auto'
}

const modalTitle = computed(() => {
  return props.mode === 'renew' ? $gettext('Renew Certificate') : $gettext('Issue Certificate')
})

const showProgress = computed(() => {
  return progressLogs.value.length > 0 || !!errorMsg.value || isComplete.value
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="modalTitle"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    :mask-closable="!loading"
    :closable="!loading"
    @after-leave="handleClose"
  >
    <n-form v-if="mode !== 'renew'" :model="model">
      <n-form-item path="type" :label="$gettext('Issuance Mode')">
        <n-select
          v-model:value="model.type"
          :options="options"
          :disabled="loading || showProgress"
        />
      </n-form-item>
    </n-form>

    <n-timeline v-if="showProgress" style="margin: 16px 0">
      <n-timeline-item
        v-for="(log, index) in progressLogs"
        :key="index"
        type="success"
        :content="log"
      />
      <n-timeline-item v-if="errorMsg" type="error" :content="errorMsg" />
      <n-timeline-item
        v-if="isComplete"
        type="success"
        :content="
          mode === 'renew' ? $gettext('Renewal successful') : $gettext('Issuance successful')
        "
      />
    </n-timeline>

    <n-button
      v-if="!isComplete"
      type="info"
      block
      :loading="loading"
      :disabled="loading"
      @click="handleSubmit"
    >
      {{ $gettext('Submit') }}
    </n-button>
    <n-button v-else type="success" block @click="show = false">
      {{ $gettext('Close') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
