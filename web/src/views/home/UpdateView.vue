<script setup lang="ts">
defineOptions({
  name: 'home-update',
})

import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import ws from '@/api/ws'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTabStore } from '@/stores'
import { formatDateTime, http } from '@/utils'

import ChangelogContent from './ChangelogContent.vue'

interface Version {
  version: string
  type: string
  description: string
  updated_at: string
}

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const route = useRoute()
const tabStore = useTabStore()
const [DefineRelease, ReuseRelease] = createReusableTemplate<{ item: Version }>()

// 进入页面即检查；离线模式或上游不可达时接口报错，错误在本页展示而不是弹窗
const { data: info, error: checkError, send: check } = useRequest(home.updateInfo)

const updating = ref(false)
// 后端推送的各个步骤，等待面板重启作为最后一步追加进来
const progress = ref<{ text: string; at: number }[]>([])
const errorMsg = ref('')
const waitingRestart = ref(false)
const restartTimedOut = ref(false)

const versions = computed<Version[]>(() => info.value?.versions ?? [])

// 最后一条是正在进行（或出问题）的那一步，之前的都已完成，耗时取到下一步开始为止
const steps = computed(() =>
  progress.value.map(({ text, at }, index) => {
    const next = progress.value[index + 1]
    if (next) return { text, state: 'done', cost: `${((next.at - at) / 1000).toFixed(1)}s` }
    const state = errorMsg.value ? 'failed' : restartTimedOut.value ? 'stalled' : 'running'
    return { text, state, cost: '' }
  }),
)

const status = computed(() => {
  if (errorMsg.value) return 'failed'
  if (restartTimedOut.value) return 'timeout'
  if (updating.value) return 'updating'
  if (checkError.value) return 'unreachable'
  if (!info.value) return 'checking'
  return info.value.update ? 'available' : 'latest'
})

const tones = {
  checking: 'bg-bg-subtle text-text-tertiary',
  unreachable: 'bg-warning-subtle text-warning',
  available: 'bg-info-subtle text-info',
  latest: 'bg-success-subtle text-success',
  updating: 'bg-info-subtle text-info',
  failed: 'bg-error-subtle text-error',
  timeout: 'bg-warning-subtle text-warning',
}

const title = computed(() => {
  switch (status.value) {
    case 'checking':
      return $gettext('Checking for updates...')
    case 'unreachable':
      return $gettext('Failed to check for updates')
    case 'available':
      return $gettext('A new version is available')
    case 'latest':
      return $gettext('Current version is the latest')
    case 'updating':
      return $gettext('Updating to v%{ v }...', { v: info.value?.latest })
    case 'failed':
      return $gettext('Update failed')
    default:
      return $gettext('Update timed out')
  }
})

const typeLabel = (type: string) => (type === 'beta' ? $gettext('Beta') : $gettext('Stable'))

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

// 关闭更新页再整页跳转，避免刷新后仍停留在此页
const finish = async () => {
  tabStore.removeTab(route.path)
  await nextTick()
  window.location.href = '/'
}

// 升级会重启面板，重启期间请求失败（连接拒绝或 503）；静默轮询直到新版本就绪后刷新
const waitForRestart = async (targetVersion: string) => {
  waitingRestart.value = true
  progress.value.push({ text: $gettext('Panel is restarting, please wait...'), at: Date.now() })
  const started = Date.now()
  while (Date.now() - started < 120000) {
    await sleep(3000)
    try {
      const data: any = await http.Get('/home/system_info', { meta: { noAlert: true } })
      if (data && (!targetVersion || data.panel_version === targetVersion)) {
        await finish()
        return
      }
    } catch {
      // 面板重启中，继续等待
    }
  }
  restartTimedOut.value = true
}

const resetUpdate = () => {
  updating.value = false
  progress.value = []
  errorMsg.value = ''
  waitingRestart.value = false
  restartTimedOut.value = false
}

const startUpdate = () => {
  resetUpdate()
  updating.value = true
  const targetVersion = info.value?.latest ?? ''

  ws.panelUpdate()
    .then((socket) => {
      socket.onmessage = (event) => {
        let data
        try {
          data = JSON.parse(event.data)
        } catch {
          return
        }
        if (data.status === 'progress') {
          progress.value.push({ text: data.msg, at: Date.now() })
        } else if (data.status === 'error') {
          errorMsg.value = data.msg
        } else if (data.status === 'success') {
          waitForRestart(targetVersion)
        }
      }
      socket.onclose = () => {
        // 连接断开且未报错/未进入等待，可能面板已重启，转入等待重启轮询
        if (updating.value && !waitingRestart.value && !errorMsg.value) {
          waitForRestart(targetVersion)
        }
      }
      socket.onerror = () => {
        if (updating.value && !waitingRestart.value && !errorMsg.value) {
          errorMsg.value = $gettext('WebSocket connection failed')
        }
      }
    })
    .catch(() => {
      errorMsg.value = $gettext('WebSocket connection failed')
    })
}

const handleUpdate = async () => {
  const ok = await confirmAction({
    type: 'warning',
    title: $gettext('Update Panel'),
    content: $gettext('Are you sure you want to update the panel?'),
  })
  if (ok) startUpdate()
}
</script>

<template>
  <PageContainer :show-footer="true" bare>
    <div class="mx-auto flex flex-col gap-4 max-w-240 w-full">
      <!-- 一个版本的标题行，单个版本和折叠列表共用 -->
      <DefineRelease v-slot="{ item }">
        <div class="flex flex-wrap gap-2 items-center">
          <span class="text-base text-text-primary font-semibold">v{{ item.version }}</span>
          <n-tag
            v-if="info?.update && item.version === info.latest"
            type="success"
            size="small"
            round
          >
            {{ $gettext('Latest') }}
          </n-tag>
          <n-tag :type="item.type === 'beta' ? 'warning' : 'default'" size="small" round>
            {{ typeLabel(item.type) }}
          </n-tag>
          <span class="text-xs text-text-tertiary ml-auto">
            {{ formatDateTime(item.updated_at, 'yyyy-MM-dd') }}
          </span>
        </div>
      </DefineRelease>

      <!-- 状态 -->
      <n-card>
        <div class="flex flex-wrap gap-4 items-center">
          <div
            class="rounded-xl flex shrink-0 size-12 items-center justify-center"
            :class="tones[status]"
          >
            <n-spin v-if="status === 'checking' || status === 'updating'" :size="22" />
            <i-mdi-rocket-launch-outline v-else-if="status === 'available'" class="text-2xl" />
            <i-mdi-check-circle-outline v-else-if="status === 'latest'" class="text-2xl" />
            <i-mdi-close-circle-outline v-else-if="status === 'failed'" class="text-2xl" />
            <i-mdi-alert-outline v-else class="text-2xl" />
          </div>

          <div class="flex-1 min-w-0">
            <div class="text-lg text-text-primary font-semibold">{{ title }}</div>
            <div class="text-sm text-text-secondary mt-1 break-words">
              <span v-if="status === 'unreachable'">{{ checkError?.message }}</span>
              <span v-else-if="status === 'failed'">{{ errorMsg }}</span>
              <span v-else-if="status === 'timeout'">
                {{
                  $gettext(
                    'Update may have failed, please check the panel logs and refresh manually.',
                  )
                }}
              </span>
              <span v-else-if="status === 'updating'">
                {{ $gettext('Please do not close or refresh this page during the update') }}
              </span>
              <span v-else-if="status === 'available'" class="inline-flex gap-2 items-center">
                v{{ info.current }}
                <i-mdi-arrow-right class="text-text-tertiary" />
                <span class="text-success font-medium">v{{ info.latest }}</span>
              </span>
              <span v-else-if="status === 'latest'">
                {{ $gettext('Current version: v%{ v }', { v: info.current }) }}
              </span>
            </div>
          </div>

          <app-button
            v-if="status === 'available'"
            type="primary"
            size="large"
            class="lt-md:w-full"
            @click="handleUpdate"
          >
            <template #icon><i-mdi-download /></template>
            {{ $gettext('Update Now') }}
          </app-button>
          <app-button
            v-else-if="status === 'latest' || status === 'unreachable'"
            class="lt-md:w-full"
            @click="check"
          >
            <template #icon><i-mdi-refresh /></template>
            {{ status === 'latest' ? $gettext('Check Again') : $gettext('Retry') }}
          </app-button>
          <n-button v-else-if="status === 'failed'" class="lt-md:w-full" @click="resetUpdate">
            {{ $gettext('Back') }}
          </n-button>
          <n-button
            v-else-if="status === 'timeout'"
            type="primary"
            class="lt-md:w-full"
            @click="finish"
          >
            {{ $gettext('Refresh') }}
          </n-button>
        </div>
      </n-card>

      <!-- 更新进度 -->
      <n-card v-if="updating && steps.length">
        <div class="flex flex-col gap-1">
          <div
            v-for="(step, index) in steps"
            :key="index"
            class="px-3 py-2 rounded-lg flex gap-3 items-center"
            :class="{ 'bg-bg-subtle': step.state !== 'done' }"
          >
            <span class="flex shrink-0 size-5 items-center justify-center">
              <n-spin v-if="step.state === 'running'" :size="16" />
              <i-mdi-check-circle v-else-if="step.state === 'done'" class="text-xl text-success" />
              <i-mdi-close-circle v-else-if="step.state === 'failed'" class="text-xl text-error" />
              <i-mdi-alert-circle v-else class="text-xl text-warning" />
            </span>
            <span
              class="text-sm flex-1 min-w-0"
              :class="
                step.state === 'done' ? 'text-text-secondary' : 'text-text-primary font-medium'
              "
            >
              {{ step.text }}
            </span>
            <span class="text-xs text-text-tertiary font-mono">{{ step.cost }}</span>
          </div>
        </div>
      </n-card>

      <!-- 更新日志：有更新时是待更新的各个版本，已是最新时是当前版本 -->
      <n-card
        v-else-if="status === 'available' || status === 'latest'"
        :title="$gettext('Changelog')"
      >
        <n-empty v-if="!versions.length" :description="$gettext('No changelog available')" />
        <template v-else-if="versions.length === 1">
          <ReuseRelease :item="versions[0]!" />
          <changelog-content class="mt-4" :content="versions[0]!.description" />
        </template>
        <n-collapse v-else :default-expanded-names="[versions[0]!.version]">
          <n-collapse-item v-for="item in versions" :key="item.version" :name="item.version">
            <template #header>
              <ReuseRelease :item="item" class="flex-1" />
            </template>
            <changelog-content :content="item.description" />
          </n-collapse-item>
        </n-collapse>
      </n-card>
    </div>
  </PageContainer>
</template>
