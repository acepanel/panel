<script setup lang="ts">
defineOptions({
  name: 'nginx-config-tune'
})

import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  api: any
}>()

const { $gettext } = useGettext()
const currentTab = ref('general')

// 常规设置
const workerProcesses = ref('')
const workerConnections = ref('')
const keepaliveTimeout = ref('')
const clientMaxBodySize = ref('')
const clientBodyBufferSize = ref('')
const clientHeaderBufferSize = ref('')
const serverNamesHashBucketSize = ref('')
const serverTokens = ref('')

// Gzip
const gzip = ref('')
const gzipMinLength = ref('')
const gzipCompLevel = ref<number | null>(null)
const gzipTypes = ref('')
const gzipVary = ref('')
const gzipProxied = ref('')

// Brotli
const brotli = ref('')
const brotliMinLength = ref('')
const brotliCompLevel = ref<number | null>(null)
const brotliTypes = ref('')
const brotliStatic = ref('')

// Zstd
const zstd = ref('')
const zstdMinLength = ref('')
const zstdCompLevel = ref<number | null>(null)
const zstdTypes = ref('')
const zstdStatic = ref('')

const saveLoading = ref(false)

const onOffOptions = [
  { label: 'on', value: 'on' },
  { label: 'off', value: 'off' }
]

const onOffAlwaysOptions = [
  { label: 'on', value: 'on' },
  { label: 'off', value: 'off' },
  { label: 'always', value: 'always' }
]

useRequest(props.api.configTune()).onSuccess(({ data }: any) => {
  workerProcesses.value = data.worker_processes ?? ''
  workerConnections.value = data.worker_connections ?? ''
  keepaliveTimeout.value = data.keepalive_timeout ?? ''
  clientMaxBodySize.value = data.client_max_body_size ?? ''
  clientBodyBufferSize.value = data.client_body_buffer_size ?? ''
  clientHeaderBufferSize.value = data.client_header_buffer_size ?? ''
  serverNamesHashBucketSize.value = data.server_names_hash_bucket_size ?? ''
  serverTokens.value = data.server_tokens ?? ''
  gzip.value = data.gzip ?? ''
  gzipMinLength.value = data.gzip_min_length ?? ''
  gzipCompLevel.value = Number(data.gzip_comp_level) || null
  gzipTypes.value = data.gzip_types ?? ''
  gzipVary.value = data.gzip_vary ?? ''
  gzipProxied.value = data.gzip_proxied ?? ''
  brotli.value = data.brotli ?? ''
  brotliMinLength.value = data.brotli_min_length ?? ''
  brotliCompLevel.value = Number(data.brotli_comp_level) || null
  brotliTypes.value = data.brotli_types ?? ''
  brotliStatic.value = data.brotli_static ?? ''
  zstd.value = data.zstd ?? ''
  zstdMinLength.value = data.zstd_min_length ?? ''
  zstdCompLevel.value = Number(data.zstd_comp_level) || null
  zstdTypes.value = data.zstd_types ?? ''
  zstdStatic.value = data.zstd_static ?? ''
})

const getConfigData = () => ({
  worker_processes: workerProcesses.value,
  worker_connections: workerConnections.value,
  keepalive_timeout: keepaliveTimeout.value,
  client_max_body_size: clientMaxBodySize.value,
  client_body_buffer_size: clientBodyBufferSize.value,
  client_header_buffer_size: clientHeaderBufferSize.value,
  server_names_hash_bucket_size: serverNamesHashBucketSize.value,
  server_tokens: serverTokens.value,
  gzip: gzip.value,
  gzip_min_length: gzipMinLength.value,
  gzip_comp_level: String(gzipCompLevel.value ?? ''),
  gzip_types: gzipTypes.value,
  gzip_vary: gzipVary.value,
  gzip_proxied: gzipProxied.value,
  brotli: brotli.value,
  brotli_min_length: brotliMinLength.value,
  brotli_comp_level: String(brotliCompLevel.value ?? ''),
  brotli_types: brotliTypes.value,
  brotli_static: brotliStatic.value,
  zstd: zstd.value,
  zstd_min_length: zstdMinLength.value,
  zstd_comp_level: String(zstdCompLevel.value ?? ''),
  zstd_types: zstdTypes.value,
  zstd_static: zstdStatic.value
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(props.api.saveConfigTune(getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="general" :tab="$gettext('General')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Common Nginx general settings.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Worker Processes (worker_processes)">
            <n-input
              v-model:value="workerProcesses"
              :placeholder="$gettext('e.g. auto or number')"
            />
          </n-form-item>
          <n-form-item label="Worker Connections (worker_connections)">
            <n-input v-model:value="workerConnections" :placeholder="$gettext('e.g. 65535')" />
          </n-form-item>
          <n-form-item label="Keepalive Timeout (keepalive_timeout)">
            <n-input v-model:value="keepaliveTimeout" :placeholder="$gettext('e.g. 60')" />
          </n-form-item>
          <n-form-item label="Client Max Body Size (client_max_body_size)">
            <n-input v-model:value="clientMaxBodySize" :placeholder="$gettext('e.g. 200m')" />
          </n-form-item>
          <n-form-item label="Client Body Buffer Size (client_body_buffer_size)">
            <n-input v-model:value="clientBodyBufferSize" :placeholder="$gettext('e.g. 10M')" />
          </n-form-item>
          <n-form-item label="Client Header Buffer Size (client_header_buffer_size)">
            <n-input v-model:value="clientHeaderBufferSize" :placeholder="$gettext('e.g. 32k')" />
          </n-form-item>
          <n-form-item label="Server Names Hash Bucket Size">
            <n-input
              v-model:value="serverNamesHashBucketSize"
              :placeholder="$gettext('e.g. 512')"
            />
          </n-form-item>
          <n-form-item label="Server Tokens (server_tokens)">
            <n-select v-model:value="serverTokens" :options="onOffOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="gzip" tab="Gzip">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Gzip compression settings. Gzip is the most widely supported compression method.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Gzip (gzip)">
            <n-select v-model:value="gzip" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (gzip_min_length)">
            <n-input v-model:value="gzipMinLength" :placeholder="$gettext('e.g. 1k')" />
          </n-form-item>
          <n-form-item label="Compression Level (gzip_comp_level)">
            <n-input-number class="w-full" v-model:value="gzipCompLevel" :min="1" :max="9" />
          </n-form-item>
          <n-form-item label="Types (gzip_types)">
            <n-input v-model:value="gzipTypes" :placeholder="$gettext('e.g. *')" />
          </n-form-item>
          <n-form-item label="Vary (gzip_vary)">
            <n-select v-model:value="gzipVary" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Proxied (gzip_proxied)">
            <n-input v-model:value="gzipProxied" :placeholder="$gettext('e.g. any')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="brotli" tab="Brotli">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Brotli compression settings. Brotli provides better compression ratio than Gzip.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Brotli (brotli)">
            <n-select v-model:value="brotli" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (brotli_min_length)">
            <n-input v-model:value="brotliMinLength" :placeholder="$gettext('e.g. 1k')" />
          </n-form-item>
          <n-form-item label="Compression Level (brotli_comp_level)">
            <n-input-number class="w-full" v-model:value="brotliCompLevel" :min="0" :max="11" />
          </n-form-item>
          <n-form-item label="Types (brotli_types)">
            <n-input v-model:value="brotliTypes" :placeholder="$gettext('e.g. *')" />
          </n-form-item>
          <n-form-item label="Static (brotli_static)">
            <n-select v-model:value="brotliStatic" :options="onOffAlwaysOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="zstd" tab="Zstd">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Zstd compression settings. Zstd provides fast compression with high ratio.') }}
        </n-alert>
        <n-form>
          <n-form-item label="Zstd (zstd)">
            <n-select v-model:value="zstd" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (zstd_min_length)">
            <n-input v-model:value="zstdMinLength" :placeholder="$gettext('e.g. 1k')" />
          </n-form-item>
          <n-form-item label="Compression Level (zstd_comp_level)">
            <n-input-number class="w-full" v-model:value="zstdCompLevel" :min="1" :max="22" />
          </n-form-item>
          <n-form-item label="Types (zstd_types)">
            <n-input v-model:value="zstdTypes" :placeholder="$gettext('e.g. *')" />
          </n-form-item>
          <n-form-item label="Static (zstd_static)">
            <n-select v-model:value="zstdStatic" :options="onOffAlwaysOptions" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>
