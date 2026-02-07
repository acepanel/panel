<script setup lang="ts">
defineOptions({
  name: 'php-config-tune'
})

import { useGettext } from 'vue3-gettext'

import php from '@/api/panel/environment/php'

const props = defineProps<{
  slug: number
}>()

const { $gettext } = useGettext()
const currentTab = ref('general')

// 常规设置
const shortOpenTag = ref('')
const dateTimezone = ref('')
const displayErrors = ref('')
const errorReporting = ref('')

// 禁用函数
const disableFunctions = ref('')

// 上传限制
const uploadMaxFilesize = ref('')
const postMaxSize = ref('')
const maxFileUploads = ref('')
const memoryLimit = ref('')

// 超时限制
const maxExecutionTime = ref('')
const maxInputTime = ref('')
const maxInputVars = ref('')

// Session
const sessionSaveHandler = ref('')
const sessionSavePath = ref('')
const sessionGcMaxlifetime = ref('')
const sessionCookieLifetime = ref('')

// Redis/Memcached 可视化字段
const sessionRedisHost = ref('127.0.0.1')
const sessionRedisPort = ref('6379')
const sessionRedisPassword = ref('')
const sessionMemcachedHost = ref('127.0.0.1')
const sessionMemcachedPort = ref('11211')

// 性能调整
const pm = ref('')
const pmMaxChildren = ref('')
const pmStartServers = ref('')
const pmMinSpareServers = ref('')
const pmMaxSpareServers = ref('')

// loading 状态
const saveLoading = ref(false)
const cleanSessionLoading = ref(false)

// 解析 Redis save_path 为可视化字段
const parseRedisSavePath = (path: string) => {
  // 格式: tcp://host:port?auth=password
  try {
    const url = new URL(path)
    sessionRedisHost.value = url.hostname || '127.0.0.1'
    sessionRedisPort.value = url.port || '6379'
    sessionRedisPassword.value = url.searchParams.get('auth') || ''
  } catch {
    sessionRedisHost.value = '127.0.0.1'
    sessionRedisPort.value = '6379'
    sessionRedisPassword.value = ''
  }
}

// 解析 Memcached save_path 为可视化字段
const parseMemcachedSavePath = (path: string) => {
  // 格式: host:port
  const parts = path.split(':')
  sessionMemcachedHost.value = parts[0] || '127.0.0.1'
  sessionMemcachedPort.value = parts[1] || '11211'
}

// 组合 Redis save_path
const composeRedisSavePath = () => {
  const host = sessionRedisHost.value || '127.0.0.1'
  const port = sessionRedisPort.value || '6379'
  const password = sessionRedisPassword.value
  if (password) {
    return `tcp://${host}:${port}?auth=${password}`
  }
  return `tcp://${host}:${port}`
}

// 组合 Memcached save_path
const composeMemcachedSavePath = () => {
  const host = sessionMemcachedHost.value || '127.0.0.1'
  const port = sessionMemcachedPort.value || '11211'
  return `${host}:${port}`
}

// 加载配置
useRequest(php.configTune(props.slug)).onSuccess(({ data }) => {
  shortOpenTag.value = data.short_open_tag ?? ''
  dateTimezone.value = data.date_timezone ?? ''
  displayErrors.value = data.display_errors ?? ''
  errorReporting.value = data.error_reporting ?? ''
  disableFunctions.value = data.disable_functions ?? ''
  uploadMaxFilesize.value = data.upload_max_filesize ?? ''
  postMaxSize.value = data.post_max_size ?? ''
  maxFileUploads.value = data.max_file_uploads ?? ''
  memoryLimit.value = data.memory_limit ?? ''
  maxExecutionTime.value = data.max_execution_time ?? ''
  maxInputTime.value = data.max_input_time ?? ''
  maxInputVars.value = data.max_input_vars ?? ''
  sessionSaveHandler.value = data.session_save_handler ?? 'files'
  sessionSavePath.value = data.session_save_path ?? ''
  sessionGcMaxlifetime.value = data.session_gc_maxlifetime ?? ''
  sessionCookieLifetime.value = data.session_cookie_lifetime ?? ''
  pm.value = data.pm ?? 'dynamic'
  pmMaxChildren.value = data.pm_max_children ?? ''
  pmStartServers.value = data.pm_start_servers ?? ''
  pmMinSpareServers.value = data.pm_min_spare_servers ?? ''
  pmMaxSpareServers.value = data.pm_max_spare_servers ?? ''

  // 解析 save_path 到可视化字段
  if (sessionSaveHandler.value === 'redis' && sessionSavePath.value) {
    parseRedisSavePath(sessionSavePath.value)
  } else if (sessionSaveHandler.value === 'memcached' && sessionSavePath.value) {
    parseMemcachedSavePath(sessionSavePath.value)
  }
})

// 获取当前配置数据
const getConfigData = () => {
  // 根据 handler 类型组合 save_path
  let savePath = sessionSavePath.value
  if (sessionSaveHandler.value === 'redis') {
    savePath = composeRedisSavePath()
  } else if (sessionSaveHandler.value === 'memcached') {
    savePath = composeMemcachedSavePath()
  }

  return {
    short_open_tag: shortOpenTag.value,
    date_timezone: dateTimezone.value,
    display_errors: displayErrors.value,
    error_reporting: errorReporting.value,
    disable_functions: disableFunctions.value,
    upload_max_filesize: uploadMaxFilesize.value,
    post_max_size: postMaxSize.value,
    max_file_uploads: maxFileUploads.value,
    memory_limit: memoryLimit.value,
    max_execution_time: maxExecutionTime.value,
    max_input_time: maxInputTime.value,
    max_input_vars: maxInputVars.value,
    session_save_handler: sessionSaveHandler.value,
    session_save_path: savePath,
    session_gc_maxlifetime: sessionGcMaxlifetime.value,
    session_cookie_lifetime: sessionCookieLifetime.value,
    pm: pm.value,
    pm_max_children: pmMaxChildren.value,
    pm_start_servers: pmStartServers.value,
    pm_min_spare_servers: pmMinSpareServers.value,
    pm_max_spare_servers: pmMaxSpareServers.value
  }
}

// 保存配置
const handleSave = () => {
  saveLoading.value = true
  useRequest(php.saveConfigTune(props.slug, getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}

// 清理 Session 文件
const handleCleanSession = () => {
  cleanSessionLoading.value = true
  useRequest(php.cleanSession(props.slug))
    .onSuccess(() => {
      window.$message.success($gettext('Cleaned successfully'))
    })
    .onComplete(() => {
      cleanSessionLoading.value = false
    })
}

// Session save_handler 选项
const sessionHandlerOptions = [
  { label: 'files', value: 'files' },
  { label: 'redis', value: 'redis' },
  { label: 'memcached', value: 'memcached' }
]

// PM 模式选项
const pmOptions = [
  { label: 'dynamic', value: 'dynamic' },
  { label: 'static', value: 'static' },
  { label: 'ondemand', value: 'ondemand' }
]

// short_open_tag / display_errors 选项
const onOffOptions = [
  { label: 'On', value: 'On' },
  { label: 'Off', value: 'Off' }
]
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="general" :tab="$gettext('General')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Common PHP general settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="short_open_tag">
            <n-select v-model:value="shortOpenTag" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="date.timezone">
            <n-input
              v-model:value="dateTimezone"
              :placeholder="$gettext('e.g. Asia/Shanghai')"
            />
          </n-form-item>
          <n-form-item label="display_errors">
            <n-select v-model:value="displayErrors" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="error_reporting">
            <n-input
              v-model:value="errorReporting"
              :placeholder="$gettext('e.g. E_ALL')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="disabled_functions" :tab="$gettext('Disabled Functions')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Enter the PHP functions to disable, separated by commas. Common dangerous functions include: exec, shell_exec, system, passthru, proc_open, popen, etc.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Disabled Functions')">
            <n-input
              v-model:value="disableFunctions"
              type="textarea"
              :rows="8"
              :placeholder="$gettext('e.g. exec,shell_exec,system,passthru')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="upload" :tab="$gettext('Upload Limits')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Adjust PHP file upload limits. post_max_size should be greater than upload_max_filesize.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="upload_max_filesize">
            <n-input
              v-model:value="uploadMaxFilesize"
              :placeholder="$gettext('e.g. 50M')"
            />
          </n-form-item>
          <n-form-item label="post_max_size">
            <n-input
              v-model:value="postMaxSize"
              :placeholder="$gettext('e.g. 50M')"
            />
          </n-form-item>
          <n-form-item label="max_file_uploads">
            <n-input
              v-model:value="maxFileUploads"
              :placeholder="$gettext('e.g. 20')"
            />
          </n-form-item>
          <n-form-item label="memory_limit">
            <n-input
              v-model:value="memoryLimit"
              :placeholder="$gettext('e.g. 256M')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="timeout" :tab="$gettext('Timeout Limits')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Adjust PHP script timeout limits. Values are in seconds, -1 means no limit.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="max_execution_time">
            <n-input
              v-model:value="maxExecutionTime"
              :placeholder="$gettext('e.g. 30')"
            />
          </n-form-item>
          <n-form-item label="max_input_time">
            <n-input
              v-model:value="maxInputTime"
              :placeholder="$gettext('e.g. 60')"
            />
          </n-form-item>
          <n-form-item label="max_input_vars">
            <n-input
              v-model:value="maxInputVars"
              :placeholder="$gettext('e.g. 1000')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="performance" :tab="$gettext('Performance Tuning')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Adjust PHP-FPM process manager settings. These settings are in php-fpm.conf.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="pm">
            <n-select v-model:value="pm" :options="pmOptions" />
          </n-form-item>
          <n-form-item label="pm.max_children">
            <n-input
              v-model:value="pmMaxChildren"
              :placeholder="$gettext('e.g. 30')"
            />
          </n-form-item>
          <n-form-item
            v-if="pm === 'dynamic'"
            label="pm.start_servers"
          >
            <n-input
              v-model:value="pmStartServers"
              :placeholder="$gettext('e.g. 5')"
            />
          </n-form-item>
          <n-form-item
            v-if="pm === 'dynamic'"
            label="pm.min_spare_servers"
          >
            <n-input
              v-model:value="pmMinSpareServers"
              :placeholder="$gettext('e.g. 3')"
            />
          </n-form-item>
          <n-form-item
            v-if="pm === 'dynamic'"
            label="pm.max_spare_servers"
          >
            <n-input
              v-model:value="pmMaxSpareServers"
              :placeholder="$gettext('e.g. 10')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="session" tab="Session">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Adjust PHP session settings. When using redis or memcached, make sure the corresponding extension is installed and the service is running.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="session.save_handler">
            <n-select
              v-model:value="sessionSaveHandler"
              :options="sessionHandlerOptions"
            />
          </n-form-item>
          <!-- files 模式：显示路径 -->
          <n-form-item
            v-if="sessionSaveHandler === 'files'"
            label="session.save_path"
          >
            <n-input
              v-model:value="sessionSavePath"
              :placeholder="$gettext('e.g. /tmp')"
            />
          </n-form-item>
          <!-- redis 模式：显示主机、端口、密码 -->
          <template v-if="sessionSaveHandler === 'redis'">
            <n-form-item :label="$gettext('Redis Host')">
              <n-input
                v-model:value="sessionRedisHost"
                placeholder="127.0.0.1"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Redis Port')">
              <n-input
                v-model:value="sessionRedisPort"
                placeholder="6379"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Redis Password')">
              <n-input
                v-model:value="sessionRedisPassword"
                type="password"
                show-password-on="click"
                :placeholder="$gettext('Leave empty if no password')"
              />
            </n-form-item>
          </template>
          <!-- memcached 模式：显示主机、端口 -->
          <template v-if="sessionSaveHandler === 'memcached'">
            <n-form-item :label="$gettext('Memcached Host')">
              <n-input
                v-model:value="sessionMemcachedHost"
                placeholder="127.0.0.1"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Memcached Port')">
              <n-input
                v-model:value="sessionMemcachedPort"
                placeholder="11211"
              />
            </n-form-item>
          </template>
          <n-form-item label="session.gc_maxlifetime">
            <n-input
              v-model:value="sessionGcMaxlifetime"
              :placeholder="$gettext('e.g. 1440 (seconds)')"
            />
          </n-form-item>
          <n-form-item label="session.cookie_lifetime">
            <n-input
              v-model:value="sessionCookieLifetime"
              :placeholder="$gettext('e.g. 0 (until browser closes)')"
            />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
          <n-popconfirm
            v-if="sessionSaveHandler === 'files'"
            @positive-click="handleCleanSession"
          >
            <template #trigger>
              <n-button
                type="warning"
                :loading="cleanSessionLoading"
                :disabled="cleanSessionLoading"
              >
                {{ $gettext('Clean Session Files') }}
              </n-button>
            </template>
            {{ $gettext('Are you sure you want to clean all session files?') }}
          </n-popconfirm>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>
