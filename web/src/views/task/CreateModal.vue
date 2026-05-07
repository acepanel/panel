<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import storage from '@/api/panel/backup-storage'
import container from '@/api/panel/container'
import cron from '@/api/panel/cron'
import database from '@/api/panel/database'
import home from '@/api/panel/home'
import website from '@/api/panel/website'
import CronSelector from '@/components/common/CronSelector.vue'
import PathSelector from '@/components/common/PathSelector.vue'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const loading = ref(false)

const props = defineProps<{
  mode: 'create' | 'edit'
  editData?: any
}>()

const emit = defineEmits(['saved'])

const defaultModel = () => ({
  id: 0,
  name: '',
  type: 'shell',
  targets: [] as string[],
  keep: 1,
  sub_type: 'website',
  flock: false,
  storage: 0,
  script:
    `#!/bin/bash\nexport PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH\n\n` +
    $gettext('# Enter your script content here') +
    `\n`,
  time: '*/30 * * * *',
  url: '',
  method: 'GET',
  headers: {} as Record<string, string>,
  body: '',
  timeout: 10,
  insecure: false,
  retries: 0,
})

const headerList = ref<{ key: string; value: string }[]>([])

const formModel = ref(defaultModel())

const websites = ref<any[]>([])
const storages = ref<any[]>([])
const containers = ref<any[]>([])
const databases = ref<any[]>([])

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    db: [
      {
        label: '',
        value: '',
      },
    ],
  },
})

const mySQLInstalled = computed(() => {
  return installedEnvironment.value.db.find((item: any) => item.value === 'mysql')
})

const postgreSQLInstalled = computed(() => {
  return installedEnvironment.value.db.find((item: any) => item.value === 'postgresql')
})

const containerInstalled = ref(false)

const showPathSelector = ref(false)
const pathSelectorPath = ref('/')
const editingPathIndex = ref(-1)

const openPathSelector = (index: number) => {
  editingPathIndex.value = index
  pathSelectorPath.value = formModel.value.targets[index] || '/'
  showPathSelector.value = true
}

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value && editingPathIndex.value >= 0) {
    formModel.value.targets[editingPathIndex.value] = pathSelectorPath.value
    editingPathIndex.value = -1
  }
})

const handleSubmit = async () => {
  loading.value = true
  // 提交前将 headerList 同步到 headers map
  const headersMap: Record<string, string> = {}
  for (const item of headerList.value) {
    if (item.key) {
      headersMap[item.key] = item.value
    }
  }
  formModel.value.headers = headersMap
  const data = { ...formModel.value }
  const request = props.mode === 'create' ? cron.create(data) : cron.update(data.id, data)

  useRequest(request)
    .onSuccess(() => {
      show.value = false
      window.$message.success(
        props.mode === 'create'
          ? $gettext('Created successfully')
          : $gettext('Modified successfully'),
      )
      emit('saved')
      window.$bus.emit('task:refresh-cron')
    })
    .onComplete(() => {
      loading.value = false
    })
}

const generateTaskName = () => {
  const type = formModel.value.type
  const targets = formModel.value.targets

  if (type === 'backup') {
    const backupTypeMap: Record<string, string> = {
      website: $gettext('Backup Website'),
      mysql: $gettext('Backup MySQL'),
      postgresql: $gettext('Backup PostgreSQL'),
      path: $gettext('Backup Directory'),
    }
    const prefix = backupTypeMap[formModel.value.sub_type] || $gettext('Backup')
    formModel.value.name = targets.length ? `${prefix} - ${targets.join(', ')}` : prefix
  } else if (type === 'cutoff') {
    const cutoffTypeMap: Record<string, string> = {
      website: $gettext('Log Rotation - Website'),
      container: $gettext('Log Rotation - Container'),
    }
    const prefix = cutoffTypeMap[formModel.value.sub_type] || $gettext('Log Rotation')
    formModel.value.name = targets.length ? `${prefix} - ${targets.join(', ')}` : prefix
  } else if (type === 'url') {
    formModel.value.name = formModel.value.url
      ? `${formModel.value.method} - ${formModel.value.url}`
      : $gettext('Access URL')
  } else if (type === 'synctime') {
    formModel.value.name = $gettext('Sync Time')
  }
}

// 监听类型变更自动生成名称
watch(
  () => [
    formModel.value.type,
    formModel.value.sub_type,
    formModel.value.targets,
    formModel.value.url,
    formModel.value.method,
  ],
  () => {
    if (props.mode === 'create') {
      generateTaskName()
    }
  },
  { deep: true },
)

const skipClearTargets = ref(false)

// 切换类型或子类型时重置 targets
watch(
  () => formModel.value.sub_type,
  () => {
    if (skipClearTargets.value) return
    formModel.value.targets = []
  },
)
watch(
  () => formModel.value.type,
  () => {
    if (skipClearTargets.value) return
    formModel.value.targets = []
  },
)

// 加载容器列表
const loadContainers = () => {
  useRequest(container.containerList(1, 10000)).onSuccess(({ data }: { data: any }) => {
    containers.value = (data.items || []).map((item: any) => ({
      label: item.name,
      value: item.name,
    }))
  })
}

// 加载数据库列表
const loadDatabases = (type: string) => {
  useRequest(database.list(1, 10000, type)).onSuccess(({ data }: { data: any }) => {
    databases.value = (data.items || []).map((item: any) => ({
      label: `${item.name} (${item.server_name || 'local'})`,
      value: item.name,
    }))
  })
}

// 显示时初始化数据
watch(show, (val) => {
  if (val) {
    if (props.mode === 'edit' && props.editData) {
      const config = props.editData.config || {}
      skipClearTargets.value = true
      formModel.value = {
        id: props.editData.id,
        name: props.editData.name,
        type: props.editData.type,
        time: props.editData.time,
        targets: config.targets || [],
        keep: config.keep || 1,
        sub_type: config.type || '',
        flock: config.flock ?? false,
        storage: config.storage || 0,
        script: '',
        url: config.url || '',
        method: config.method || 'GET',
        headers: config.headers || {},
        body: config.body || '',
        timeout: config.timeout ?? 10,
        insecure: config.insecure ?? false,
        retries: config.retries ?? 0,
      }
      nextTick(() => {
        skipClearTargets.value = false
      })
      // 回填 headerList
      headerList.value = Object.entries(config.headers || {}).map(
        ([key, value]: [string, any]) => ({ key, value }),
      )
      // 加载关联数据
      if (props.editData.type === 'cutoff' && config.type === 'container') {
        loadContainers()
      }
      if (
        props.editData.type === 'backup' &&
        (config.type === 'mysql' || config.type === 'postgresql')
      ) {
        loadDatabases(config.type)
      }
    } else {
      formModel.value = defaultModel()
      headerList.value = []
    }
  }
})

// 监听子类型变化加载数据
watch(
  () => formModel.value.sub_type,
  (val) => {
    if (formModel.value.type === 'cutoff' && val === 'container') {
      loadContainers()
    }
    if (formModel.value.type === 'backup' && (val === 'mysql' || val === 'postgresql')) {
      loadDatabases(val)
    }
  },
)

onMounted(() => {
  useRequest(app.isInstalled('nginx,openresty,apache,caddy')).onSuccess(({ data }) => {
    if (data) {
      useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: { data: any }) => {
        for (const item of data.items) {
          websites.value.push({
            label: item.name,
            value: item.name,
          })
        }
      })
    }
  })
  useRequest(app.isInstalled('docker,podman')).onSuccess(({ data }) => {
    containerInstalled.value = !!data
  })
  useRequest(storage.list(1, 10000)).onSuccess(({ data }: { data: any }) => {
    storages.value = []
    for (const item of data.items) {
      storages.value.push({
        label: item.name,
        value: item.id,
      })
    }
  })
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="mode === 'create' ? $gettext('Create Scheduled Task') : $gettext('Edit Scheduled Task')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form>
      <n-form-item v-if="mode === 'create'" :label="$gettext('Task Type')">
        <n-select
          v-model:value="formModel.type"
          :options="[
            { label: $gettext('Run Script'), value: 'shell' },
            { label: $gettext('Backup Data'), value: 'backup' },
            { label: $gettext('Log Rotation'), value: 'cutoff' },
            { label: $gettext('Access URL'), value: 'url' },
            { label: $gettext('Sync Time'), value: 'synctime' },
          ]"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Task Name')">
        <n-input v-model:value="formModel.name" :placeholder="$gettext('Task Name')" />
      </n-form-item>
      <n-grid
        :cols="formModel.type === 'backup' || formModel.type === 'cutoff' ? 2 : 1"
        :x-gap="16"
      >
        <n-gi>
          <n-form-item :label="$gettext('Task Schedule')">
            <cron-selector v-model:value="formModel.time" />
          </n-form-item>
        </n-gi>
        <n-gi v-if="formModel.type === 'backup' || formModel.type === 'cutoff'">
          <n-form-item :label="$gettext('Retention Count')">
            <n-input-number v-model:value="formModel.keep" :min="1" w-full />
          </n-form-item>
        </n-gi>
      </n-grid>
      <n-form-item :label="$gettext('Process Lock')">
        <n-switch v-model:value="formModel.flock" />
        <n-text ml-10 depth="3">
          {{
            $gettext(
              'Prevent duplicate execution: skip this run if the previous one is still running',
            )
          }}
        </n-text>
      </n-form-item>
      <div v-if="formModel.type === 'shell'">
        <n-text>{{ $gettext('Script Content') }}</n-text>
        <common-editor v-model:value="formModel.script" lang="shell" height="40vh" />
      </div>
      <!-- URL 类型 -->
      <template v-if="formModel.type === 'url'">
        <n-form-item :label="$gettext('Request Method')">
          <n-select
            v-model:value="formModel.method"
            :options="[
              { label: 'GET', value: 'GET' },
              { label: 'POST', value: 'POST' },
              { label: 'PUT', value: 'PUT' },
              { label: 'DELETE', value: 'DELETE' },
              { label: 'PATCH', value: 'PATCH' },
              { label: 'HEAD', value: 'HEAD' },
            ]"
          />
        </n-form-item>
        <n-form-item label="URL">
          <n-input v-model:value="formModel.url" placeholder="https://example.com" />
        </n-form-item>
        <n-form-item :label="$gettext('Request Headers')">
          <n-dynamic-input v-model:value="headerList" :on-create="() => ({ key: '', value: '' })">
            <template #default="{ value }">
              <div style="display: flex; align-items: center; width: 100%; gap: 8px">
                <n-input
                  v-model:value="value.key"
                  :placeholder="$gettext('Header Name')"
                  style="flex: 1"
                />
                <n-input
                  v-model:value="value.value"
                  :placeholder="$gettext('Header Value')"
                  style="flex: 2"
                />
              </div>
            </template>
          </n-dynamic-input>
        </n-form-item>
        <n-form-item
          v-if="
            formModel.method === 'POST' ||
            formModel.method === 'PUT' ||
            formModel.method === 'PATCH'
          "
          :label="$gettext('Request Body')"
        >
          <n-input
            v-model:value="formModel.body"
            type="textarea"
            :placeholder="$gettext('Request Body')"
            :autosize="{ minRows: 3, maxRows: 10 }"
          />
        </n-form-item>
        <n-grid :cols="3" :x-gap="16">
          <n-gi>
            <n-form-item :label="$gettext('Timeout (seconds)')">
              <n-input-number v-model:value="formModel.timeout" :min="0" w-full />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="$gettext('Retry Count')">
              <n-input-number v-model:value="formModel.retries" :min="0" w-full />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="$gettext('Ignore Certificate')">
              <n-switch v-model:value="formModel.insecure" />
            </n-form-item>
          </n-gi>
        </n-grid>
      </template>
      <!-- 备份类型 -->
      <n-form-item v-if="formModel.type === 'backup'" :label="$gettext('Backup Type')">
        <n-radio-group v-model:value="formModel.sub_type">
          <n-radio value="website">{{ $gettext('Website') }}</n-radio>
          <n-radio value="mysql" :disabled="!mySQLInstalled">
            {{ $gettext('MySQL Database') }}
          </n-radio>
          <n-radio value="postgresql" :disabled="!postgreSQLInstalled">
            {{ $gettext('PostgreSQL Database') }}
          </n-radio>
          <n-radio value="path">{{ $gettext('Directory') }}</n-radio>
        </n-radio-group>
      </n-form-item>
      <!-- 日志切割子类型 -->
      <n-form-item v-if="formModel.type === 'cutoff'" :label="$gettext('Rotation Type')">
        <n-radio-group v-model:value="formModel.sub_type">
          <n-radio value="website">{{ $gettext('Website') }}</n-radio>
          <n-radio value="container" :disabled="!containerInstalled">{{
            $gettext('Container')
          }}</n-radio>
        </n-radio-group>
      </n-form-item>
      <!-- 网站多选 -->
      <n-form-item
        v-if="
          (formModel.sub_type === 'website' && formModel.type === 'backup') ||
          (formModel.type === 'cutoff' && formModel.sub_type === 'website')
        "
        :label="$gettext('Select Website')"
      >
        <n-select
          v-model:value="formModel.targets"
          :options="websites"
          multiple
          :placeholder="$gettext('Select Website')"
        />
      </n-form-item>
      <!-- 数据库多选 -->
      <n-form-item
        v-if="
          formModel.type === 'backup' &&
          (formModel.sub_type === 'mysql' || formModel.sub_type === 'postgresql')
        "
        :label="$gettext('Select Database')"
      >
        <n-select
          v-model:value="formModel.targets"
          :options="databases"
          multiple
          :placeholder="$gettext('Select Database')"
        />
      </n-form-item>
      <!-- 目录路径 -->
      <n-form-item
        v-if="formModel.type === 'backup' && formModel.sub_type === 'path'"
        :label="$gettext('Select Directory')"
      >
        <n-dynamic-input v-model:value="formModel.targets" :on-create="() => ''">
          <template #default="{ index }">
            <div style="display: flex; align-items: center; width: 100%; gap: 8px">
              <n-input
                v-model:value="formModel.targets[index]"
                :placeholder="$gettext('Enter directory path')"
                style="flex: 1"
              />
              <n-button @click="openPathSelector(index)">
                {{ $gettext('Browse') }}
              </n-button>
            </div>
          </template>
        </n-dynamic-input>
      </n-form-item>
      <!-- 容器多选 -->
      <n-form-item
        v-if="formModel.type === 'cutoff' && formModel.sub_type === 'container'"
        :label="$gettext('Select Container')"
      >
        <n-select
          v-model:value="formModel.targets"
          :options="containers"
          multiple
          :placeholder="$gettext('Select Container')"
        />
      </n-form-item>
      <!-- 存储选择 -->
      <n-form-item
        v-if="formModel.type === 'backup' || formModel.type === 'cutoff'"
        :label="$gettext('Storage')"
      >
        <n-select
          v-model:value="formModel.storage"
          :options="storages"
          :placeholder="$gettext('Select storage')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" :loading="loading" :disabled="loading" @click="handleSubmit" mt-10 block>
      {{ mode === 'create' ? $gettext('Submit') : $gettext('Save') }}
    </n-button>
  </n-modal>
  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
