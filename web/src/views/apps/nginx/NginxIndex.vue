<script setup lang="ts">
import {
  NButton,
  NDataTable,
  NDynamicTags,
  NInputGroup,
  NInputNumber,
  NPopconfirm,
  NSelect
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import nginx from '@/api/apps/nginx'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const props = defineProps<{
  api: typeof nginx
  service: string
}>()

const { $gettext } = useGettext()
const currentTab = ref('status')
const streamTab = ref('server')

// 时间单位常量（纳秒）
const SECOND = 1000000000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE

// 从纳秒解析为 {value, unit} 格式
const parseDuration = (ns: number): { value: number; unit: string } => {
  if (!ns || ns <= 0) return { value: 5, unit: 's' }

  if (ns >= HOUR && ns % HOUR === 0) {
    return { value: ns / HOUR, unit: 'h' }
  }
  if (ns >= MINUTE && ns % MINUTE === 0) {
    return { value: ns / MINUTE, unit: 'm' }
  }
  return { value: Math.floor(ns / SECOND), unit: 's' }
}

// 构建纳秒时间
const buildDuration = (value: number, unit: string): number => {
  switch (unit) {
    case 'h':
      return value * HOUR
    case 'm':
      return value * MINUTE
    default:
      return value * SECOND
  }
}

// 更新超时时间值
const updateResolverTimeoutValue = (value: number) => {
  const parsed = parseDuration(streamUpstreamModel.value.resolver_timeout)
  streamUpstreamModel.value.resolver_timeout = buildDuration(value, parsed.unit)
}

// 更新超时时间单位
const updateResolverTimeoutUnit = (unit: string) => {
  const parsed = parseDuration(streamUpstreamModel.value.resolver_timeout)
  streamUpstreamModel.value.resolver_timeout = buildDuration(parsed.value, unit)
}

const { data: config } = useRequest(props.api.config, {
  initialData: ''
})
const { data: errorLog } = useRequest(props.api.errorLog, {
  initialData: ''
})
const { data: load } = useRequest(props.api.load, {
  initialData: []
})

// Stream Server 数据
const {
  data: streamServers,
  loading: streamServersLoading,
  refresh: loadStreamServers
} = usePagination(props.api.stream.listServers, {
  initialData: []
})

// Stream Upstream 数据
const {
  data: streamUpstreams,
  loading: streamUpstreamsLoading,
  refresh: loadStreamUpstreams
} = usePagination(props.api.stream.listUpstreams, {
  initialData: []
})

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
  servers: {} as Record<string, string>,
  resolver: [] as string[],
  resolver_timeout: 5 * SECOND
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
  useRequest(props.api.saveConfig(config.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleClearErrorLog = () => {
  useRequest(props.api.clearErrorLog()).onSuccess(() => {
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

const handleSaveStreamServer = () => {
  const data = {
    ...streamServerModel.value,
    proxy_timeout: streamServerModel.value.proxy_timeout * 1000000000,
    proxy_connect_timeout: streamServerModel.value.proxy_connect_timeout * 1000000000
  }

  const request = streamServerEditName.value
    ? props.api.stream.updateServer(streamServerEditName.value, data)
    : props.api.stream.createServer(data)

  useRequest(request).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
    streamServerModal.value = false
    loadStreamServers()
  })
}

const handleDeleteStreamServer = (name: string) => {
  useRequest(props.api.stream.deleteServer(name)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    loadStreamServers()
  })
}

// Stream Upstream 操作
const handleCreateStreamUpstream = () => {
  streamUpstreamModalTitle.value = $gettext('Add Stream Upstream')
  streamUpstreamEditName.value = ''
  streamUpstreamModel.value = {
    name: '',
    algo: '',
    servers: {},
    resolver: [],
    resolver_timeout: 5 * SECOND
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
    servers: { ...row.servers },
    resolver: row.resolver,
    resolver_timeout: row.resolver_timeout || 5 * SECOND
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

const handleSaveStreamUpstream = () => {
  if (Object.keys(streamUpstreamModel.value.servers).length === 0) {
    window.$message.warning($gettext('Please add at least one server'))
    return
  }

  const data = {
    name: streamUpstreamModel.value.name,
    algo: streamUpstreamModel.value.algo,
    servers: streamUpstreamModel.value.servers,
    resolver: streamUpstreamModel.value.resolver,
    resolver_timeout: streamUpstreamModel.value.resolver_timeout
  }

  const request = streamUpstreamEditName.value
    ? props.api.stream.updateUpstream(streamUpstreamEditName.value, data)
    : props.api.stream.createUpstream(data)

  useRequest(request).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
    streamUpstreamModal.value = false
    loadStreamUpstreams()
  })
}

const handleDeleteStreamUpstream = (name: string) => {
  useRequest(props.api.stream.deleteUpstream(name)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    loadStreamUpstreams()
  })
}
</script>

<template>
  <common-page show-footer>
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status :service="service" show-reload />
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
        <realtime-log :service="service" />
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
      <n-form-item
        v-if="streamServerModel.ssl"
        path="ssl_certificate"
        :label="$gettext('SSL Certificate Path')"
      >
        <n-input
          v-model:value="streamServerModel.ssl_certificate"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g. /path/to/cert.pem')"
        />
      </n-form-item>
      <n-form-item
        v-if="streamServerModel.ssl"
        path="ssl_certificate_key"
        :label="$gettext('SSL Private Key Path')"
      >
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
        <n-select
          v-model:value="streamUpstreamModel.algo"
          :options="[
            { label: $gettext('Round Robin (Default)'), value: '' },
            { label: 'least_conn', value: 'least_conn' },
            { label: 'ip_hash', value: 'ip_hash' },
            { label: 'hash $remote_addr', value: 'hash $remote_addr' },
            { label: 'random', value: 'random' },
            { label: 'least_time connect', value: 'least_time connect' },
            { label: 'least_time first_byte', value: 'least_time first_byte' }
          ]"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Servers')">
        <n-flex vertical wh-full>
          <n-flex>
            <n-input
              v-model:value="upstreamServerAddr"
              type="text"
              flex-1
              :placeholder="$gettext('Server address, e.g. 127.0.0.1:3306')"
            />
            <n-input
              v-model:value="upstreamServerOptions"
              type="text"
              flex-1
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
                <th w-100>{{ $gettext('Actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(options, addr) in streamUpstreamModel.servers" :key="addr">
                <td>{{ addr }}</td>
                <td>{{ options || '-' }}</td>
                <td>
                  <n-button
                    size="small"
                    type="error"
                    @click="handleRemoveUpstreamServer(addr as string)"
                  >
                    {{ $gettext('Delete') }}
                  </n-button>
                </td>
              </tr>
              <tr v-if="Object.keys(streamUpstreamModel.servers).length === 0">
                <td colspan="3" text-center>
                  {{ $gettext('No servers added yet') }}
                </td>
              </tr>
            </tbody>
          </n-table>
        </n-flex>
      </n-form-item>
      <n-form-item path="resolver" :label="$gettext('DNS Resolver')">
        <n-dynamic-tags
          v-model:value="streamUpstreamModel.resolver"
          :placeholder="$gettext('e.g., 8.8.8.8')"
        />
      </n-form-item>
      <n-form-item
        v-if="streamUpstreamModel.resolver.length"
        path="resolver_timeout"
        :label="$gettext('Resolver Timeout')"
      >
        <n-input-group>
          <n-input-number
            :value="parseDuration(streamUpstreamModel.resolver_timeout).value"
            :min="1"
            :max="3600"
            style="flex: 1"
            @update:value="(v: number | null) => updateResolverTimeoutValue(v ?? 5)"
          />
          <n-select
            :value="parseDuration(streamUpstreamModel.resolver_timeout).unit"
            :options="[
              { label: $gettext('Seconds'), value: 's' },
              { label: $gettext('Minutes'), value: 'm' },
              { label: $gettext('Hours'), value: 'h' }
            ]"
            style="width: 100px"
            @update:value="(v: string) => updateResolverTimeoutUnit(v)"
          />
        </n-input-group>
      </n-form-item>
    </n-form>
    <n-button type="info" block @click="handleSaveStreamUpstream">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>
