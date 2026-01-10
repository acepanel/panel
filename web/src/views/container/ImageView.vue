<script setup lang="ts">
import { NButton, NDataTable, NFlex, NInput, NPopconfirm, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import ws from '@/api/ws'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()

const pullModel = ref({
  name: '',
  auth: false,
  username: '',
  password: ''
})
const pullModal = ref(false)
const selectedRowKeys = ref<any>([])

// 镜像拉取进度状态
const isPulling = ref(false)
const pullProgress = ref<Map<string, any>>(new Map())
const pullStatus = ref('')
const pullError = ref('')
let pullWs: WebSocket | null = null

// 计算总体拉取进度
const totalProgress = computed(() => {
  const layers = Array.from(pullProgress.value.values())
  if (layers.length === 0) return 0

  const completed = layers.filter(
    (p) => p.status === 'Pull complete' || p.status === 'Already exists'
  ).length
  const total = layers.filter((p) => p.id && p.id.length === 12).length

  return total > 0 ? Math.round((completed / total) * 100) : 0
})

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: 'ID',
    key: 'id',
    minWidth: 400,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Container Count'),
    key: 'containers',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Image'),
    key: 'repo_tags',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NFlex, null, {
        default: () =>
          row.repo_tags.map((tag: any) =>
            h(NTag, null, {
              default: () => tag
            })
          )
      })
    }
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    resizable: true,
    render(row: any) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NPopconfirm,
          {
            onPositiveClick: async () => {
              await handleDelete(row)
            }
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete?')
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => container.imageList(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleDelete = async (row: any) => {
  useRequest(container.imageRemove(row.id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Delete successful'))
  })
}

const handlePrune = () => {
  useRequest(container.imagePrune()).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Cleanup successful'))
  })
}

const handleBulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => container.imageRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

// 取消拉取
const cancelPull = () => {
  if (pullWs) {
    pullWs.close()
    pullWs = null
  }
  resetState()
}

// 重置拉取状态
const resetState = () => {
  isPulling.value = false
  pullProgress.value = new Map()
  pullStatus.value = ''
  pullError.value = ''
}

// 拉取镜像
const handlePull = () => {
  if (!pullModel.value.name) {
    window.$message.warning($gettext('Please enter image name'))
    return
  }

  isPulling.value = true
  pullProgress.value = new Map()
  pullStatus.value = $gettext('Connecting...')
  pullError.value = ''

  const auth = pullModel.value.auth
    ? { username: pullModel.value.username, password: pullModel.value.password }
    : undefined

  ws.imagePull(pullModel.value.name, auth)
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
            pullModal.value = false
            refresh()
            window.$message.success($gettext('Pull successful'))
            return
          }

          // 更新进度
          if (data.id) {
            pullProgress.value.set(data.id, data)
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

// 监听弹窗打开，重置状态
watch(pullModal, (val) => {
  if (val) {
    resetState()
  } else {
    if (pullWs) {
      pullWs.close()
      pullWs = null
    }
  }
})

onMounted(() => {
  refresh()
})

onUnmounted(() => {
  cancelPull()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="pullModal = true">{{ $gettext('Pull Image') }}</n-button>
      <n-button type="primary" @click="handlePrune" ghost>
        {{ $gettext('Cleanup Images') }}
      </n-button>
      <n-popconfirm @positive-click="handleBulkDelete">
        <template #trigger>
          <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </n-button>
        </template>
        {{ $gettext('Are you sure you want to delete the selected images?') }}
      </n-popconfirm>
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1000"
      :data="data"
      :columns="columns"
      :row-key="(row: any) => row.id"
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-flex>
  <n-modal
    v-model:show="pullModal"
    preset="card"
    :title="$gettext('Pull Image')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    :mask-closable="!isPulling"
    :closable="!isPulling"
  >
    <!-- 拉取进度 -->
    <template v-if="isPulling || pullProgress.size > 0">
      <n-flex vertical :size="16">
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
    </template>

    <!-- 拉取错误 -->
    <n-result
      v-else-if="pullError"
      status="error"
      :title="$gettext('Pull Failed')"
      :description="pullError"
    >
      <template #footer>
        <n-flex justify="center">
          <n-button @click="resetState">{{ $gettext('Cancel') }}</n-button>
          <n-button type="primary" @click="handlePull">{{ $gettext('Retry') }}</n-button>
        </n-flex>
      </template>
    </n-result>

    <!-- 拉取表单 -->
    <template v-else>
      <n-form :model="pullModel">
        <n-form-item path="name" :label="$gettext('Image Name')">
          <n-input
            v-model:value="pullModel.name"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('docker.io/php:8.3-fpm')"
          />
        </n-form-item>
        <n-form-item path="auth" :label="$gettext('Authentication')">
          <n-switch v-model:value="pullModel.auth" />
        </n-form-item>
        <n-form-item v-if="pullModel.auth" path="username" :label="$gettext('Username')">
          <n-input
            v-model:value="pullModel.username"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item v-if="pullModel.auth" path="password" :label="$gettext('Password')">
          <n-input
            v-model:value="pullModel.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handlePull">
        {{ $gettext('Submit') }}
      </n-button>
    </template>
  </n-modal>
</template>
