<script setup lang="ts">
defineOptions({
  name: 'php-config-tune',
})

import { useGettext } from 'vue3-gettext'

import php from '@/api/panel/environment/php'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { decodeNumber, decodeSize, encodeNumber, encodeSize, unitOptions } from '@/utils/tune'
import { phpPreset, type TuneProfile } from '@/utils/tunepreset'

const props = defineProps<{
  slug: number
}>()

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const [DefineNumber, ReuseNumber] = createReusableTemplate<{
  name: string
  placeholder: string
  suffix?: string
}>()
const [DefineSize, ReuseSize] = createReusableTemplate<{ name: string; placeholder: string }>()
const currentTab = ref('general')

// 常规设置
const shortOpenTag = ref<string | null>(null)
const dateTimezone = ref('')
const displayErrors = ref<string | null>(null)
const errorReporting = ref('')

// 禁用函数
const disableFunctions = ref('')

// 数字项与容量项，键同请求体，值为控件下限
const numberMin: Record<string, number> = {
  max_file_uploads: 0,
  max_execution_time: -1,
  max_input_time: -1,
  max_input_vars: 0,
  session_gc_maxlifetime: 0,
  session_cookie_lifetime: 0,
  pm_max_children: 1,
  pm_start_servers: 1,
  pm_min_spare_servers: 1,
  pm_max_spare_servers: 1,
  opcache_memory_consumption: 8,
  opcache_interned_strings_buffer: 0,
  opcache_max_accelerated_files: 200,
  opcache_revalidate_freq: 0,
}
const sizeMin: Record<string, number> = {
  upload_max_filesize: 0,
  post_max_size: 0,
  memory_limit: -1,
  opcache_jit_buffer_size: 0,
}

// 数字项与容量项的控件值，控件表示不了的值存的是原文字符串，改用文本框编辑
const decodeFields = (data: Record<string, any>) => ({
  ...Object.fromEntries(
    Object.entries(numberMin).map(([key, min]) => [key, decodeNumber(data[key], min)]),
  ),
  ...Object.fromEntries(
    Object.entries(sizeMin).map(([key, min]) => [key, decodeSize(data[key], 'M', min)]),
  ),
})
const model = ref<Record<string, any>>(decodeFields({}))

// Session
const sessionSaveHandler = ref<string | null>(null)
const sessionSavePath = ref('')
// 可视化字段拼不回原样的 save_path（多节点、unix socket、带额外参数等）改为直接编辑原文
const sessionPathRaw = ref(false)

// Redis/Memcached 可视化字段
const sessionRedisHost = ref('127.0.0.1')
const sessionRedisPort = ref('6379')
const sessionRedisPassword = ref('')
const sessionMemcachedHost = ref('127.0.0.1')
const sessionMemcachedPort = ref('11211')

// 性能调整
const pm = ref<string | null>(null)

// OPcache
const opcacheJit = ref<string | null>(null)

// loading 状态
const saveLoading = ref(false)
const showPresetModal = ref(false)
const presetScenarios = computed(() => [
  { label: $gettext('WordPress / general (~50MB per process)'), value: '50' },
  { label: $gettext('Framework application (~80MB per process)'), value: '80' },
  { label: $gettext('Heavy application (~128MB per process)'), value: '128' },
])

const handlePreset = (profile: TuneProfile) => {
  const r = phpPreset(profile)
  pm.value = r.pm
  Object.assign(model.value, {
    pm_max_children: r.pmMaxChildren,
    pm_start_servers: r.pmStartServers,
    pm_min_spare_servers: r.pmMinSpareServers,
    pm_max_spare_servers: r.pmMaxSpareServers,
    memory_limit: { num: r.memoryLimitM, unit: 'M' },
    opcache_memory_consumption: r.opcacheMemoryConsumption,
    opcache_interned_strings_buffer: r.opcacheInternedStringsBuffer,
    opcache_max_accelerated_files: r.opcacheMaxAcceleratedFiles,
    opcache_revalidate_freq: r.opcacheRevalidateFreq,
  })
  // JIT 从 PHP 8.0 开始才有
  if (props.slug >= 80) {
    opcacheJit.value = r.opcacheJit
    model.value.opcache_jit_buffer_size = { num: r.opcacheJitBufferSizeM, unit: 'M' }
  }
  window.$message.success($gettext('Generated, review the values and save'))
}

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
    // phpredis 按查询串解码 auth，特殊字符不转义会被截断或改写
    return `tcp://${host}:${port}?auth=${encodeURIComponent(password)}`
  }
  return `tcp://${host}:${port}`
}

// 组合 Memcached save_path
const composeMemcachedSavePath = () => {
  const host = sessionMemcachedHost.value || '127.0.0.1'
  const port = sessionMemcachedPort.value || '11211'
  return `${host}:${port}`
}

// 当前 handler 下由可视化字段拼出的 save_path，没有可视化字段的 handler 就是原文
const composeSavePath = () => {
  if (sessionSaveHandler.value === 'redis') return composeRedisSavePath()
  if (sessionSaveHandler.value === 'memcached') return composeMemcachedSavePath()
  return sessionSavePath.value
}

// 加载配置
useRequest(php.configTune(props.slug)).onSuccess(({ data }) => {
  shortOpenTag.value = data.short_open_tag || null
  dateTimezone.value = data.date_timezone ?? ''
  displayErrors.value = data.display_errors || null
  errorReporting.value = data.error_reporting ?? ''
  disableFunctions.value = data.disable_functions ?? ''
  model.value = decodeFields(data)
  sessionSaveHandler.value = data.session_save_handler || null
  sessionSavePath.value = data.session_save_path ?? ''
  pm.value = data.pm || null
  opcacheJit.value = data.opcache_jit || null

  // 解析 save_path 到可视化字段
  if (sessionSaveHandler.value === 'redis' && sessionSavePath.value) {
    parseRedisSavePath(sessionSavePath.value)
  } else if (sessionSaveHandler.value === 'memcached' && sessionSavePath.value) {
    parseMemcachedSavePath(sessionSavePath.value)
  }
  sessionPathRaw.value = composeSavePath() !== sessionSavePath.value
})

// 获取当前配置数据
const getConfigData = () => ({
  short_open_tag: shortOpenTag.value ?? '',
  date_timezone: dateTimezone.value,
  display_errors: displayErrors.value ?? '',
  error_reporting: errorReporting.value,
  disable_functions: disableFunctions.value,
  ...Object.fromEntries(Object.keys(numberMin).map((key) => [key, encodeNumber(model.value[key])])),
  ...Object.fromEntries(Object.keys(sizeMin).map((key) => [key, encodeSize(model.value[key])])),
  session_save_handler: sessionSaveHandler.value ?? '',
  session_save_path: sessionPathRaw.value ? sessionSavePath.value : composeSavePath(),
  pm: pm.value ?? '',
  opcache_jit: opcacheJit.value ?? '',
})

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
const handleCleanSession = async () => {
  await php.cleanSession(props.slug)
  window.$message.success($gettext('Cleaned successfully'))
}

// Session save_handler 选项
const sessionHandlerOptions = [
  { label: 'files', value: 'files' },
  { label: 'redis', value: 'redis' },
  { label: 'memcached', value: 'memcached' },
]

// PM 模式选项
const pmOptions = [
  { label: 'dynamic', value: 'dynamic' },
  { label: 'static', value: 'static' },
  { label: 'ondemand', value: 'ondemand' },
]

// short_open_tag / display_errors 选项
const onOffOptions = [
  { label: 'On', value: 'On' },
  { label: 'Off', value: 'Off' },
]

const jitOptions = computed(() => [
  { label: $gettext('Disabled'), value: 'disable' },
  { label: 'tracing', value: 'tracing' },
  { label: 'function', value: 'function' },
])

// opcache.jit 的数字写法和 on/off 都有等价的选项，旧版模块脚本写的是 1205
const jitAliases: Record<string, string> = {
  on: 'tracing',
  off: 'disable',
  '1254': 'tracing',
  '1205': 'function',
}

// 等价写法按对应选项的名字显示，值保持原样，不改动就不会被改写
const jitFallback = (value: string | number) => ({
  label:
    jitOptions.value.find((option) => option.value === jitAliases[value])?.label ?? String(value),
  value,
})

const sizeUnitOptions = unitOptions(['K', 'M', 'G'])
</script>

<template>
  <n-flex vertical>
    <DefineNumber v-slot="{ name, placeholder, suffix }">
      <n-input
        v-if="typeof model[name] === 'string'"
        v-model:value="model[name]"
        :placeholder="placeholder"
      />
      <n-input-number
        v-else
        class="w-full"
        v-model:value="model[name]"
        :placeholder="placeholder"
        :min="numberMin[name]"
      >
        <template v-if="suffix" #suffix>{{ suffix }}</template>
      </n-input-number>
    </DefineNumber>
    <DefineSize v-slot="{ name, placeholder }">
      <n-input
        v-if="typeof model[name] === 'string'"
        v-model:value="model[name]"
        :placeholder="placeholder"
      />
      <n-input-group v-else>
        <n-input-number
          class="w-full"
          v-model:value="model[name].num"
          :placeholder="placeholder"
          :min="sizeMin[name]"
          style="flex: 1"
        />
        <n-select v-model:value="model[name].unit" :options="sizeUnitOptions" class="w-20" />
      </n-input-group>
    </DefineSize>
    <n-tabs v-model:value="currentTab" type="line" :placement="isMobile ? 'top' : 'left'" animated>
      <n-tab-pane name="general" :tab="$gettext('General')">
        <n-flex vertical>
          <n-alert type="info">
            {{ $gettext('Common PHP general settings.') }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Short Tag (short_open_tag)')">
              <n-select v-model:value="shortOpenTag" :options="onOffOptions" clearable />
            </n-form-item>
            <n-form-item :label="$gettext('Timezone (date.timezone)')">
              <n-input v-model:value="dateTimezone" :placeholder="$gettext('e.g. Asia/Shanghai')" />
            </n-form-item>
            <n-form-item :label="$gettext('Display Errors (display_errors)')">
              <n-select v-model:value="displayErrors" :options="onOffOptions" clearable />
            </n-form-item>
            <n-form-item :label="$gettext('Error Reporting (error_reporting)')">
              <n-input v-model:value="errorReporting" :placeholder="$gettext('e.g. E_ALL')" />
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="disabled_functions" :tab="$gettext('Disabled Functions')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Enter the PHP functions to disable, separated by commas. Common dangerous functions include: exec, shell_exec, system, passthru, proc_open, popen, etc.',
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="upload" :tab="$gettext('Upload Limits')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Adjust PHP file upload limits. post_max_size should be greater than upload_max_filesize.',
              )
            }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Max Upload Size (upload_max_filesize)')">
              <ReuseSize name="upload_max_filesize" :placeholder="$gettext('e.g. 50')" />
            </n-form-item>
            <n-form-item :label="$gettext('Max POST Size (post_max_size)')">
              <ReuseSize name="post_max_size" :placeholder="$gettext('e.g. 50')" />
            </n-form-item>
            <n-form-item :label="$gettext('Max File Uploads (max_file_uploads)')">
              <ReuseNumber name="max_file_uploads" :placeholder="$gettext('e.g. 20')" />
            </n-form-item>
            <n-form-item :label="$gettext('Memory Limit (memory_limit)')">
              <ReuseSize name="memory_limit" :placeholder="$gettext('e.g. 256')" />
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="timeout" :tab="$gettext('Timeout Limits')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Adjust PHP script timeout limits. Values are in seconds, -1 means no limit.',
              )
            }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Max Execution Time (max_execution_time)')">
              <ReuseNumber name="max_execution_time" :placeholder="$gettext('e.g. 30')" />
            </n-form-item>
            <n-form-item :label="$gettext('Max Input Time (max_input_time)')">
              <ReuseNumber name="max_input_time" :placeholder="$gettext('e.g. 60')" />
            </n-form-item>
            <n-form-item :label="$gettext('Max Input Vars (max_input_vars)')">
              <ReuseNumber name="max_input_vars" :placeholder="$gettext('e.g. 1000')" />
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="performance" :tab="$gettext('Performance Tuning')">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Adjust PHP-FPM process manager settings. These settings are in php-fpm.conf.',
              )
            }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Process Manager (pm)')">
              <n-select v-model:value="pm" :options="pmOptions" />
            </n-form-item>
            <n-form-item :label="$gettext('Max Children (pm.max_children)')">
              <ReuseNumber name="pm_max_children" :placeholder="$gettext('e.g. 30')" />
            </n-form-item>
            <n-form-item
              v-if="pm === 'dynamic'"
              :label="$gettext('Start Servers (pm.start_servers)')"
            >
              <ReuseNumber name="pm_start_servers" :placeholder="$gettext('e.g. 5')" />
            </n-form-item>
            <n-form-item
              v-if="pm === 'dynamic'"
              :label="$gettext('Min Spare Servers (pm.min_spare_servers)')"
            >
              <ReuseNumber name="pm_min_spare_servers" :placeholder="$gettext('e.g. 3')" />
            </n-form-item>
            <n-form-item
              v-if="pm === 'dynamic'"
              :label="$gettext('Max Spare Servers (pm.max_spare_servers)')"
            >
              <ReuseNumber name="pm_max_spare_servers" :placeholder="$gettext('e.g. 10')" />
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="opcache" tab="OPcache">
        <n-flex vertical>
          <n-alert type="info">
            {{ $gettext('Adjust OPcache and JIT settings. The tracing JIT mode is recommended.') }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Memory Size (opcache.memory_consumption)')">
              <ReuseNumber
                name="opcache_memory_consumption"
                :placeholder="$gettext('e.g. 256')"
                suffix="MB"
              />
            </n-form-item>
            <n-form-item
              :label="$gettext('Interned Strings Buffer (opcache.interned_strings_buffer)')"
            >
              <ReuseNumber
                name="opcache_interned_strings_buffer"
                :placeholder="$gettext('e.g. 32')"
                suffix="MB"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Max Cached Files (opcache.max_accelerated_files)')">
              <ReuseNumber
                name="opcache_max_accelerated_files"
                :placeholder="$gettext('e.g. 100000')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Revalidate Frequency (opcache.revalidate_freq)')">
              <ReuseNumber
                name="opcache_revalidate_freq"
                :placeholder="$gettext('e.g. 3 (seconds)')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('JIT Mode (opcache.jit)')">
              <n-select
                v-model:value="opcacheJit"
                :options="jitOptions"
                :fallback-option="jitFallback"
                clearable
              />
            </n-form-item>
            <n-form-item :label="$gettext('JIT Buffer Size (opcache.jit_buffer_size)')">
              <ReuseSize name="opcache_jit_buffer_size" :placeholder="$gettext('e.g. 128')" />
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="session" tab="Session">
        <n-flex vertical>
          <n-alert type="info">
            {{
              $gettext(
                'Adjust PHP session settings. When using redis or memcached, make sure the corresponding extension is installed and the service is running.',
              )
            }}
          </n-alert>
          <n-form>
            <n-form-item :label="$gettext('Save Handler (session.save_handler)')">
              <n-select
                v-model:value="sessionSaveHandler"
                :options="sessionHandlerOptions"
                @update:value="sessionPathRaw = false"
              />
            </n-form-item>
            <!-- redis 模式：显示主机、端口、密码 -->
            <template v-if="sessionSaveHandler === 'redis' && !sessionPathRaw">
              <n-form-item :label="$gettext('Redis Host')">
                <n-input v-model:value="sessionRedisHost" placeholder="127.0.0.1" />
              </n-form-item>
              <n-form-item :label="$gettext('Redis Port')">
                <n-input v-model:value="sessionRedisPort" placeholder="6379" />
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
            <template v-else-if="sessionSaveHandler === 'memcached' && !sessionPathRaw">
              <n-form-item :label="$gettext('Memcached Host')">
                <n-input v-model:value="sessionMemcachedHost" placeholder="127.0.0.1" />
              </n-form-item>
              <n-form-item :label="$gettext('Memcached Port')">
                <n-input v-model:value="sessionMemcachedPort" placeholder="11211" />
              </n-form-item>
            </template>
            <!-- 其余情况直接编辑路径原文 -->
            <n-form-item v-else :label="$gettext('Save Path (session.save_path)')">
              <n-input v-model:value="sessionSavePath" :placeholder="$gettext('e.g. /tmp')" />
            </n-form-item>
            <n-form-item :label="$gettext('GC Max Lifetime (session.gc_maxlifetime)')">
              <ReuseNumber
                name="session_gc_maxlifetime"
                :placeholder="$gettext('e.g. 1440 (seconds)')"
              />
            </n-form-item>
            <n-form-item :label="$gettext('Cookie Lifetime (session.cookie_lifetime)')">
              <ReuseNumber
                name="session_cookie_lifetime"
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
            <n-button type="info" @click="showPresetModal = true">
              {{ $gettext('Generate Recommended Configuration') }}
            </n-button>
            <ConfirmDialog
              v-if="sessionSaveHandler === 'files'"
              type="danger"
              :content="$gettext('Are you sure you want to clean all session files?')"
              @confirm="handleCleanSession"
            >
              <template #trigger>
                <app-button type="warning">
                  {{ $gettext('Clean Session Files') }}
                </app-button>
              </template>
            </ConfirmDialog>
          </n-flex>
        </n-flex>
      </n-tab-pane>
    </n-tabs>
    <tune-preset-modal
      v-model:show="showPresetModal"
      :fields="['memory']"
      :scenario-options="presetScenarios"
      :scenario-label="$gettext('Application Profile')"
      :memory-ratio="0.5"
      @generate="handlePreset"
    />
  </n-flex>
</template>
