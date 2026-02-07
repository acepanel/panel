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
const workerConnections = ref<number | null>(null)
const keepaliveTimeout = ref<number | null>(null)
const clientMaxBodySizeNum = ref<number | null>(null)
const clientMaxBodySizeUnit = ref('m')
const clientBodyBufferSizeNum = ref<number | null>(null)
const clientBodyBufferSizeUnit = ref('k')
const clientHeaderBufferSizeNum = ref<number | null>(null)
const clientHeaderBufferSizeUnit = ref('k')
const serverNamesHashBucketSize = ref<number | null>(null)
const serverTokens = ref('')

// Gzip
const gzip = ref('')
const gzipMinLengthNum = ref<number | null>(null)
const gzipMinLengthUnit = ref('k')
const gzipCompLevel = ref<number | null>(null)
const gzipTypes = ref('')
const gzipVary = ref('')
const gzipProxied = ref('')

// Brotli
const brotli = ref('')
const brotliMinLengthNum = ref<number | null>(null)
const brotliMinLengthUnit = ref('k')
const brotliCompLevel = ref<number | null>(null)
const brotliTypes = ref('')
const brotliStatic = ref('')

// Zstd
const zstd = ref('')
const zstdMinLengthNum = ref<number | null>(null)
const zstdMinLengthUnit = ref('k')
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

// Nginx 容量单位选项（小写）
const sizeUnitOptions = [
  { label: 'k', value: 'k' },
  { label: 'm', value: 'm' },
  { label: 'g', value: 'g' }
]

// 解析带单位的值，如 "200m" -> { num: 200, unit: "m" }
const parseSizeValue = (val: string): { num: number | null; unit: string } => {
  if (!val) return { num: null, unit: 'k' }
  const match = val.match(/^(\d+)\s*([kmg])$/i)
  if (match) {
    return { num: Number(match[1]), unit: match[2].toLowerCase() }
  }
  return { num: Number(val) || null, unit: 'k' }
}

// 组合数值和单位
const composeSizeValue = (num: number | null, unit: string): string => {
  if (num == null) return ''
  return `${num}${unit}`
}

useRequest(props.api.configTune()).onSuccess(({ data }: any) => {
  workerProcesses.value = data.worker_processes ?? ''
  workerConnections.value = Number(data.worker_connections) || null
  keepaliveTimeout.value = Number(data.keepalive_timeout) || null
  const cmbs = parseSizeValue(data.client_max_body_size ?? '')
  clientMaxBodySizeNum.value = cmbs.num
  clientMaxBodySizeUnit.value = cmbs.unit
  const cbbs = parseSizeValue(data.client_body_buffer_size ?? '')
  clientBodyBufferSizeNum.value = cbbs.num
  clientBodyBufferSizeUnit.value = cbbs.unit
  const chbs = parseSizeValue(data.client_header_buffer_size ?? '')
  clientHeaderBufferSizeNum.value = chbs.num
  clientHeaderBufferSizeUnit.value = chbs.unit
  serverNamesHashBucketSize.value = Number(data.server_names_hash_bucket_size) || null
  serverTokens.value = data.server_tokens ?? ''
  gzip.value = data.gzip ?? ''
  const gml = parseSizeValue(data.gzip_min_length ?? '')
  gzipMinLengthNum.value = gml.num
  gzipMinLengthUnit.value = gml.unit
  gzipCompLevel.value = Number(data.gzip_comp_level) || null
  gzipTypes.value = data.gzip_types ?? ''
  gzipVary.value = data.gzip_vary ?? ''
  gzipProxied.value = data.gzip_proxied ?? ''
  brotli.value = data.brotli ?? ''
  const bml = parseSizeValue(data.brotli_min_length ?? '')
  brotliMinLengthNum.value = bml.num
  brotliMinLengthUnit.value = bml.unit
  brotliCompLevel.value = Number(data.brotli_comp_level) || null
  brotliTypes.value = data.brotli_types ?? ''
  brotliStatic.value = data.brotli_static ?? ''
  zstd.value = data.zstd ?? ''
  const zml = parseSizeValue(data.zstd_min_length ?? '')
  zstdMinLengthNum.value = zml.num
  zstdMinLengthUnit.value = zml.unit
  zstdCompLevel.value = Number(data.zstd_comp_level) || null
  zstdTypes.value = data.zstd_types ?? ''
  zstdStatic.value = data.zstd_static ?? ''
})

const getConfigData = () => ({
  worker_processes: workerProcesses.value,
  worker_connections: String(workerConnections.value ?? ''),
  keepalive_timeout: String(keepaliveTimeout.value ?? ''),
  client_max_body_size: composeSizeValue(clientMaxBodySizeNum.value, clientMaxBodySizeUnit.value),
  client_body_buffer_size: composeSizeValue(
    clientBodyBufferSizeNum.value,
    clientBodyBufferSizeUnit.value
  ),
  client_header_buffer_size: composeSizeValue(
    clientHeaderBufferSizeNum.value,
    clientHeaderBufferSizeUnit.value
  ),
  server_names_hash_bucket_size: String(serverNamesHashBucketSize.value ?? ''),
  server_tokens: serverTokens.value,
  gzip: gzip.value,
  gzip_min_length: composeSizeValue(gzipMinLengthNum.value, gzipMinLengthUnit.value),
  gzip_comp_level: String(gzipCompLevel.value ?? ''),
  gzip_types: gzipTypes.value,
  gzip_vary: gzipVary.value,
  gzip_proxied: gzipProxied.value,
  brotli: brotli.value,
  brotli_min_length: composeSizeValue(brotliMinLengthNum.value, brotliMinLengthUnit.value),
  brotli_comp_level: String(brotliCompLevel.value ?? ''),
  brotli_types: brotliTypes.value,
  brotli_static: brotliStatic.value,
  zstd: zstd.value,
  zstd_min_length: composeSizeValue(zstdMinLengthNum.value, zstdMinLengthUnit.value),
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
            <n-input-number
              class="w-full"
              v-model:value="workerConnections"
              :placeholder="$gettext('e.g. 65535')"
              :min="1"
            />
          </n-form-item>
          <n-form-item label="Keepalive Timeout (keepalive_timeout)">
            <n-input-number
              class="w-full"
              v-model:value="keepaliveTimeout"
              :placeholder="$gettext('e.g. 60')"
              :min="0"
            />
          </n-form-item>
          <n-form-item label="Client Max Body Size (client_max_body_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="clientMaxBodySizeNum"
                :placeholder="$gettext('e.g. 200')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="clientMaxBodySizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Client Body Buffer Size (client_body_buffer_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="clientBodyBufferSizeNum"
                :placeholder="$gettext('e.g. 10')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="clientBodyBufferSizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Client Header Buffer Size (client_header_buffer_size)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="clientHeaderBufferSizeNum"
                :placeholder="$gettext('e.g. 32')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="clientHeaderBufferSizeUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
          </n-form-item>
          <n-form-item label="Server Names Hash Bucket Size (server_names_hash_bucket_size)">
            <n-input-number
              class="w-full"
              v-model:value="serverNamesHashBucketSize"
              :placeholder="$gettext('e.g. 512')"
              :min="1"
            />
          </n-form-item>
          <n-form-item label="Server Tokens (server_tokens)">
            <n-select v-model:value="serverTokens" :options="onOffOptions" />
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
    <n-tab-pane name="gzip" tab="Gzip">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Gzip compression settings. Gzip is the most widely supported compression method.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="Gzip (gzip)">
            <n-select v-model:value="gzip" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (gzip_min_length)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="gzipMinLengthNum"
                :placeholder="$gettext('e.g. 1')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="gzipMinLengthUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
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
    <n-tab-pane name="brotli" tab="Brotli">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Brotli compression settings. Brotli provides better compression ratio than Gzip.'
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item label="Brotli (brotli)">
            <n-select v-model:value="brotli" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (brotli_min_length)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="brotliMinLengthNum"
                :placeholder="$gettext('e.g. 1')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="brotliMinLengthUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
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
    <n-tab-pane name="zstd" tab="Zstd">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext('Zstd compression settings. Zstd provides fast compression with high ratio.')
          }}
        </n-alert>
        <n-form>
          <n-form-item label="Zstd (zstd)">
            <n-select v-model:value="zstd" :options="onOffOptions" />
          </n-form-item>
          <n-form-item label="Min Length (zstd_min_length)">
            <n-input-group>
              <n-input-number
                class="w-full"
                v-model:value="zstdMinLengthNum"
                :placeholder="$gettext('e.g. 1')"
                :min="0"
                style="flex: 1"
              />
              <n-select
                v-model:value="zstdMinLengthUnit"
                :options="sizeUnitOptions"
                style="width: 80px"
              />
            </n-input-group>
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
  </n-tabs>
</template>
