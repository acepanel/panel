<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import project from '@/api/panel/project'
import PathSelector from '@/components/common/PathSelector.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const editId = defineModel<number>('editId', { type: Number, required: true })

const { $gettext } = useGettext()

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
  read_only_paths: [] as string[]
})

const loading = ref(false)

// Restart 策略选项
const restartOptions = [
  { label: $gettext('No restart'), value: 'no' },
  { label: $gettext('Always restart'), value: 'always' },
  { label: $gettext('Restart on failure'), value: 'on-failure' },
  { label: $gettext('Restart on abnormal'), value: 'on-abnormal' },
  { label: $gettext('Restart on abort'), value: 'on-abort' },
  { label: $gettext('Restart on success'), value: 'on-success' }
]

// 输出选项
const outputOptions = [
  { label: 'journal', value: 'journal' },
  { label: 'syslog', value: 'syslog' },
  { label: 'kmsg', value: 'kmsg' },
  { label: 'null', value: 'null' },
  { label: $gettext('File (append)'), value: 'append:/var/log/' },
  { label: $gettext('File (truncate)'), value: 'truncate:/var/log/' }
]

// ProtectSystem 选项
const protectSystemOptions = [
  { label: $gettext('Disabled'), value: '' },
  { label: 'true', value: 'true' },
  { label: 'full', value: 'full' },
  { label: 'strict', value: 'strict' }
]

// 目录选择器
const showPathSelector = ref(false)
const pathSelectorPath = ref('')
const pathSelectorTarget = ref<'root_dir' | 'working_dir'>('root_dir')

const handleSelectPath = (target: 'root_dir' | 'working_dir') => {
  pathSelectorTarget.value = target
  pathSelectorPath.value = model.value[target] || '/opt/ace/projects'
  showPathSelector.value = true
}

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    model.value[pathSelectorTarget.value] = pathSelectorPath.value
  }
})

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
        read_only_paths: data.read_only_paths || []
      }
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
  }
})

// 环境变量操作
const onCreateEnv = () => {
  return { key: '', value: '' }
}

// 保存
const handleSave = async () => {
  useRequest(project.update(model.value.id, model.value)).onSuccess(() => {
    window.$bus.emit('project:refresh')
    window.$message.success($gettext('Saved successfully'))
    show.value = false
  })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    :title="$gettext('Edit Project - %{ name }', { name: model.name })"
    preset="card"
    style="width: 70vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-spin :show="loading">
      <n-tabs v-model:value="currentTab" type="line" animated>
        <!-- 基本设置 -->
        <n-tab-pane name="basic" :tab="$gettext('Basic Settings')">
          <n-form :model="model" label-placement="left" label-width="120">
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
                  { label: 'nobody', value: 'nobody' }
                ]"
                :placeholder="$gettext('Select User')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 运行设置 -->
        <n-tab-pane name="runtime" :tab="$gettext('Runtime Settings')">
          <n-form :model="model" label-placement="left" label-width="140">
            <n-form-item path="exec_start" :label="$gettext('Start Command')">
              <n-input
                v-model:value="model.exec_start"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('e.g., php artisan serve, node app.js')"
              />
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

            <n-row :gutter="[24, 0]">
              <n-col :span="12">
                <n-form-item path="restart" :label="$gettext('Restart Strategy')">
                  <n-select
                    v-model:value="model.restart"
                    :options="restartOptions"
                    @keydown.enter.prevent
                  />
                </n-form-item>
              </n-col>
              <n-col :span="12">
                <n-form-item path="restart_sec" :label="$gettext('Restart Interval')">
                  <n-input
                    v-model:value="model.restart_sec"
                    type="text"
                    @keydown.enter.prevent
                    :placeholder="$gettext('e.g., 5s, 1min')"
                  />
                </n-form-item>
              </n-col>
            </n-row>
            <n-row :gutter="[24, 0]">
              <n-col :span="8">
                <n-form-item path="restart_max" :label="$gettext('Max Restarts')">
                  <n-input-number
                    v-model:value="model.restart_max"
                    :min="0"
                    :max="100"
                    style="width: 100%"
                  />
                </n-form-item>
              </n-col>
              <n-col :span="8">
                <n-form-item path="timeout_start_sec" :label="$gettext('Start Timeout (s)')">
                  <n-input-number
                    v-model:value="model.timeout_start_sec"
                    :min="0"
                    :max="3600"
                    style="width: 100%"
                  />
                </n-form-item>
              </n-col>
              <n-col :span="8">
                <n-form-item path="timeout_stop_sec" :label="$gettext('Stop Timeout (s)')">
                  <n-input-number
                    v-model:value="model.timeout_stop_sec"
                    :min="0"
                    :max="3600"
                    style="width: 100%"
                  />
                </n-form-item>
              </n-col>
            </n-row>

            <n-divider title-placement="left">{{ $gettext('Other') }}</n-divider>

            <n-row :gutter="[24, 0]">
              <n-col :span="12">
                <n-form-item path="standard_output" :label="$gettext('Standard Output')">
                  <n-select
                    v-model:value="model.standard_output"
                    :options="outputOptions"
                    tag
                    filterable
                    @keydown.enter.prevent
                  />
                </n-form-item>
              </n-col>
              <n-col :span="12">
                <n-form-item path="standard_error" :label="$gettext('Standard Error')">
                  <n-select
                    v-model:value="model.standard_error"
                    :options="outputOptions"
                    tag
                    filterable
                    @keydown.enter.prevent
                  />
                </n-form-item>
              </n-col>
            </n-row>
            <n-form-item :label="$gettext('Environment Variables')">
              <n-dynamic-input
                v-model:value="model.environments"
                :on-create="onCreateEnv"
                show-sort-button
              >
                <template #default="{ value }">
                  <div flex gap-2 w-full items-center>
                    <n-input
                      v-model:value="value.key"
                      :placeholder="$gettext('Variable name')"
                      style="flex: 1"
                    />
                    <span>=</span>
                    <n-input
                      v-model:value="value.value"
                      :placeholder="$gettext('Variable value')"
                      style="flex: 2"
                    />
                  </div>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 依赖设置 -->
        <n-tab-pane name="dependencies" :tab="$gettext('Dependencies')">
          <n-form :model="model" label-placement="left" label-width="140">
            <n-alert type="info" style="margin-bottom: 16px">
              {{
                $gettext(
                  'Configure service dependencies to control startup order. Common services: network.target, mysql.service, redis.service'
                )
              }}
            </n-alert>

            <n-form-item path="requires" :label="$gettext('Requires')">
              <n-dynamic-tags v-model:value="model.requires" />
              <template #feedback>
                <span class="text-gray-400">
                  {{
                    $gettext('Strong dependencies, service will fail if these are not available')
                  }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="wants" :label="$gettext('Wants')">
              <n-dynamic-tags v-model:value="model.wants" />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Weak dependencies, service will still start if these fail') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="after" :label="$gettext('After')">
              <n-dynamic-tags v-model:value="model.after" />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Start this service after the specified services') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="before" :label="$gettext('Before')">
              <n-dynamic-tags v-model:value="model.before" />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Start this service before the specified services') }}
                </span>
              </template>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 资源限制 -->
        <n-tab-pane name="resources" :tab="$gettext('Resource Limits')">
          <n-form :model="model" label-placement="left" label-width="140">
            <n-alert type="info" style="margin-bottom: 16px">
              {{
                $gettext(
                  'Set resource limits to prevent the service from consuming too many system resources'
                )
              }}
            </n-alert>

            <n-row :gutter="[24, 0]">
              <n-col :span="12">
                <n-form-item path="memory_limit" :label="$gettext('Memory Limit (MB)')">
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
                </n-form-item>
              </n-col>
              <n-col :span="12">
                <n-form-item path="cpu_quota" :label="$gettext('CPU Quota')">
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
                </n-form-item>
              </n-col>
            </n-row>
          </n-form>
        </n-tab-pane>

        <!-- 安全设置 -->
        <n-tab-pane name="security" :tab="$gettext('Security Settings')">
          <n-form :model="model" label-placement="left" label-width="160">
            <n-alert type="warning" style="margin-bottom: 16px">
              {{
                $gettext(
                  'Security settings can enhance service isolation but may affect functionality. Please test thoroughly before enabling.'
                )
              }}
            </n-alert>

            <n-divider title-placement="left">{{ $gettext('Privilege Control') }}</n-divider>

            <n-row :gutter="[24, 0]">
              <n-col :span="8">
                <n-form-item path="no_new_privileges" :label="$gettext('No New Privileges')">
                  <n-switch v-model:value="model.no_new_privileges" />
                </n-form-item>
              </n-col>
              <n-col :span="8">
                <n-form-item path="protect_tmp" :label="$gettext('Protect /tmp')">
                  <n-switch v-model:value="model.protect_tmp" />
                </n-form-item>
              </n-col>
              <n-col :span="8">
                <n-form-item path="protect_home" :label="$gettext('Protect /home')">
                  <n-switch v-model:value="model.protect_home" />
                </n-form-item>
              </n-col>
            </n-row>

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
                      'true: /usr, /boot read-only; full: + /etc read-only; strict: entire filesystem read-only'
                    )
                  }}
                </span>
              </template>
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Path Access Control') }}</n-divider>

            <n-form-item path="read_write_paths" :label="$gettext('Read-Write Paths')">
              <n-dynamic-tags v-model:value="model.read_write_paths" />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Paths that the service can read and write to') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="read_only_paths" :label="$gettext('Read-Only Paths')">
              <n-dynamic-tags v-model:value="model.read_only_paths" />
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
        <n-button type="primary" @click="handleSave" :loading="loading">
          {{ $gettext('Save') }}
        </n-button>
      </n-flex>
    </template>
  </n-modal>

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
