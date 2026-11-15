<script setup lang="ts">
import type { SelectFilter, SelectOption, SelectRenderTag } from 'naive-ui'
import { NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import project from '@/api/panel/project'
import systemctl from '@/api/panel/systemctl'
import { useFormLayout } from '@/components/system/composables/useFormLayout'

import { buildCommand, currentRuntime } from './command'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })

const { $gettext } = useGettext()
const formLayout = useFormLayout(120)
const formLayoutWide = useFormLayout(140)
const securityFormLayout = useFormLayout(160)

const currentTab = ref('basic')

const model = ref({
  id: 0,
  name: '',
  description: '',
  root_dir: '',
  working_dir: '',
  exec_start_pre: '',
  exec_start_post: '',
  exec_start: '',
  exec_stop: '',
  exec_reload: '',
  user: 'www',
  restart: 'on-failure',
  restart_sec: '5s',
  restart_max: 3,
  timeout_start_sec: 90,
  timeout_stop_sec: 90,
  environments: [] as { key: string; value: string }[],
  standard_output: 'journal',
  standard_error: 'journal',
  requires: [] as string[],
  wants: [] as string[],
  after: [] as string[],
  before: [] as string[],
  memory_limit: 0,
  cpu_quota: '',
  no_new_privileges: false,
  protect_tmp: false,
  protect_home: false,
  protect_system: '',
  read_write_paths: [] as string[],
  read_only_paths: [] as string[],
})

const loading = ref(false)
const projectType = ref('')

// Restart 策略选项
const restartOptions = [
  { label: $gettext('No restart'), value: 'no' },
  { label: $gettext('Always restart'), value: 'always' },
  { label: $gettext('Restart on failure'), value: 'on-failure' },
  { label: $gettext('Restart on abnormal'), value: 'on-abnormal' },
  { label: $gettext('Restart on abort'), value: 'on-abort' },
  { label: $gettext('Restart on success'), value: 'on-success' },
]

// 输出选项
const outputOptions = [
  { label: 'journal', value: 'journal' },
  { label: 'syslog', value: 'syslog' },
  { label: 'kmsg', value: 'kmsg' },
  { label: 'null', value: 'null' },
  { label: $gettext('File (append)'), value: 'append:/var/log/' },
  { label: $gettext('File (truncate)'), value: 'truncate:/var/log/' },
]

// ProtectSystem 选项
const protectSystemOptions = [
  { label: $gettext('Disabled'), value: '' },
  { label: 'true', value: 'true' },
  { label: 'full', value: 'full' },
  { label: 'strict', value: 'strict' },
]

// 可被依赖的系统单元
const {
  data: units,
  loading: unitsLoading,
  send: loadUnits,
} = useRequest(systemctl.units, {
  immediate: false,
  initialData: [],
})

const unitOptions = computed(() =>
  (units.value as { name: string; description: string }[]).map((unit) => ({
    label: unit.name,
    value: unit.name,
    description: unit.description,
  })),
)

const filterUnit: SelectFilter = (pattern, option) =>
  [option.label, option.description].some((text) =>
    String(text).toLowerCase().includes(pattern.trim().toLowerCase()),
  )

const renderUnitLabel = (option: SelectOption) =>
  h('div', { class: 'flex items-center gap-3' }, [
    h('span', option.label as string),
    h('span', { class: 'text-xs text-gray-400 truncate' }, option.description as string),
  ])

// 不自定义的话已选标签会沿用 renderUnitLabel 连描述一起显示
const renderUnitTag: SelectRenderTag = ({ option, handleClose }) =>
  h(
    NTag,
    {
      closable: true,
      onMousedown: (e: MouseEvent) => e.preventDefault(),
      onClose: (e: MouseEvent) => {
        e.stopPropagation()
        handleClose()
      },
    },
    { default: () => option.label as string },
  )

const dependencyItems = [
  {
    key: 'requires',
    label: $gettext('Requires'),
    help: $gettext('Strong dependencies, service will fail if these are not available'),
  },
  {
    key: 'wants',
    label: $gettext('Wants'),
    help: $gettext('Weak dependencies, service will still start if these fail'),
  },
  {
    key: 'after',
    label: $gettext('After'),
    help: $gettext('Start this service after the specified services'),
  },
  {
    key: 'before',
    label: $gettext('Before'),
    help: $gettext('Start this service before the specified services'),
  },
] as const

// 与后端一致：工作目录留空时用项目目录，缺少前导 / 时补上
const workDir = computed(() => {
  const dir = model.value.working_dir.trim() || model.value.root_dir.trim()
  return dir.startsWith('/') ? dir : '/' + dir
})

// 目录选择器，exec_start 选的是启动命令要用的文件
type PathTarget = 'root_dir' | 'working_dir' | 'exec_start' | 'read_write_paths' | 'read_only_paths'
const showPathSelector = ref(false)
const pathSelectorPath = ref('')
const pathSelectorTarget = ref<PathTarget>('root_dir')

const handleSelectPath = (target: PathTarget) => {
  const current = model.value[target]
  pathSelectorTarget.value = target
  pathSelectorPath.value =
    target === 'exec_start'
      ? workDir.value
      : (Array.isArray(current) ? '' : current) || model.value.root_dir || '/opt/ace/projects'
  showPathSelector.value = true
}

const handlePathSelected = (path: string) => {
  const target = pathSelectorTarget.value
  if (target === 'exec_start') {
    const runtime = currentRuntime(projectType.value, model.value.exec_start)
    model.value.exec_start = buildCommand(projectType.value, runtime, path, workDir.value)
  } else if (target === 'root_dir' || target === 'working_dir') {
    model.value[target] = path
  } else if (!model.value[target].includes(path)) {
    model.value[target].push(path)
  }
}

// 加载项目数据
const loadProject = async () => {
  if (!editId.value) return
  loading.value = true
  useRequest(project.get(editId.value))
    .onSuccess(({ data }) => {
      model.value = {
        id: data.id,
        name: data.name || '',
        description: data.description || '',
        root_dir: data.root_dir || '',
        working_dir: data.working_dir || '',
        exec_start_pre: data.exec_start_pre || '',
        exec_start_post: data.exec_start_post || '',
        exec_start: data.exec_start || '',
        exec_stop: data.exec_stop || '',
        exec_reload: data.exec_reload || '',
        user: data.user || 'www',
        restart: data.restart || 'on-failure',
        restart_sec: data.restart_sec || '5s',
        restart_max: data.restart_max || 3,
        timeout_start_sec: data.timeout_start_sec || 90,
        timeout_stop_sec: data.timeout_stop_sec || 90,
        environments: data.environments || [],
        standard_output: data.standard_output || 'journal',
        standard_error: data.standard_error || 'journal',
        requires: data.requires || [],
        wants: data.wants || [],
        after: data.after || [],
        before: data.before || [],
        memory_limit: data.memory_limit || 0,
        cpu_quota: data.cpu_quota || '',
        no_new_privileges: data.no_new_privileges || false,
        protect_tmp: data.protect_tmp || false,
        protect_home: data.protect_home || false,
        protect_system: data.protect_system || '',
        read_write_paths: data.read_write_paths || [],
        read_only_paths: data.read_only_paths || [],
      }
      projectType.value = data.type
    })
    .onComplete(() => {
      loading.value = false
    })
}

// 监听 show 变化加载数据
watch(show, (val) => {
  if (val && editId.value) {
    currentTab.value = 'basic'
    loadProject()
    loadUnits()
  }
})

// 环境变量操作
const onCreateEnv = () => {
  return { key: '', value: '' }
}

// 保存
const handleSave = async () => {
  loading.value = true
  useRequest(project.update(model.value.id, model.value))
    .onSuccess(() => {
      window.$bus.emit('project:refresh')
      window.$message.success($gettext('Saved successfully'))
      show.value = false
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <app-modal
    v-model:show="show"
    size="lg"
    :title="$gettext('Edit Project - %{ name }', { name: model.name })"
  >
    <n-spin :show="loading">
      <n-tabs v-model:value="currentTab" type="line" animated>
        <!-- 基本设置 -->
        <n-tab-pane name="basic" :tab="$gettext('Basic Settings')">
          <n-form :model="model" v-bind="formLayout">
            <n-form-item path="name" :label="$gettext('Project Name')">
              <n-input
                v-model:value="model.name"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Project name, used as service identifier')"
              />
            </n-form-item>

            <n-form-item path="description" :label="$gettext('Description')">
              <n-input
                v-model:value="model.description"
                type="textarea"
                :rows="2"
                @keydown.enter.prevent
                :placeholder="$gettext('Project description')"
              />
            </n-form-item>

            <n-form-item path="root_dir" :label="$gettext('Project Directory')">
              <n-input-group>
                <n-input
                  v-model:value="model.root_dir"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('Project root directory')"
                />
                <n-button @click="handleSelectPath('root_dir')">
                  <template #icon>
                    <i-mdi-folder-open />
                  </template>
                </n-button>
              </n-input-group>
            </n-form-item>

            <n-form-item path="working_dir" :label="$gettext('Working Directory')">
              <n-input-group>
                <n-input
                  v-model:value="model.working_dir"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="
                    $gettext('Working directory (optional, defaults to project directory)')
                  "
                />
                <n-button @click="handleSelectPath('working_dir')">
                  <template #icon>
                    <i-mdi-folder-open />
                  </template>
                </n-button>
              </n-input-group>
            </n-form-item>

            <n-form-item path="user" :label="$gettext('Run User')">
              <n-select
                v-model:value="model.user"
                :options="[
                  { label: 'www', value: 'www' },
                  { label: 'root', value: 'root' },
                  { label: 'nobody', value: 'nobody' },
                ]"
                :placeholder="$gettext('Select or enter user')"
                filterable
                tag
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 运行设置 -->
        <n-tab-pane name="runtime" :tab="$gettext('Runtime Settings')">
          <n-form :model="model" v-bind="formLayoutWide">
            <n-form-item path="exec_start" :label="$gettext('Start Command')">
              <n-input-group>
                <n-input
                  v-model:value="model.exec_start"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('e.g., php artisan serve, node app.js')"
                />
                <n-button @click="handleSelectPath('exec_start')">
                  <template #icon>
                    <i-mdi-file-outline />
                  </template>
                </n-button>
              </n-input-group>
            </n-form-item>
            <n-form-item path="exec_start_pre" :label="$gettext('Pre-start Command')">
              <n-input
                v-model:value="model.exec_start_pre"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Command to run before starting (optional)')"
              />
            </n-form-item>
            <n-form-item path="exec_start_post" :label="$gettext('Post-start Command')">
              <n-input
                v-model:value="model.exec_start_post"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Command to run after starting (optional)')"
              />
            </n-form-item>
            <n-form-item path="exec_stop" :label="$gettext('Stop Command')">
              <n-input
                v-model:value="model.exec_stop"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Custom stop command (optional)')"
              />
            </n-form-item>
            <n-form-item path="exec_reload" :label="$gettext('Reload Command')">
              <n-input
                v-model:value="model.exec_reload"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Custom reload command (optional)')"
              />
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Restart Policy') }}</n-divider>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="12" path="restart" :label="$gettext('Restart Strategy')">
                <n-select
                  v-model:value="model.restart"
                  :options="restartOptions"
                  @keydown.enter.prevent
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" path="restart_sec" :label="$gettext('Restart Interval')">
                <n-input
                  v-model:value="model.restart_sec"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('e.g., 5s, 1min')"
                />
              </n-form-item-gi>
            </n-grid>
            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="8" path="restart_max" :label="$gettext('Max Restarts')">
                <n-input-number
                  v-model:value="model.restart_max"
                  :min="0"
                  :max="100"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi
                :span="8"
                path="timeout_start_sec"
                :label="$gettext('Start Timeout (s)')"
              >
                <n-input-number
                  v-model:value="model.timeout_start_sec"
                  :min="0"
                  :max="3600"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi
                :span="8"
                path="timeout_stop_sec"
                :label="$gettext('Stop Timeout (s)')"
              >
                <n-input-number
                  v-model:value="model.timeout_stop_sec"
                  :min="0"
                  :max="3600"
                  style="width: 100%"
                />
              </n-form-item-gi>
            </n-grid>

            <n-divider title-placement="left">{{ $gettext('Other') }}</n-divider>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi
                :span="12"
                path="standard_output"
                :label="$gettext('Standard Output')"
              >
                <n-select
                  v-model:value="model.standard_output"
                  :options="outputOptions"
                  tag
                  filterable
                  @keydown.enter.prevent
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" path="standard_error" :label="$gettext('Standard Error')">
                <n-select
                  v-model:value="model.standard_error"
                  :options="outputOptions"
                  tag
                  filterable
                  @keydown.enter.prevent
                />
              </n-form-item-gi>
            </n-grid>
            <n-form-item :label="$gettext('Environment Variables')">
              <n-dynamic-input
                v-model:value="model.environments"
                :on-create="onCreateEnv"
                show-sort-button
              >
                <template #default="{ value }">
                  <!-- 手机上输入框各占一行，桌面按 1:2 同行 -->
                  <div flex flex-wrap gap-2 w-full items-center>
                    <n-input
                      v-model:value="value.key"
                      :placeholder="$gettext('Variable name')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">=</span>
                    <n-input
                      v-model:value="value.value"
                      :placeholder="$gettext('Variable value')"
                      class="md:flex-[2]"
                    />
                  </div>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 依赖设置 -->
        <n-tab-pane name="dependencies" :tab="$gettext('Dependencies')">
          <n-form :model="model" v-bind="formLayoutWide">
            <n-alert type="info" class="mb-4">
              {{
                $gettext(
                  'Configure service dependencies to control startup order. Common services: network.target, mysqld.service, postgresql.service, redis.service',
                )
              }}
            </n-alert>

            <n-form-item
              v-for="item in dependencyItems"
              :key="item.key"
              :path="item.key"
              :label="item.label"
            >
              <n-select
                v-model:value="model[item.key]"
                :options="unitOptions"
                :loading="unitsLoading"
                :filter="filterUnit"
                :render-label="renderUnitLabel"
                :render-tag="renderUnitTag"
                :placeholder="$gettext('Search by name or description')"
                multiple
                filterable
              />
              <template #feedback>
                <span class="text-gray-400">{{ item.help }}</span>
              </template>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 资源限制 -->
        <n-tab-pane name="resources" :tab="$gettext('Resource Limits')">
          <n-form :model="model" v-bind="formLayoutWide">
            <n-alert type="info" class="mb-4">
              {{
                $gettext(
                  'Set resource limits to prevent the service from consuming too many system resources',
                )
              }}
            </n-alert>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="12" path="memory_limit" :label="$gettext('Memory Limit (MB)')">
                <n-input-number
                  v-model:value="model.memory_limit"
                  :min="0"
                  :max="1024000"
                  style="width: 100%"
                  :placeholder="$gettext('0 means no limit')"
                />
                <template #feedback>
                  <span class="text-gray-400">
                    {{ $gettext('Set to 0 to disable memory limit') }}
                  </span>
                </template>
              </n-form-item-gi>
              <n-form-item-gi :span="12" path="cpu_quota" :label="$gettext('CPU Quota')">
                <n-input
                  v-model:value="model.cpu_quota"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('e.g., 50% or 200%')"
                />
                <template #feedback>
                  <span class="text-gray-400">
                    {{ $gettext('100% = 1 CPU core, 200% = 2 cores') }}
                  </span>
                </template>
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>

        <!-- 安全设置 -->
        <n-tab-pane name="security" :tab="$gettext('Security Settings')">
          <n-form :model="model" v-bind="securityFormLayout">
            <n-alert type="warning" class="mb-4">
              {{
                $gettext(
                  'Security settings can enhance service isolation but may affect functionality. Please test thoroughly before enabling.',
                )
              }}
            </n-alert>

            <n-divider title-placement="left">{{ $gettext('Privilege Control') }}</n-divider>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi
                :span="8"
                path="no_new_privileges"
                :label="$gettext('No New Privileges')"
              >
                <n-switch v-model:value="model.no_new_privileges" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="protect_tmp" :label="$gettext('Protect /tmp')">
                <n-switch v-model:value="model.protect_tmp" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="protect_home" :label="$gettext('Protect /home')">
                <n-switch v-model:value="model.protect_home" />
              </n-form-item-gi>
            </n-grid>

            <n-form-item path="protect_system" :label="$gettext('Protect System')">
              <n-select
                v-model:value="model.protect_system"
                :options="protectSystemOptions"
                @keydown.enter.prevent
              />
              <template #feedback>
                <span class="text-gray-400">
                  {{
                    $gettext(
                      'true: /usr, /boot read-only; full: + /etc read-only; strict: entire filesystem read-only',
                    )
                  }}
                </span>
              </template>
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Path Access Control') }}</n-divider>

            <n-form-item path="read_write_paths" :label="$gettext('Read-Write Paths')">
              <n-dynamic-tags v-model:value="model.read_write_paths">
                <template #trigger>
                  <n-button size="small" dashed @click="handleSelectPath('read_write_paths')">
                    <template #icon>
                      <i-mdi-folder-plus-outline />
                    </template>
                    {{ $gettext('Select Directory') }}
                  </n-button>
                </template>
              </n-dynamic-tags>
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Paths that the service can read and write to') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="read_only_paths" :label="$gettext('Read-Only Paths')">
              <n-dynamic-tags v-model:value="model.read_only_paths">
                <template #trigger>
                  <n-button size="small" dashed @click="handleSelectPath('read_only_paths')">
                    <template #icon>
                      <i-mdi-folder-plus-outline />
                    </template>
                    {{ $gettext('Select Directory') }}
                  </n-button>
                </template>
              </n-dynamic-tags>
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Paths that the service can only read from') }}
                </span>
              </template>
            </n-form-item>
          </n-form>
        </n-tab-pane>
      </n-tabs>
    </n-spin>

    <template #footer>
      <n-flex justify="end">
        <n-button @click="show = false">
          {{ $gettext('Cancel') }}
        </n-button>
        <n-button type="primary" @click="handleSave" :loading="loading" :disabled="loading">
          {{ $gettext('Save') }}
        </n-button>
      </n-flex>
    </template>
  </app-modal>

  <!-- 目录选择器 -->
  <path-selector
    v-model:show="showPathSelector"
    v-model:path="pathSelectorPath"
    :dir="pathSelectorTarget !== 'exec_start'"
    @select="handlePathSelected"
  />
</template>

<style scoped lang="scss"></style>
