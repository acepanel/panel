<script setup lang="ts">
defineOptions({
  name: 'opensearch-config-tune',
})

import { useGettext } from 'vue3-gettext'

import opensearch from '@/api/apps/opensearch'

const { $gettext } = useGettext()
const currentTab = ref('cluster')

const clusterName = ref('')
const nodeName = ref('')
const networkHost = ref('')
const httpPort = ref('')
const discoveryType = ref('')
const pathData = ref('')
const pathLogs = ref('')
const heapInitSize = ref('')
const heapMaxSize = ref('')

const saveLoading = ref(false)

const discoveryTypeOptions = [
  { label: 'single-node', value: 'single-node' },
  { label: 'multi-node', value: 'multi-node' },
]

useRequest(opensearch.configTune()).onSuccess(({ data }: any) => {
  clusterName.value = data.cluster_name ?? ''
  nodeName.value = data.node_name ?? ''
  networkHost.value = data.network_host ?? ''
  httpPort.value = data.http_port ?? ''
  discoveryType.value = data.discovery_type ?? ''
  pathData.value = data.path_data ?? ''
  pathLogs.value = data.path_logs ?? ''
  heapInitSize.value = data.heap_init_size ?? ''
  heapMaxSize.value = data.heap_max_size ?? ''
})

const getConfigData = () => ({
  cluster_name: clusterName.value,
  node_name: nodeName.value,
  network_host: networkHost.value,
  http_port: httpPort.value,
  discovery_type: discoveryType.value,
  path_data: pathData.value,
  path_logs: pathLogs.value,
  heap_init_size: heapInitSize.value,
  heap_max_size: heapMaxSize.value,
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(opensearch.saveConfigTune(getConfigData()))
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
    <n-tab-pane name="cluster" :tab="$gettext('Cluster')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('OpenSearch cluster and network settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Cluster Name (cluster.name)')">
            <n-input v-model:value="clusterName" placeholder="opensearch" />
          </n-form-item>
          <n-form-item :label="$gettext('Node Name (node.name)')">
            <n-input v-model:value="nodeName" placeholder="node-1" />
          </n-form-item>
          <n-form-item :label="$gettext('Network Host (network.host)')">
            <n-input v-model:value="networkHost" placeholder="127.0.0.1" />
          </n-form-item>
          <n-form-item :label="$gettext('HTTP Port (http.port)')">
            <n-input v-model:value="httpPort" placeholder="9200" />
          </n-form-item>
          <n-form-item :label="$gettext('Discovery Type (discovery.type)')">
            <n-select v-model:value="discoveryType" :options="discoveryTypeOptions" clearable />
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
    <n-tab-pane name="paths-jvm" :tab="$gettext('Paths & JVM')">
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'Data paths and JVM heap memory settings. Heap size is recommended to be 50% of system RAM, max 31g.',
            )
          }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Data Path (path.data)')">
            <n-input v-model:value="pathData" placeholder="/var/lib/opensearch" />
          </n-form-item>
          <n-form-item :label="$gettext('Logs Path (path.logs)')">
            <n-input v-model:value="pathLogs" placeholder="/var/log/opensearch" />
          </n-form-item>
          <n-form-item :label="$gettext('Initial Heap Size (-Xms)')">
            <n-input v-model:value="heapInitSize" placeholder="1g" />
          </n-form-item>
          <n-form-item :label="$gettext('Max Heap Size (-Xmx)')">
            <n-input v-model:value="heapMaxSize" placeholder="1g" />
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
