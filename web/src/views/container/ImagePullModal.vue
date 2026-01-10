<script setup lang="ts">
import ws from '@/api/ws'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps<{
  image: string
}>()

const emit = defineEmits<{
  success: []
  cancel: []
}>()

const isPulling = ref(false)
const pullProgress = ref<Map<string, any>>(new Map())
const pullStatus = ref('')
const pullError = ref('')
let pullWs: WebSocket | null = null

// 计算总体拉取进度
const totalProgress = computed(() => {
  const layers = Array.from(pullProgress.value.values())
  if (layers.length === 0) return 0

  // 统计各状态的层数
  const completed = layers.filter(
    (p) => p.status === 'Pull complete' || p.status === 'Already exists'
  ).length
  const total = layers.filter((p) => p.id && p.id.length === 12).length

  return total > 0 ? Math.round((completed / total) * 100) : 0
})

// 拉取镜像
const pullImage = () => {
  isPulling.value = true
  pullProgress.value = new Map()
  pullStatus.value = $gettext('Connecting...')
  pullError.value = ''

  ws.imagePull(props.image)
    .then((socket) => {
      pullWs = socket
      pullStatus.value = $gettext('Pulling image...')

      socket.onmessage = (event) => {
        try {
          const data: any = JSON.parse(event.data)

          if (data.error) {
            pullError.value = data.error
            isPulling.value = false
            return
          }

          if (data.complete) {
            pullStatus.value = $gettext('Pull completed')
            isPulling.value = false
            show.value = false
            emit('success')
            return
          }

          // 更新进度
          if (data.id) {
            pullProgress.value.set(data.id, data)
            // 触发响应式更新
            pullProgress.value = new Map(pullProgress.value)
          }
          pullStatus.value = data.status
        } catch {
          // 忽略解析错误
        }
      }

      socket.onclose = () => {
        if (isPulling.value) {
          isPulling.value = false
        }
      }

      socket.onerror = () => {
        pullError.value = $gettext('Connection error')
        isPulling.value = false
      }
    })
    .catch((err) => {
      pullError.value = err.message || $gettext('Failed to connect')
      isPulling.value = false
    })
}

// 取消拉取
const cancelPull = () => {
  if (pullWs) {
    pullWs.close()
    pullWs = null
  }
  resetState()
  show.value = false
  emit('cancel')
}

// 重置状态
const resetState = () => {
  isPulling.value = false
  pullProgress.value = new Map()
  pullStatus.value = ''
  pullError.value = ''
}

watch(show, (val) => {
  if (val) {
    resetState()
    pullImage()
  } else {
    if (pullWs) {
      pullWs.close()
      pullWs = null
    }
  }
})

onUnmounted(() => {
  if (pullWs) {
    pullWs.close()
    pullWs = null
  }
})
</script>

<template>
  <n-modal
    v-model:show="show"
    :title="$gettext('Pulling Image')"
    preset="card"
    style="width: 60vw"
    size="medium"
    :bordered="false"
    :segmented="false"
    :mask-closable="false"
    :closable="false"
  >
    <!-- 拉取进度 -->
    <n-flex v-if="isPulling || (!pullError && pullProgress.size > 0)" vertical :size="16">
      <n-progress
        type="line"
        :percentage="totalProgress"
        :indicator-placement="'inside'"
        processing
      />

      <n-card size="small" :bordered="true" class="max-h-300 overflow-y-auto">
        <n-flex vertical :size="8">
          <div
            v-for="[id, progress] in pullProgress"
            :key="id"
            class="p-1 px-2 rounded bg-gray-100 dark:bg-gray-800"
          >
            <n-flex justify="space-between" align="center">
              <n-text depth="3" class="text-12 font-mono">
                {{ id.substring(0, 12) }}
              </n-text>
              <n-text depth="2" class="text-12">
                {{ progress.status }}
                <template v-if="progress.progress">
                  {{ progress.progress }}
                </template>
              </n-text>
            </n-flex>
          </div>
          <n-text v-if="pullProgress.size === 0" depth="3">
            {{ pullStatus }}
          </n-text>
        </n-flex>
      </n-card>

      <n-flex justify="center">
        <n-button @click="cancelPull" type="error" ghost>
          {{ $gettext('Cancel') }}
        </n-button>
      </n-flex>
    </n-flex>

    <!-- 拉取错误 -->
    <n-result
      v-else-if="pullError"
      status="error"
      :title="$gettext('Pull Failed')"
      :description="pullError"
    >
      <template #footer>
        <n-flex justify="center">
          <n-button @click="cancelPull">{{ $gettext('Cancel') }}</n-button>
          <n-button type="primary" @click="pullImage">{{ $gettext('Retry') }}</n-button>
        </n-flex>
      </template>
    </n-result>
  </n-modal>
</template>
