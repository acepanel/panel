<script setup lang="ts">
defineOptions({
  name: 'toolbox-migration'
})

import home from '@/api/panel/home'
import migration from '@/api/panel/toolbox-migration'
import ws from '@/api/ws'
import { useRequest } from 'alova/client'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

// 步骤状态
const currentStep = ref(1)
const loading = ref(false)

// 第一步：连接信息
const connectionForm = ref({
  url: '',
  token_id: 1,
  token: ''
})

// 第二步：环境对比
const localEnv = ref<any>(null)
const remoteEnv = ref<any>(null)
const envCheckPassed = ref(false)
const envWarnings = ref<string[]>([])

// 第三步：迁移项选择
const websites = ref<any[]>([])
const databases = ref<any[]>([])
const projects = ref<any[]>([])
const selectedWebsites = ref<number[]>([])
const selectedDatabases = ref<string[]>([])
const selectedProjects = ref<number[]>([])
const stopOnMig = ref(true)

// 第四步：迁移进度
const migrationLogs = ref<string[]>([])
const migrationResults = ref<any[]>([])
const migrationRunning = ref(false)

// 第五步：迁移结果
const migrationStartedAt = ref<string | null>(null)
const migrationEndedAt = ref<string | null>(null)

// WebSocket 连接
let progressWs: WebSocket | null = null

// 日志容器引用
const logContainer = ref<HTMLElement | null>(null)

// 初始化：检查是否有正在进行的迁移
const checkStatus = () => {
  useRequest(migration.status()).onSuccess(({ data }: any) => {
    if (data.step === 'running') {
      currentStep.value = 4
      migrationRunning.value = true
      connectProgressWs()
    } else if (data.step === 'done') {
      currentStep.value = 5
      migrationResults.value = data.results || []
      migrationStartedAt.value = data.started_at
      migrationEndedAt.value = data.ended_at
    }
  })
}

onMounted(() => {
  checkStatus()
})

onUnmounted(() => {
  if (progressWs) {
    progressWs.close()
    progressWs = null
  }
})

// 第一步：连接并预检查
const handlePreCheck = () => {
  if (!connectionForm.value.url || !connectionForm.value.token_id || !connectionForm.value.token) {
    window.$message.error($gettext('Please fill in all connection fields'))
    return
  }

  loading.value = true
  useRequest(migration.precheck(connectionForm.value))
    .onSuccess(({ data }: any) => {
      remoteEnv.value = data.remote
      // 同时获取本地环境信息
      useRequest(home.installedEnvironment()).onSuccess(({ data: localData }: any) => {
        localEnv.value = localData
        checkEnvironment()
        currentStep.value = 2
        loading.value = false
      })
    })
    .onComplete(() => {
      loading.value = false
    })
}

// 第二步：刷新环境检查
const handleRefreshPreCheck = () => {
  if (!connectionForm.value.url || !connectionForm.value.token_id || !connectionForm.value.token) {
    window.$message.error($gettext('Please fill in all connection fields'))
    return
  }

  loading.value = true
  useRequest(migration.precheck(connectionForm.value))
    .onSuccess(({ data }: any) => {
      remoteEnv.value = data.remote
      useRequest(home.installedEnvironment()).onSuccess(({ data: localData }: any) => {
        localEnv.value = localData
        checkEnvironment()
        loading.value = false
        window.$message.success($gettext('Environment check refreshed'))
      })
    })
    .onComplete(() => {
      loading.value = false
    })
}

// 环境检查逻辑
const checkEnvironment = () => {
  const warnings: string[] = []
  let passed = true

  if (!localEnv.value || !remoteEnv.value) return

  // webserver 必须一致
  if (localEnv.value.webserver !== remoteEnv.value.webserver) {
    warnings.push(
      $gettext(
        'Web server mismatch: local is %{local}, remote is %{remote}. Migration cannot proceed.',
        {
          local: localEnv.value.webserver || $gettext('none'),
          remote: remoteEnv.value.webserver || $gettext('none')
        }
      )
    )
    passed = false
  }

  // 检查其他环境差异
  const envTypes = ['go', 'java', 'nodejs', 'php', 'python']
  for (const envType of envTypes) {
    const localItems = localEnv.value[envType] || []
    const remoteItems = remoteEnv.value[envType] || []
    if (localItems.length > 0 && remoteItems.length === 0) {
      warnings.push(
        $gettext(
          '%{type} is installed locally but not on the remote server. Related projects may need reconfiguration.',
          {
            type: envType.toUpperCase()
          }
        )
      )
    }
  }

  // 检查数据库差异
  const localDBTypes = (localEnv.value.db || [])
    .map((d: any) => d.value)
    .filter((v: string) => v !== '0')
  const remoteDBTypes = (remoteEnv.value.db || [])
    .map((d: any) => d.value)
    .filter((v: string) => v !== '0')
  for (const dbType of localDBTypes) {
    if (!remoteDBTypes.includes(dbType)) {
      warnings.push(
        $gettext(
          '%{type} is installed locally but not on the remote server. Database migration for this type will be skipped.',
          {
            type: dbType.toUpperCase()
          }
        )
      )
    }
  }

  envWarnings.value = warnings
  envCheckPassed.value = passed
}

// 第二步：获取可迁移项列表
const handleGetItems = () => {
  loading.value = true
  useRequest(migration.items())
    .onSuccess(({ data }: any) => {
      websites.value = data.websites || []
      databases.value = data.databases || []
      projects.value = data.projects || []
      currentStep.value = 3
      loading.value = false
    })
    .onComplete(() => {
      loading.value = false
    })
}

// 第三步：开始迁移
const handleStartMigration = () => {
  const selectedItems = {
    websites: websites.value
      .filter((_: any, i: number) => selectedWebsites.value.includes(i))
      .map((w: any) => ({ id: w.id, name: w.name, path: w.path })),
    databases: databases.value
      .filter((_: any, i: number) => selectedDatabases.value.includes(String(i)))
      .map((d: any) => ({
        type: d.type,
        name: d.name,
        server_id: d.server_id,
        server: d.server
      })),
    projects: projects.value
      .filter((_: any, i: number) => selectedProjects.value.includes(i))
      .map((p: any) => ({ id: p.id, name: p.name, path: p.root_dir || p.path })),
    stop_on_mig: stopOnMig.value
  }

  if (
    selectedItems.websites.length === 0 &&
    selectedItems.databases.length === 0 &&
    selectedItems.projects.length === 0
  ) {
    window.$message.warning($gettext('Please select at least one item to migrate'))
    return
  }

  window.$dialog.warning({
    title: $gettext('Confirm Migration'),
    content: $gettext(
      'Are you sure you want to start migration? This will transfer the selected items to the remote server.'
    ),
    positiveText: $gettext('Start'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      loading.value = true
      migrationLogs.value = []
      migrationResults.value = []

      useRequest(migration.start(selectedItems))
        .onSuccess(() => {
          currentStep.value = 4
          migrationRunning.value = true
          loading.value = false
          connectProgressWs()
        })
        .onComplete(() => {
          loading.value = false
        })
    }
  })
}

// 连接 WebSocket 获取进度
const connectProgressWs = async () => {
  try {
    progressWs = await ws.migrationProgress()
    progressWs.onmessage = (event: MessageEvent) => {
      const data = JSON.parse(event.data)
      migrationResults.value = data.results || []
      migrationStartedAt.value = data.started_at
      migrationEndedAt.value = data.ended_at

      if (data.new_logs) {
        migrationLogs.value.push(...data.new_logs)
        // 限制日志行数
        if (migrationLogs.value.length > 1000) {
          migrationLogs.value = migrationLogs.value.slice(-1000)
        }
        // 自动滚动到底部
        nextTick(() => {
          if (logContainer.value) {
            logContainer.value.scrollTop = logContainer.value.scrollHeight
          }
        })
      }

      if (data.step === 'done') {
        migrationRunning.value = false
        currentStep.value = 5
        if (progressWs) {
          progressWs.close()
          progressWs = null
        }
      }
    }
    progressWs.onclose = () => {
      if (migrationRunning.value) {
        // 连接意外断开，尝试重连
        setTimeout(connectProgressWs, 3000)
      }
    }
  } catch {
    // 如果 WebSocket 连接失败，回退到轮询
    pollProgress()
  }
}

// 轮询进度（备用方案）
const pollProgress = () => {
  const timer = setInterval(() => {
    useRequest(migration.results()).onSuccess(({ data }: any) => {
      migrationResults.value = data.results || []
      migrationStartedAt.value = data.started_at
      migrationEndedAt.value = data.ended_at
      if (data.logs) {
        migrationLogs.value = data.logs
      }
      if (data.step === 'done') {
        migrationRunning.value = false
        currentStep.value = 5
        clearInterval(timer)
      }
    })
  }, 2000)
}

// 重置迁移
const handleReset = () => {
  window.$dialog.warning({
    title: $gettext('Reset Migration'),
    content: $gettext('Are you sure you want to reset the migration state?'),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      useRequest(migration.reset()).onSuccess(() => {
        currentStep.value = 1
        connectionForm.value = { url: '', token_id: 1, token: '' }
        localEnv.value = null
        remoteEnv.value = null
        envCheckPassed.value = false
        envWarnings.value = []
        websites.value = []
        databases.value = []
        projects.value = []
        selectedWebsites.value = []
        selectedDatabases.value = []
        selectedProjects.value = []
        migrationLogs.value = []
        migrationResults.value = []
        migrationStartedAt.value = null
        migrationEndedAt.value = null
        window.$message.success($gettext('Migration state has been reset'))
      })
    }
  })
}

// 获取状态标签类型
const getStatusType = (status: string) => {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'error'
    case 'running':
      return 'warning'
    case 'skipped':
      return 'default'
    default:
      return 'info'
  }
}

// 格式化耗时
const formatDuration = (seconds: number) => {
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  const mins = Math.floor(seconds / 60)
  const secs = (seconds % 60).toFixed(1)
  return `${mins}m ${secs}s`
}
</script>

<template>
  <n-flex vertical>
    <!-- 步骤指示器 -->
    <n-card>
      <n-steps :current="currentStep" size="small">
        <n-step
          :title="$gettext('Connection')"
          :description="$gettext('Enter remote server info')"
        />
        <n-step :title="$gettext('Pre-check')" :description="$gettext('Verify environment')" />
        <n-step
          :title="$gettext('Select Items')"
          :description="$gettext('Choose what to migrate')"
        />
        <n-step :title="$gettext('Migrating')" :description="$gettext('Transfer in progress')" />
        <n-step :title="$gettext('Complete')" :description="$gettext('View results')" />
      </n-steps>
    </n-card>

    <!-- 第一步：连接信息 -->
    <n-card v-if="currentStep === 1" :title="$gettext('Remote Server Connection')">
      <n-form label-placement="left" label-width="auto">
        <n-form-item :label="$gettext('Panel URL')">
          <n-input
            v-model:value="connectionForm.url"
            :placeholder="$gettext('e.g. https://remote-server:8888')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Token ID')">
          <n-input-number
            v-model:value="connectionForm.token_id"
            :placeholder="$gettext('API Token ID')"
            :min="1"
            style="width: 100%"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Access Token')">
          <n-input
            v-model:value="connectionForm.token"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('API Access Token')"
          />
        </n-form-item>
      </n-form>
      <n-flex justify="end">
        <n-button type="primary" :loading="loading" :disabled="loading" @click="handlePreCheck">
          {{ $gettext('Next') }}
        </n-button>
      </n-flex>
    </n-card>

    <!-- 第二步：环境预检查 -->
    <n-card v-if="currentStep === 2" :title="$gettext('Environment Pre-check')">
      <!-- 警告信息 -->
      <n-flex v-if="envWarnings.length > 0" vertical style="margin-bottom: 16px">
        <n-alert
          v-for="(warning, index) in envWarnings"
          :key="index"
          :type="envCheckPassed ? 'warning' : 'error'"
          style="margin-bottom: 8px"
        >
          {{ warning }}
        </n-alert>
      </n-flex>

      <!-- 环境对比表 -->
      <n-table :bordered="true" :single-line="false" size="small">
        <thead>
          <tr>
            <th>{{ $gettext('Environment') }}</th>
            <th>{{ $gettext('Local') }}</th>
            <th>{{ $gettext('Remote') }}</th>
            <th>{{ $gettext('Status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>{{ $gettext('Web Server') }}</td>
            <td>{{ localEnv?.webserver || $gettext('None') }}</td>
            <td>{{ remoteEnv?.webserver || $gettext('None') }}</td>
            <td>
              <n-tag
                :type="localEnv?.webserver === remoteEnv?.webserver ? 'success' : 'error'"
                size="small"
              >
                {{
                  localEnv?.webserver === remoteEnv?.webserver
                    ? $gettext('Match')
                    : $gettext('Mismatch')
                }}
              </n-tag>
            </td>
          </tr>
          <tr v-for="envType in ['go', 'java', 'nodejs', 'php', 'python']" :key="envType">
            <td>{{ envType.toUpperCase() }}</td>
            <td>
              <template v-if="(localEnv?.[envType] || []).length > 0">
                <n-tag
                  v-for="item in localEnv[envType]"
                  :key="item.value"
                  size="small"
                  style="margin: 2px"
                >
                  {{ item.label }}
                </n-tag>
              </template>
              <template v-else>
                <n-text depth="3">{{ $gettext('Not installed') }}</n-text>
              </template>
            </td>
            <td>
              <template v-if="(remoteEnv?.[envType] || []).length > 0">
                <n-tag
                  v-for="item in remoteEnv[envType]"
                  :key="item.value"
                  size="small"
                  style="margin: 2px"
                >
                  {{ item.label }}
                </n-tag>
              </template>
              <template v-else>
                <n-text depth="3">{{ $gettext('Not installed') }}</n-text>
              </template>
            </td>
            <td>
              <n-tag
                :type="
                  JSON.stringify(localEnv?.[envType] || []) ===
                  JSON.stringify(remoteEnv?.[envType] || [])
                    ? 'success'
                    : 'warning'
                "
                size="small"
              >
                {{
                  JSON.stringify(localEnv?.[envType] || []) ===
                  JSON.stringify(remoteEnv?.[envType] || [])
                    ? $gettext('Match')
                    : $gettext('Different')
                }}
              </n-tag>
            </td>
          </tr>
          <tr>
            <td>{{ $gettext('Database') }}</td>
            <td>
              <template v-if="(localEnv?.db || []).filter((d: any) => d.value !== '0').length > 0">
                <n-tag
                  v-for="item in localEnv.db.filter((d: any) => d.value !== '0')"
                  :key="item.value"
                  size="small"
                  style="margin: 2px"
                >
                  {{ item.label }}
                </n-tag>
              </template>
              <template v-else>
                <n-text depth="3">{{ $gettext('None') }}</n-text>
              </template>
            </td>
            <td>
              <template v-if="(remoteEnv?.db || []).filter((d: any) => d.value !== '0').length > 0">
                <n-tag
                  v-for="item in remoteEnv.db.filter((d: any) => d.value !== '0')"
                  :key="item.value"
                  size="small"
                  style="margin: 2px"
                >
                  {{ item.label }}
                </n-tag>
              </template>
              <template v-else>
                <n-text depth="3">{{ $gettext('None') }}</n-text>
              </template>
            </td>
            <td>
              <n-tag :type="'info'" size="small">-</n-tag>
            </td>
          </tr>
        </tbody>
      </n-table>

      <n-flex justify="space-between" style="margin-top: 16px">
        <n-flex>
          <n-button @click="currentStep = 1">{{ $gettext('Previous') }}</n-button>
          <n-button :loading="loading" :disabled="loading" @click="handleRefreshPreCheck">
            {{ $gettext('Refresh') }}
          </n-button>
        </n-flex>
        <n-button
          type="primary"
          :disabled="!envCheckPassed || loading"
          :loading="loading"
          @click="handleGetItems"
        >
          {{ $gettext('Next') }}
        </n-button>
      </n-flex>
    </n-card>

    <!-- 第三步：选择迁移项 -->
    <n-card v-if="currentStep === 3" :title="$gettext('Select Migration Items')">
      <!-- 网站 -->
      <n-card :title="$gettext('Websites')" size="small" embedded style="margin-bottom: 12px">
        <template v-if="websites.length > 0">
          <n-checkbox-group v-model:value="selectedWebsites">
            <n-flex vertical>
              <n-checkbox v-for="(site, index) in websites" :key="site.id" :value="index">
                {{ site.name }}
                <n-text depth="3" style="margin-left: 8px">{{ site.path }}</n-text>
              </n-checkbox>
            </n-flex>
          </n-checkbox-group>
        </template>
        <n-empty v-else :description="$gettext('No websites found')" />
      </n-card>

      <!-- 数据库 -->
      <n-card :title="$gettext('Databases')" size="small" embedded style="margin-bottom: 12px">
        <template v-if="databases.length > 0">
          <n-checkbox-group v-model:value="selectedDatabases">
            <n-flex vertical>
              <n-checkbox v-for="(db, index) in databases" :key="db.name" :value="String(index)">
                {{ db.name }}
                <n-tag size="small" style="margin-left: 8px">{{ db.type }}</n-tag>
                <n-text depth="3" style="margin-left: 8px">{{ db.server }}</n-text>
              </n-checkbox>
            </n-flex>
          </n-checkbox-group>
        </template>
        <n-empty v-else :description="$gettext('No databases found')" />
      </n-card>

      <!-- 项目 -->
      <n-card :title="$gettext('Projects')" size="small" embedded style="margin-bottom: 12px">
        <template v-if="projects.length > 0">
          <n-checkbox-group v-model:value="selectedProjects">
            <n-flex vertical>
              <n-checkbox v-for="(proj, index) in projects" :key="proj.id" :value="index">
                {{ proj.name }}
                <n-tag size="small" style="margin-left: 8px">{{ proj.type }}</n-tag>
                <n-text depth="3" style="margin-left: 8px">{{ proj.root_dir || proj.path }}</n-text>
              </n-checkbox>
            </n-flex>
          </n-checkbox-group>
        </template>
        <n-empty v-else :description="$gettext('No projects found')" />
      </n-card>

      <!-- 选项 -->
      <n-card size="small" embedded style="margin-bottom: 12px">
        <n-checkbox v-model:checked="stopOnMig">
          {{ $gettext('Stop services during migration to ensure data consistency (recommended)') }}
        </n-checkbox>
      </n-card>

      <n-flex justify="space-between">
        <n-button @click="currentStep = 2">{{ $gettext('Previous') }}</n-button>
        <n-button
          type="primary"
          :loading="loading"
          :disabled="loading"
          @click="handleStartMigration"
        >
          {{ $gettext('Start Migration') }}
        </n-button>
      </n-flex>
    </n-card>

    <!-- 第四步：迁移进度 -->
    <n-card v-if="currentStep === 4" :title="$gettext('Migration Progress')">
      <!-- 迁移项状态 -->
      <n-table :bordered="true" :single-line="false" size="small" style="margin-bottom: 12px">
        <thead>
          <tr>
            <th>{{ $gettext('Type') }}</th>
            <th>{{ $gettext('Name') }}</th>
            <th>{{ $gettext('Status') }}</th>
            <th>{{ $gettext('Duration') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(result, index) in migrationResults" :key="index">
            <td>
              <n-tag size="small">{{ result.type }}</n-tag>
            </td>
            <td>{{ result.name }}</td>
            <td>
              <n-tag :type="getStatusType(result.status)" size="small">
                {{ result.status }}
              </n-tag>
            </td>
            <td>{{ result.duration ? formatDuration(result.duration) : '-' }}</td>
          </tr>
        </tbody>
      </n-table>

      <!-- 实时日志 -->
      <n-card :title="$gettext('Migration Logs')" size="small" embedded>
        <template #header-extra>
          <n-button
            size="small"
            :disabled="migrationLogs.length === 0"
            tag="a"
            :href="migration.logUrl"
            target="_blank"
          >
            {{ $gettext('Download Log') }}
          </n-button>
        </template>
        <div
          ref="logContainer"
          style="
            height: 400px;
            overflow-y: auto;
            font-family: monospace;
            font-size: 13px;
            line-height: 1.6;
            background: var(--n-color);
            padding: 8px;
            border-radius: 4px;
          "
        >
          <div
            v-for="(log, index) in migrationLogs"
            :key="index"
            style="white-space: pre-wrap; word-break: break-all"
          >
            {{ log }}
          </div>
          <div v-if="migrationRunning" style="color: var(--n-text-color-3)">
            {{ $gettext('Migration in progress...') }}
          </div>
        </div>
      </n-card>
    </n-card>

    <!-- 第五步：迁移完成 -->
    <n-card v-if="currentStep === 5" :title="$gettext('Migration Complete')">
      <n-result
        :status="migrationResults.every((r: any) => r.status === 'success') ? 'success' : 'warning'"
        :title="
          migrationResults.every((r: any) => r.status === 'success')
            ? $gettext('All items migrated successfully')
            : $gettext('Migration completed with some issues')
        "
      >
        <template #footer>
          <n-flex vertical>
            <n-text v-if="migrationStartedAt && migrationEndedAt">
              {{ $gettext('Started') }}: {{ new Date(migrationStartedAt).toLocaleString() }}
              &nbsp;|&nbsp;
              {{ $gettext('Ended') }}: {{ new Date(migrationEndedAt).toLocaleString() }}
            </n-text>
          </n-flex>
        </template>
      </n-result>

      <!-- 详细结果表 -->
      <n-table :bordered="true" :single-line="false" size="small" style="margin-top: 16px">
        <thead>
          <tr>
            <th>{{ $gettext('Type') }}</th>
            <th>{{ $gettext('Name') }}</th>
            <th>{{ $gettext('Status') }}</th>
            <th>{{ $gettext('Duration') }}</th>
            <th>{{ $gettext('Details') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(result, index) in migrationResults" :key="index">
            <td>
              <n-tag size="small">{{ result.type }}</n-tag>
            </td>
            <td>{{ result.name }}</td>
            <td>
              <n-tag :type="getStatusType(result.status)" size="small">
                {{ result.status }}
              </n-tag>
            </td>
            <td>{{ result.duration ? formatDuration(result.duration) : '-' }}</td>
            <td>
              <n-text v-if="result.error" type="error">{{ result.error }}</n-text>
              <n-text v-else-if="result.status === 'success' && result.ended_at" type="success">
                {{ $gettext('Migration succeeded') }} -
                {{ new Date(result.ended_at).toLocaleString() }}
              </n-text>
              <n-text v-else-if="result.ended_at">
                {{ new Date(result.ended_at).toLocaleString() }}
              </n-text>
              <n-text v-else>-</n-text>
            </td>
          </tr>
        </tbody>
      </n-table>

      <!-- 环境差异提醒 -->
      <n-alert
        v-if="envWarnings.length > 0"
        type="warning"
        :title="$gettext('Reminder')"
        style="margin-top: 16px"
      >
        {{
          $gettext(
            'Some environments differ between local and remote servers. You may need to adjust settings on the remote server otherwise related items may not work properly after migration.'
          )
        }}
      </n-alert>

      <n-flex justify="center" style="margin-top: 16px">
        <n-button tag="a" :href="migration.logUrl" target="_blank">
          {{ $gettext('Download Log') }}
        </n-button>
        <n-button type="primary" @click="handleReset">
          {{ $gettext('Start New Migration') }}
        </n-button>
      </n-flex>
    </n-card>
  </n-flex>
</template>

<style scoped lang="scss"></style>
