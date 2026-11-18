<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import ssh from '@/api/panel/ssh'
import { formatBytes, joinPath } from '@/utils'

const { $gettext } = useGettext()

const props = defineProps<{
  hosts: { label: string; value: number }[]
}>()

const hostId = defineModel<number>('hostId', { required: true })
const path = defineModel<string>('path', { required: true })

interface FileItem {
  name: string
  size: number
  mode: string
  mod_time: number
  is_dir: boolean
  is_link: boolean
}

const files = ref<FileItem[]>([])
const loading = ref(false)
const selected = ref<Set<string>>(new Set())
const showMkdir = ref(false)
const mkdirName = ref('')

// 面板本机在终端界面约定为 -1,API 侧用 0 表示
const apiId = computed(() => (hostId.value === -1 ? 0 : hostId.value))

const selectedFiles = computed(() => files.value.filter((f) => selected.value.has(f.name)))

const refresh = () => {
  loading.value = true
  selected.value = new Set()
  useRequest(ssh.listFiles(apiId.value, path.value))
    .onSuccess(({ data }: any) => {
      files.value = data || []
    })
    .onError(() => {
      files.value = []
    })
    .onComplete(() => {
      loading.value = false
    })
}

const handleRowClick = (file: FileItem) => {
  if (file.is_dir || file.is_link) {
    path.value = joinPath(path.value, file.name)
    return
  }
  toggleSelect(file.name)
}

const toggleSelect = (name: string) => {
  const next = new Set(selected.value)
  if (next.has(name)) {
    next.delete(name)
  } else {
    next.add(name)
  }
  selected.value = next
}

const handleUp = () => {
  if (path.value === '/') return
  const parts = path.value.split('/').filter(Boolean)
  parts.pop()
  path.value = '/' + parts.join('/')
}

const handlePathInput = (value: string) => {
  path.value = value.trim() || '/'
}

// 输入框回车与按钮共用，请求期间两者都禁用
const creating = ref(false)

const handleMkdir = async () => {
  const name = mkdirName.value.trim()
  if (!name || creating.value) return
  creating.value = true
  try {
    await ssh.mkdir(apiId.value, joinPath(path.value, name))
    showMkdir.value = false
    mkdirName.value = ''
    refresh()
  } finally {
    creating.value = false
  }
}

const formatTime = (ts: number) => {
  if (!ts || ts <= 0) return '-'
  return new Date(ts * 1000).toLocaleString()
}

// 切换主机时回到根目录
watch(hostId, () => {
  if (path.value === '/') {
    refresh()
  } else {
    path.value = '/'
  }
})

watch(path, refresh)

onMounted(refresh)

defineExpose({ selectedFiles, refresh, hostId, path })
</script>

<template>
  <div class="sftp-browser">
    <div class="sftp-toolbar">
      <n-select
        :value="hostId"
        :options="props.hosts"
        size="small"
        class="sftp-host-select md:!w-35"
        @update:value="(v: number) => (hostId = v)"
      />
      <n-input
        :value="path"
        size="small"
        class="sftp-path-input"
        @change="handlePathInput"
        @keydown.enter="(e: any) => handlePathInput(e.target.value)"
      />
      <n-button-group size="small">
        <n-button size="small" :title="$gettext('Parent Directory')" @click="handleUp">
          <template #icon>
            <i-mdi-arrow-up />
          </template>
        </n-button>
        <n-button size="small" :title="$gettext('Refresh')" @click="refresh">
          <template #icon>
            <i-mdi-refresh />
          </template>
        </n-button>
        <n-popover v-model:show="showMkdir" trigger="click" placement="bottom">
          <template #trigger>
            <n-button size="small" :title="$gettext('Create Directory')">
              <template #icon>
                <i-mdi-folder-plus-outline />
              </template>
            </n-button>
          </template>
          <n-flex :size="8">
            <n-input
              v-model:value="mkdirName"
              size="small"
              :placeholder="$gettext('Directory name')"
              class="w-40"
              :disabled="creating"
              :loading="creating || undefined"
              @keydown.enter="handleMkdir"
            />
            <n-button size="small" type="primary" :loading="creating" @click="handleMkdir">
              {{ $gettext('Create') }}
            </n-button>
          </n-flex>
        </n-popover>
      </n-button-group>
    </div>

    <n-spin :show="loading" class="sftp-list-spin" content-class="sftp-list-spin-content">
      <div class="sftp-list">
        <div
          v-for="file in files"
          :key="file.name"
          class="sftp-row"
          :class="{ selected: selected.has(file.name) }"
          @click="handleRowClick(file)"
        >
          <n-checkbox
            :checked="selected.has(file.name)"
            class="sftp-row-check"
            @click.stop
            @update:checked="() => toggleSelect(file.name)"
          />
          <span class="sftp-row-icon">
            <i-mdi-folder v-if="file.is_dir" class="text-amber-500" />
            <i-mdi-link-variant v-else-if="file.is_link" />
            <i-mdi-file-outline v-else />
          </span>
          <span class="sftp-row-name" :title="file.name">{{ file.name }}</span>
          <span class="sftp-row-size">{{ file.is_dir ? '-' : formatBytes(file.size) }}</span>
          <span class="sftp-row-time hidden md:block">{{ formatTime(file.mod_time) }}</span>
        </div>
        <n-empty
          v-if="!files.length && !loading"
          :description="$gettext('Empty directory')"
          size="small"
          class="sftp-empty"
        />
      </div>
    </n-spin>
  </div>
</template>

<style scoped lang="scss">
.sftp-browser {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
  background: var(--color-bg-elevated);
  overflow: hidden;
}

// 手机上主机下拉满宽独占一行，路径框与按钮组换到第二行
.sftp-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  padding: 8px;
  border-bottom: 1px solid var(--color-border-default);
  flex-shrink: 0;
}

.sftp-host-select {
  flex-shrink: 0;
}

.sftp-path-input {
  flex: 1;
  min-width: 0;
}

.sftp-list-spin {
  flex: 1;
  min-height: 0;

  :deep(.sftp-list-spin-content) {
    height: 100%;
  }
}

.sftp-list {
  height: 100%;
  overflow-y: auto;
  padding: 4px;
}

.sftp-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
  color: var(--color-text-primary);

  &:hover {
    background: var(--color-bg-subtle);
  }

  &.selected {
    background: var(--color-brand-subtle);
  }
}

.sftp-row-check {
  width: 16px;
  flex-shrink: 0;
  display: inline-flex;
}

.sftp-row-icon {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  font-size: 15px;
  color: var(--color-text-secondary);
}

.sftp-row-name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sftp-row-size {
  width: 80px;
  text-align: right;
  color: var(--color-text-secondary);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.sftp-row-time {
  width: 150px;
  text-align: right;
  color: var(--color-text-secondary);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.sftp-empty {
  padding: 24px 0;
}
</style>
