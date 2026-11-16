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
    type: String as () => 'obtain' | 'renew' | 'refresh',
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

// 续签与刷新共用续签接口，后端按证书来源决定走 ACME 还是重新读取文件
const texts = computed(() => {
  switch (props.mode) {
    case 'renew':
      return { title: $gettext('Renew Certificate'), success: $gettext('Renewal successful') }
    case 'refresh':
      return { title: $gettext('Refresh Certificate'), success: $gettext('Refreshed successfully') }
    default:
      return { title: $gettext('Issue Certificate'), success: $gettext('Issuance successful') }
  }
})

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

  const wsFactory = props.mode === 'obtain' ? ws.certObtain : ws.certRenew
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
          window.$message.success(texts.value.success)
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
  if (props.mode !== 'obtain') {
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

const showProgress = computed(() => {
  return progressLogs.value.length > 0 || !!errorMsg.value || isComplete.value
})
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="texts.title"
    :mask-closable="!loading"
    :closable="!loading"
    @after-leave="handleClose"
  >
    <n-form v-if="mode === 'obtain'" :model="model">
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
      <n-timeline-item v-if="isComplete" type="success" :content="texts.success" />
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
  </app-modal>
</template>

<style scoped lang="scss"></style>
