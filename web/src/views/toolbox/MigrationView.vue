<script setup lang="ts">
defineOptions({
  name: 'toolbox-migration'
})

import dashboard from '@/api/panel/dashboard'
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
const databaseUsers = ref<any[]>([])
const selectedWebsites = ref<number[]>([])
const selectedDatabases = ref<string[]>([])
const selectedDatabaseUsers = ref<number[]>([])
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
    window.$message.error($gettext('请填写完整的连接信息'))
    return
  }

  loading.value = true
  useRequest(migration.precheck(connectionForm.value))
    .onSuccess(({ data }: any) => {
      remoteEnv.value = data.remote
      // 同时获取本地环境信息
      useRequest(dashboard.installedDbAndPhp()).onSuccess(({ data: localData }: any) => {
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
    window.$message.error($gettext('请填写完整的连接信息'))
    return
  }

  loading.value = true
  useRequest(migration.precheck(connectionForm.value))
    .onSuccess(({ data }: any) => {
      remoteEnv.value = data.remote
      useRequest(dashboard.installedDbAndPhp()).onSuccess(({ data: localData }: any) => {
        localEnv.value = localData
        checkEnvironment()
        loading.value = false
        window.$message.success($gettext('环境检查已刷新'))
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
        'Web 服务器类型不匹配。本地: %{local}，远程: %{remote}。相关网站可能无法正常工作。',
        {
          local: localEnv.value.webserver || $gettext('none'),
          remote: remoteEnv.value.webserver || $gettext('none')
        }
      )
    )
    passed = false
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
        $gettext('%{type} 已安装在本地但未安装在远程服务器。相关数据库可能无法正常工作。', {
          type: dbType.toUpperCase()
        })
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
      databaseUsers.value = data.database_users || []
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
    database_users: databaseUsers.value
      .filter((_: any, i: number) => selectedDatabaseUsers.value.includes(i))
      .map((u: any) => ({
        id: u.id,
        username: u.username,
        password: u.password,
        host: u.host,
        server_id: u.server_id,
        server: u.server?.name,
        type: u.server?.type
      })),
    stop_on_mig: stopOnMig.value
  }

  if (
    selectedItems.websites.length === 0 &&
    selectedItems.databases.length === 0 &&
    selectedItems.database_users.length === 0
  ) {
    window.$message.warning($gettext('请至少选择一个迁移项'))
    return
  }

  window.$dialog.warning({
    title: $gettext('确认迁移'),
    content: $gettext('你确定要开始迁移吗？这将把所选项迁移到远程服务器。'),
    positiveText: $gettext('开始'),
    negativeText: $gettext('取消'),
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
    title: $gettext('重置迁移状态'),
    content: $gettext('你确定要重置迁移状态吗？这将清除所有迁移记录并允许你开始新的迁移。'),
    positiveText: $gettext('确认'),
    negativeText: $gettext('取消'),
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
        databaseUsers.value = []
        selectedWebsites.value = []
        selectedDatabases.value = []
        selectedDatabaseUsers.value = []
        migrationLogs.value = []
        migrationResults.value = []
        migrationStartedAt.value = null
        migrationEndedAt.value = null
        window.$message.success($gettext('迁移状态已重置'))
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
  <common-page show-footer>
    <n-flex vertical>
      <!-- 步骤指示器 -->
      <n-card>
        <n-steps :current="currentStep" size="small">
          <n-step title="连接" description="输入远程服务器信息" />
          <n-step title="预检查" description="验证环境" />
          <n-step title="选择项目" description="选择要迁移的内容" />
          <n-step title="迁移中" description="正在传输" />
          <n-step title="完成" description="查看结果" />
        </n-steps>
      </n-card>

      <!-- 第一步：连接信息 -->
      <n-card v-if="currentStep === 1" title="远程服务器连接">
        <n-form label-placement="left" label-width="auto">
          <n-form-item label="面板地址">
            <n-input
              v-model:value="connectionForm.url"
              placeholder="例如 https://remote-server:8888"
            />
          </n-form-item>
          <n-form-item label="令牌 ID">
            <n-input-number
              v-model:value="connectionForm.token_id"
              placeholder="API 令牌 ID"
              :min="1"
              style="width: 100%"
            />
          </n-form-item>
          <n-form-item label="访问令牌">
            <n-input
              v-model:value="connectionForm.token"
              type="password"
              show-password-on="click"
              placeholder="API 访问令牌"
            />
          </n-form-item>
        </n-form>
        <n-flex justify="end">
          <n-button type="primary" :loading="loading" :disabled="loading" @click="handlePreCheck">
            下一步
          </n-button>
        </n-flex>
      </n-card>

      <!-- 第二步：环境预检查 -->
      <n-card v-if="currentStep === 2" title="环境预检查">
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
              <th>环境</th>
              <th>本地</th>
              <th>远程</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Web 服务器</td>
              <td>{{ localEnv?.webserver || '无' }}</td>
              <td>{{ remoteEnv?.webserver || '无' }}</td>
              <td>
                <n-tag
                  :type="localEnv?.webserver === remoteEnv?.webserver ? 'success' : 'error'"
                  size="small"
                >
                  {{ localEnv?.webserver === remoteEnv?.webserver ? '一致' : '不一致' }}
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
                  <n-text depth="3">未安装</n-text>
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
                  <n-text depth="3">未安装</n-text>
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
                      ? '一致'
                      : '有差异'
                  }}
                </n-tag>
              </td>
            </tr>
            <tr>
              <td>数据库</td>
              <td>
                <template
                  v-if="(localEnv?.db || []).filter((d: any) => d.value !== '0').length > 0"
                >
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
                  <n-text depth="3">无</n-text>
                </template>
              </td>
              <td>
                <template
                  v-if="(remoteEnv?.db || []).filter((d: any) => d.value !== '0').length > 0"
                >
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
                  <n-text depth="3">无</n-text>
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
            <n-button @click="currentStep = 1">上一步</n-button>
            <n-button :loading="loading" :disabled="loading" @click="handleRefreshPreCheck">
              刷新
            </n-button>
          </n-flex>
          <n-button
            type="primary"
            :disabled="!envCheckPassed || loading"
            :loading="loading"
            @click="handleGetItems"
          >
            下一步
          </n-button>
        </n-flex>
      </n-card>

      <!-- 第三步：选择迁移项 -->
      <n-card v-if="currentStep === 3" title="选择迁移项目">
        <!-- 网站 -->
        <n-card title="网站" size="small" embedded style="margin-bottom: 12px">
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
          <n-empty v-else description="未找到网站" />
        </n-card>

        <!-- 数据库 -->
        <n-card title="数据库" size="small" embedded style="margin-bottom: 12px">
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
          <n-empty v-else description="未找到数据库" />
        </n-card>

        <!-- 数据库用户 -->
        <n-card title="数据库用户" size="small" embedded style="margin-bottom: 12px">
          <template v-if="databaseUsers.length > 0">
            <n-checkbox-group v-model:value="selectedDatabaseUsers">
              <n-flex vertical>
                <n-checkbox v-for="(user, index) in databaseUsers" :key="user.id" :value="index">
                  {{ user.username }}
                  <n-text v-if="user.host" depth="3" style="margin-left: 4px"
                    >@{{ user.host }}</n-text
                  >
                  <n-tag size="small" style="margin-left: 8px">{{ user.server?.type }}</n-tag>
                  <n-text depth="3" style="margin-left: 8px">{{ user.server?.name }}</n-text>
                </n-checkbox>
              </n-flex>
            </n-checkbox-group>
          </template>
          <n-empty v-else description="未找到数据库用户" />
        </n-card>

        <!-- 选项 -->
        <n-card size="small" embedded style="margin-bottom: 12px">
          <n-checkbox v-model:checked="stopOnMig">
            迁移期间停止服务以确保数据一致性（推荐）
          </n-checkbox>
        </n-card>

        <n-flex justify="space-between">
          <n-button @click="currentStep = 2">上一步</n-button>
          <n-button
            type="primary"
            :loading="loading"
            :disabled="loading"
            @click="handleStartMigration"
          >
            开始迁移
          </n-button>
        </n-flex>
      </n-card>

      <!-- 第四步：迁移进度 -->
      <n-card v-if="currentStep === 4" title="迁移进度">
        <!-- 迁移项状态 -->
        <n-table :bordered="true" :single-line="false" size="small" style="margin-bottom: 12px">
          <thead>
            <tr>
              <th>类型</th>
              <th>名称</th>
              <th>状态</th>
              <th>耗时</th>
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
        <n-card title="迁移日志" size="small" embedded>
          <template #header-extra>
            <n-button
              size="small"
              :disabled="migrationLogs.length === 0"
              tag="a"
              :href="migration.logUrl"
              target="_blank"
            >
              下载日志
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
            <div v-if="migrationRunning" style="color: var(--n-text-color-3)">迁移进行中...</div>
          </div>
        </n-card>
      </n-card>

      <!-- 第五步：迁移完成 -->
      <n-card v-if="currentStep === 5" title="迁移完成">
        <n-result
          :status="
            migrationResults.every((r: any) => r.status === 'success') ? 'success' : 'warning'
          "
          :title="
            migrationResults.every((r: any) => r.status === 'success')
              ? '所有项目已成功迁移'
              : '迁移已完成，但存在部分问题'
          "
        >
          <template #footer>
            <n-flex vertical>
              <n-text v-if="migrationStartedAt && migrationEndedAt">
                开始：{{ new Date(migrationStartedAt).toLocaleString() }} &nbsp;|&nbsp; 结束：{{
                  new Date(migrationEndedAt).toLocaleString()
                }}
              </n-text>
            </n-flex>
          </template>
        </n-result>

        <!-- 详细结果表 -->
        <n-table :bordered="true" :single-line="false" size="small" style="margin-top: 16px">
          <thead>
            <tr>
              <th>类型</th>
              <th>名称</th>
              <th>状态</th>
              <th>耗时</th>
              <th>详情</th>
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
                  迁移成功 - {{ new Date(result.ended_at).toLocaleString() }}
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
        <n-alert v-if="envWarnings.length > 0" type="warning" title="提醒" style="margin-top: 16px">
          本地与远程服务器存在部分环境差异。你可能需要在远程服务器上调整相关设置，否则迁移后相关项目可能无法正常运行。
        </n-alert>

        <n-flex justify="center" style="margin-top: 16px">
          <n-button tag="a" :href="migration.logUrl" target="_blank">下载日志</n-button>
          <n-button type="primary" @click="handleReset">开始新的迁移</n-button>
        </n-flex>
      </n-card>
    </n-flex>
  </common-page>
</template>
