<script setup lang="ts">
defineOptions({
  name: 'apps-openresty-index'
})

import { NButton, NDataTable, NPopconfirm, NInputNumber } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import openresty from '@/api/apps/openresty'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')
const streamTab = ref('server')

const { data: config } = useRequest(openresty.config, {
  initialData: ''
})
const { data: errorLog } = useRequest(openresty.errorLog, {
  initialData: ''
})
const { data: load } = useRequest(openresty.load, {
  initialData: []
})

// Stream Server 数据
const streamServers = ref<any[]>([])
const streamServersLoading = ref(false)

// Stream Upstream 数据
const streamUpstreams = ref<any[]>([])
const streamUpstreamsLoading = ref(false)

// 创建/编辑 Stream Server 模态框
const streamServerModal = ref(false)
const streamServerModalTitle = ref('')
const streamServerEditName = ref('')
const streamServerModel = ref({
  name: '',
  listen: '',
  udp: false,
  proxy_pass: '',
  proxy_protocol: false,
  proxy_timeout: 0,
  proxy_connect_timeout: 0,
  ssl: false,
  ssl_certificate: '',
  ssl_certificate_key: ''
})

// 创建/编辑 Stream Upstream 模态框
const streamUpstreamModal = ref(false)
const streamUpstreamModalTitle = ref('')
const streamUpstreamEditName = ref('')
const streamUpstreamModel = ref({
  name: '',
  algo: '',
  servers: {} as Record<string, string>
})

// Upstream 服务器编辑
const upstreamServerAddr = ref('')
const upstreamServerOptions = ref('')

const columns: any = [
  {
    title: $gettext('Property'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Current Value'),
    key: 'value',
    minWidth: 200,
    ellipsis: { tooltip: true }
  }
]

// Stream Server 列表列
const streamServerColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Listen'),
    key: 'listen',
    minWidth: 120,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Protocol'),
    key: 'protocol',
    minWidth: 80,
    render(row: any) {
      return row.udp ? 'UDP' : 'TCP'
    }
  },
  {
    title: $gettext('Proxy Pass'),
    key: 'proxy_pass',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: 'SSL',
    key: 'ssl',
    minWidth: 60,
    render(row: any) {
      return row.ssl ? $gettext('Yes') : $gettext('No')
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleEditStreamServer(row)
          },
          {
            default: () => $gettext('Edit')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDeleteStreamServer(row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete %{ name }?', { name: row.name })
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

// Stream Upstream 列表列
const streamUpstreamColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Algorithm'),
    key: 'algo',
    minWidth: 120,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.algo || $gettext('Round Robin')
    }
  },
  {
    title: $gettext('Servers'),
    key: 'servers',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      const servers = row.servers || {}
      return Object.keys(servers).length + $gettext(' server(s)')
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleEditStreamUpstream(row)
          },
          {
            default: () => $gettext('Edit')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => handleDeleteStreamUpstream(row.name)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete %{ name }?', { name: row.name })
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

// 加载 Stream Servers
const loadStreamServers = async () => {
  streamServersLoading.value = true
  try {
    streamServers.value = await openresty.stream.listServers()
  } finally {
    streamServersLoading.value = false
  }
}

// 加载 Stream Upstreams
const loadStreamUpstreams = async () => {
  streamUpstreamsLoading.value = true
  try {
    streamUpstreams.value = await openresty.stream.listUpstreams()
  } finally {
    streamUpstreamsLoading.value = false
  }
}

// 监听标签页切换
watch(currentTab, (val) => {
  if (val === 'stream') {
    loadStreamServers()
    loadStreamUpstreams()
  }
})

watch(streamTab, (val) => {
  if (val === 'server') {
    loadStreamServers()
  } else if (val === 'upstream') {
    loadStreamUpstreams()
  }
})

const handleSaveConfig = () => {
  useRequest(openresty.saveConfig(config.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleClearErrorLog = () => {
  useRequest(openresty.clearErrorLog()).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
  })
}

// Stream Server 操作
const handleCreateStreamServer = () => {
  streamServerModalTitle.value = $gettext('Add Stream Server')
  streamServerEditName.value = ''
  streamServerModel.value = {
    name: '',
    listen: '',
    udp: false,
    proxy_pass: '',
    proxy_protocol: false,
    proxy_timeout: 0,
    proxy_connect_timeout: 0,
    ssl: false,
    ssl_certificate: '',
    ssl_certificate_key: ''
  }
  streamServerModal.value = true
}

const handleEditStreamServer = (row: any) => {
  streamServerModalTitle.value = $gettext('Edit Stream Server')
  streamServerEditName.value = row.name
  streamServerModel.value = {
    name: row.name,
    listen: row.listen,
    udp: row.udp || false,
    proxy_pass: row.proxy_pass,
    proxy_protocol: row.proxy_protocol || false,
    proxy_timeout: row.proxy_timeout ? row.proxy_timeout / 1000000000 : 0,
    proxy_connect_timeout: row.proxy_connect_timeout ? row.proxy_connect_timeout / 1000000000 : 0,
    ssl: row.ssl || false,
    ssl_certificate: row.ssl_certificate || '',
    ssl_certificate_key: row.ssl_certificate_key || ''
  }
  streamServerModal.value = true
}

const handleSaveStreamServer = async () => {
  const data = {
    ...streamServerModel.value,
    proxy_timeout: streamServerModel.value.proxy_timeout * 1000000000,
    proxy_connect_timeout: streamServerModel.value.proxy_connect_timeout * 1000000000
  }

  try {
    if (streamServerEditName.value) {
      await openresty.stream.updateServer(streamServerEditName.value, data)
    } else {
      await openresty.stream.createServer(data)
    }
    window.$message.success($gettext('Saved successfully'))
    streamServerModal.value = false
    loadStreamServers()
  } catch (e: any) {
    window.$message.error(e.message || $gettext('Operation failed'))
  }
}

const handleDeleteStreamServer = async (name: string) => {
  try {
    await openresty.stream.deleteServer(name)
    window.$message.success($gettext('Deleted successfully'))
    loadStreamServers()
  } catch (e: any) {
    window.$message.error(e.message || $gettext('Operation failed'))
  }
}

// Stream Upstream 操作
const handleCreateStreamUpstream = () => {
  streamUpstreamModalTitle.value = $gettext('Add Stream Upstream')
  streamUpstreamEditName.value = ''
  streamUpstreamModel.value = {
    name: '',
    algo: '',
    servers: {}
  }
  upstreamServerAddr.value = ''
  upstreamServerOptions.value = ''
  streamUpstreamModal.value = true
}

const handleEditStreamUpstream = (row: any) => {
  streamUpstreamModalTitle.value = $gettext('Edit Stream Upstream')
  streamUpstreamEditName.value = row.name
  streamUpstreamModel.value = {
    name: row.name,
    algo: row.algo || '',
    servers: { ...row.servers } || {}
  }
  upstreamServerAddr.value = ''
  upstreamServerOptions.value = ''
  streamUpstreamModal.value = true
}

const handleAddUpstreamServer = () => {
  if (!upstreamServerAddr.value) {
    window.$message.warning($gettext('Please enter server address'))
    return
  }
  streamUpstreamModel.value.servers[upstreamServerAddr.value] = upstreamServerOptions.value
  upstreamServerAddr.value = ''
  upstreamServerOptions.value = ''
}

const handleRemoveUpstreamServer = (addr: string) => {
  delete streamUpstreamModel.value.servers[addr]
}

const handleSaveStreamUpstream = async () => {
  if (Object.keys(streamUpstreamModel.value.servers).length === 0) {
    window.$message.warning($gettext('Please add at least one server'))
    return
  }

  try {
    if (streamUpstreamEditName.value) {
      await openresty.stream.updateUpstream(streamUpstreamEditName.value, streamUpstreamModel.value)
    } else {
      await openresty.stream.createUpstream(streamUpstreamModel.value)
    }
    window.$message.success($gettext('Saved successfully'))
    streamUpstreamModal.value = false
    loadStreamUpstreams()
  } catch (e: any) {
    window.$message.error(e.message || $gettext('Operation failed'))
  }
}

const handleDeleteStreamUpstream = async (name: string) => {
  try {
    await openresty.stream.deleteUpstream(name)
    window.$message.success($gettext('Deleted successfully'))
    loadStreamUpstreams()
  } catch (e: any) {
    window.$message.error(e.message || $gettext('Operation failed'))
  }
}

// 负载均衡算法选项
const algoOptions = [
  { label: $gettext('Round Robin (Default)'), value: '' },
  { label: 'least_conn', value: 'least_conn' },
  { label: 'ip_hash', value: 'ip_hash' },
  { label: 'hash $remote_addr', value: 'hash $remote_addr' },
  { label: 'random', value: 'random' },
  { label: 'least_time connect', value: 'least_time connect' },
  { label: 'least_time first_byte', value: 'least_time first_byte' }
]
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="nginx" show-reload />
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Modify Configuration')">
        <n-flex vertical>
          <n-alert type="warning">
            {{
              $gettext(
                'This modifies the OpenResty main configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!'
              )
            }}
          </n-alert>
          <common-editor v-model:value="config" lang="nginx" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="stream" :tab="$gettext('Stream')">
        <n-tabs v-model:value="streamTab" type="line" placement="left" animated>
          <n-tab-pane name="server" :tab="$gettext('Server')">
            <n-flex vertical>
              <n-flex>
                <n-button type="primary" @click="handleCreateStreamServer">
                  {{ $gettext('Add Server') }}
                </n-button>
              </n-flex>
              <n-data-table
                striped
                :scroll-x="800"
                :loading="streamServersLoading"
                :columns="streamServerColumns"
                :data="streamServers"
                :row-key="(row: any) => row.name"
              />
            </n-flex>
          </n-tab-pane>
          <n-tab-pane name="upstream" :tab="$gettext('Upstream')">
            <n-flex vertical>
              <n-flex>
                <n-button type="primary" @click="handleCreateStreamUpstream">
                  {{ $gettext('Add Upstream') }}
                </n-button>
              </n-flex>
              <n-data-table
                striped
                :scroll-x="600"
                :loading="streamUpstreamsLoading"
                :columns="streamUpstreamColumns"
                :data="streamUpstreams"
                :row-key="(row: any) => row.name"
              />
            </n-flex>
          </n-tab-pane>
        </n-tabs>
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <n-data-table
          striped
          remote
          :scroll-x="400"
          :loading="false"
          :columns="columns"
          :data="load"
        />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="nginx" />
      </n-tab-pane>
      <n-tab-pane name="error-log" :tab="$gettext('Error Logs')">
        <n-flex vertical>
          <n-flex>
            <n-button type="primary" @click="handleClearErrorLog">
              {{ $gettext('Clear Log') }}
            </n-button>
          </n-flex>
          <realtime-log :path="errorLog" />
        </n-flex>
      </n-tab-pane>
    </n-tabs>
  </common-page>

  <!-- Stream Server 模态框 -->
  <n-modal
    v-model:show="streamServerModal"
    preset="card"
    :title="streamServerModalTitle"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="streamServerModal = false"
  >
    <n-form :model="streamServerModel">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="streamServerModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Only letters, numbers, underscores and hyphens')"
        />
      </n-form-item>
      <n-form-item path="listen" :label="$gettext('Listen Address')">
        <n-input
          v-model:value="streamServerModel.listen"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g. 12345 or 0.0.0.0:12345')"
        />
      </n-form-item>
      <n-form-item path="proxy_pass" :label="$gettext('Proxy Pass')">
        <n-input
          v-model:value="streamServerModel.proxy_pass"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g. 127.0.0.1:3306 or upstream_name')"
        />
      </n-form-item>
      <n-form-item path="udp" :label="$gettext('UDP Protocol')">
        <n-switch v-model:value="streamServerModel.udp" />
      </n-form-item>
      <n-form-item path="proxy_protocol" :label="$gettext('Proxy Protocol')">
        <n-switch v-model:value="streamServerModel.proxy_protocol" />
      </n-form-item>
      <n-form-item path="proxy_timeout" :label="$gettext('Proxy Timeout (seconds)')">
        <n-input-number v-model:value="streamServerModel.proxy_timeout" :min="0" />
      </n-form-item>
      <n-form-item path="proxy_connect_timeout" :label="$gettext('Connect Timeout (seconds)')">
        <n-input-number v-model:value="streamServerModel.proxy_connect_timeout" :min="0" />
      </n-form-item>
      <n-form-item path="ssl" :label="$gettext('Enable SSL')">
        <n-switch v-model:value="streamServerModel.ssl" />
      </n-form-item>
      <n-form-item v-if="streamServerModel.ssl" path="ssl_certificate" :label="$gettext('SSL Certificate Path')">
        <n-input
          v-model:value="streamServerModel.ssl_certificate"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g. /path/to/cert.pem')"
        />
      </n-form-item>
      <n-form-item v-if="streamServerModel.ssl" path="ssl_certificate_key" :label="$gettext('SSL Private Key Path')">
        <n-input
          v-model:value="streamServerModel.ssl_certificate_key"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g. /path/to/key.pem')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block @click="handleSaveStreamServer">{{ $gettext('Submit') }}</n-button>
  </n-modal>

  <!-- Stream Upstream 模态框 -->
  <n-modal
    v-model:show="streamUpstreamModal"
    preset="card"
    :title="streamUpstreamModalTitle"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="streamUpstreamModal = false"
  >
    <n-form :model="streamUpstreamModel">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="streamUpstreamModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Only letters, numbers, underscores and hyphens')"
        />
      </n-form-item>
      <n-form-item path="algo" :label="$gettext('Load Balancing Algorithm')">
        <n-select v-model:value="streamUpstreamModel.algo" :options="algoOptions" />
      </n-form-item>
      <n-form-item :label="$gettext('Servers')">
        <n-flex vertical style="width: 100%">
          <n-flex>
            <n-input
              v-model:value="upstreamServerAddr"
              type="text"
              style="flex: 1"
              :placeholder="$gettext('Server address, e.g. 127.0.0.1:3306')"
            />
            <n-input
              v-model:value="upstreamServerOptions"
              type="text"
              style="flex: 1"
              :placeholder="$gettext('Options (optional), e.g. weight=5 backup')"
            />
            <n-button type="primary" @click="handleAddUpstreamServer">
              {{ $gettext('Add') }}
            </n-button>
          </n-flex>
          <n-table :bordered="false" :single-line="false" size="small">
            <thead>
              <tr>
                <th>{{ $gettext('Address') }}</th>
                <th>{{ $gettext('Options') }}</th>
                <th style="width: 100px">{{ $gettext('Actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(options, addr) in streamUpstreamModel.servers" :key="addr">
                <td>{{ addr }}</td>
                <td>{{ options || '-' }}</td>
                <td>
                  <n-button size="small" type="error" @click="handleRemoveUpstreamServer(addr as string)">
                    {{ $gettext('Delete') }}
                  </n-button>
                </td>
              </tr>
              <tr v-if="Object.keys(streamUpstreamModel.servers).length === 0">
                <td colspan="3" style="text-align: center; color: #999">
                  {{ $gettext('No servers added yet') }}
                </td>
              </tr>
            </tbody>
          </n-table>
        </n-flex>
      </n-form-item>
    </n-form>
    <n-button type="info" block @click="handleSaveStreamUpstream">{{ $gettext('Submit') }}</n-button>
  </n-modal>
</template>
