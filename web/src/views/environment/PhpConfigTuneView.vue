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
const currentTab = ref('disabled_functions')

// 配置数据
const disableFunctions = ref('')
const uploadMaxFilesize = ref('')
const postMaxSize = ref('')
const maxExecutionTime = ref('')
const maxInputTime = ref('')
const memoryLimit = ref('')
const maxInputVars = ref('')
const maxFileUploads = ref('')
const sessionSaveHandler = ref('')
const sessionSavePath = ref('')
const pm = ref('')
const pmMaxChildren = ref('')
const pmStartServers = ref('')
const pmMinSpareServers = ref('')
const pmMaxSpareServers = ref('')

// loading 状态
const saveLoading = ref(false)
const cleanSessionLoading = ref(false)

// 加载配置
useRequest(php.configTune(props.slug)).onSuccess(({ data }) => {
  disableFunctions.value = data.disable_functions ?? ''
  uploadMaxFilesize.value = data.upload_max_filesize ?? ''
  postMaxSize.value = data.post_max_size ?? ''
  maxExecutionTime.value = data.max_execution_time ?? ''
  maxInputTime.value = data.max_input_time ?? ''
  memoryLimit.value = data.memory_limit ?? ''
  maxInputVars.value = data.max_input_vars ?? ''
  maxFileUploads.value = data.max_file_uploads ?? ''
  sessionSaveHandler.value = data.session_save_handler ?? 'files'
  sessionSavePath.value = data.session_save_path ?? ''
  pm.value = data.pm ?? 'dynamic'
  pmMaxChildren.value = data.pm_max_children ?? ''
  pmStartServers.value = data.pm_start_servers ?? ''
  pmMinSpareServers.value = data.pm_min_spare_servers ?? ''
  pmMaxSpareServers.value = data.pm_max_spare_servers ?? ''
})

// 获取当前配置数据
const getConfigData = () => ({
  disable_functions: disableFunctions.value,
  upload_max_filesize: uploadMaxFilesize.value,
  post_max_size: postMaxSize.value,
  max_execution_time: maxExecutionTime.value,
  max_input_time: maxInputTime.value,
  memory_limit: memoryLimit.value,
  max_input_vars: maxInputVars.value,
  max_file_uploads: maxFileUploads.value,
  session_save_handler: sessionSaveHandler.value,
  session_save_path: sessionSavePath.value,
  pm: pm.value,
  pm_max_children: pmMaxChildren.value,
  pm_start_servers: pmStartServers.value,
  pm_min_spare_servers: pmMinSpareServers.value,
  pm_max_spare_servers: pmMaxSpareServers.value
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
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
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
          <n-form-item label="session.save_path">
            <n-input
              v-model:value="sessionSavePath"
              :placeholder="
                sessionSaveHandler === 'redis'
                  ? $gettext('e.g. tcp://127.0.0.1:6379')
                  : sessionSaveHandler === 'memcached'
                    ? $gettext('e.g. 127.0.0.1:11211')
                    : $gettext('e.g. /tmp')
              "
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
