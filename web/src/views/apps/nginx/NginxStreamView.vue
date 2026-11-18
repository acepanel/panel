<script setup lang="ts">
import { NButton, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type nginx from '@/api/apps/nginx'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useResponsive } from '@/components/system/composables/useResponsive'
import DurationInput from '@/views/website/edit/DurationInput.vue'

const props = defineProps<{
  api: typeof nginx
}>()

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const { confirmDelete } = useConfirm()
const streamTab = ref('server')

// Go time.Duration 在 JSON 中以纳秒表示
const SECOND = 1000000000

// Stream Server 数据
const {
  data: streamServers,
  loading: streamServersLoading,
  send: loadStreamServers,
} = useRequest(props.api.stream.listServers, {
  initialData: [],
})

// Stream Upstream 数据
const {
  data: streamUpstreams,
  loading: streamUpstreamsLoading,
  send: loadStreamUpstreams,
} = useRequest(props.api.stream.listUpstreams, {
  initialData: [],
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
  ssl_certificate_key: '',
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
  resolver_timeout: 5 * SECOND,
})

// Upstream 服务器编辑
const upstreamServerAddr = ref('')
const upstreamServerOptions = ref('')

// Stream Server 列表列
const streamServerColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Listen'),
    key: 'listen',
    minWidth: 120,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Protocol'),
    key: 'protocol',
    minWidth: 80,
    render(row: any) {
      return row.udp ? 'UDP' : 'TCP'
    },
  },
  {
    title: $gettext('Proxy Pass'),
    key: 'proxy_pass',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: 'SSL',
    key: 'ssl',
    minWidth: 60,
    render(row: any) {
      return row.ssl ? $gettext('Yes') : $gettext('No')
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleEditStreamServer(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete %{ name }?', { name: row.name }),
              })
              if (ok) await handleDeleteStreamServer(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

// Stream Upstream 列表列
const streamUpstreamColumns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Algorithm'),
    key: 'algo',
    minWidth: 120,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.algo || $gettext('Round Robin')
    },
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
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => handleEditStreamUpstream(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete %{ name }?', { name: row.name }),
              })
              if (ok) await handleDeleteStreamUpstream(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

watch(streamTab, (val) => {
  if (val === 'server') {
    loadStreamServers()
  } else if (val === 'upstream') {
    loadStreamUpstreams()
  }
})

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
    ssl_certificate_key: '',
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
    proxy_timeout: row.proxy_timeout ? row.proxy_timeout / SECOND : 0,
    proxy_connect_timeout: row.proxy_connect_timeout ? row.proxy_connect_timeout / SECOND : 0,
    ssl: row.ssl || false,
    ssl_certificate: row.ssl_certificate || '',
    ssl_certificate_key: row.ssl_certificate_key || '',
  }
  streamServerModal.value = true
}

const handleSaveStreamServer = async () => {
  const data = {
    ...streamServerModel.value,
    proxy_timeout: streamServerModel.value.proxy_timeout * SECOND,
    proxy_connect_timeout: streamServerModel.value.proxy_connect_timeout * SECOND,
  }

  const request = streamServerEditName.value
    ? props.api.stream.updateServer(streamServerEditName.value, data)
    : props.api.stream.createServer(data)

  await request
  window.$message.success($gettext('Saved successfully'))
  streamServerModal.value = false
  loadStreamServers()
}

const handleDeleteStreamServer = async (name: string) => {
  await props.api.stream.deleteServer(name)
  window.$message.success($gettext('Deleted successfully'))
  loadStreamServers()
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
    resolver_timeout: 5 * SECOND,
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
    resolver_timeout: row.resolver_timeout || 5 * SECOND,
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

const saveStreamUpstream = async () => {
  const data = {
    name: streamUpstreamModel.value.name,
    algo: streamUpstreamModel.value.algo,
    servers: streamUpstreamModel.value.servers,
    resolver: streamUpstreamModel.value.resolver,
    resolver_timeout: streamUpstreamModel.value.resolver_timeout,
  }

  const request = streamUpstreamEditName.value
    ? props.api.stream.updateUpstream(streamUpstreamEditName.value, data)
    : props.api.stream.createUpstream(data)

  await request
  window.$message.success($gettext('Saved successfully'))
  streamUpstreamModal.value = false
  loadStreamUpstreams()
}

// 校验不通过时同步返回，避免提交按钮闪一下加载图标
const handleSaveStreamUpstream = () => {
  if (Object.keys(streamUpstreamModel.value.servers).length === 0) {
    window.$message.warning($gettext('Please add at least one server'))
    return
  }
  return saveStreamUpstream()
}

const handleDeleteStreamUpstream = async (name: string) => {
  await props.api.stream.deleteUpstream(name)
  window.$message.success($gettext('Deleted successfully'))
  loadStreamUpstreams()
}
</script>

<template>
  <n-tabs v-model:value="streamTab" type="line" :placement="isMobile ? 'top' : 'left'" animated>
    <n-tab-pane name="server" :tab="$gettext('Server')">
      <n-flex vertical>
        <n-flex>
          <n-button type="primary" @click="handleCreateStreamServer">
            {{ $gettext('Add Server') }}
          </n-button>
        </n-flex>
        <n-data-table
          striped
          :scroll-x="900"
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
          :scroll-x="700"
          :loading="streamUpstreamsLoading"
          :columns="streamUpstreamColumns"
          :data="streamUpstreams"
          :row-key="(row: any) => row.name"
        />
      </n-flex>
    </n-tab-pane>
  </n-tabs>
  <!-- Stream Server 模态框 -->
  <app-modal v-model:show="streamServerModal" :title="streamServerModalTitle">
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
    <app-button type="info" block @click="handleSaveStreamServer">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <!-- Stream Upstream 模态框 -->
  <app-modal v-model:show="streamUpstreamModal" :title="streamUpstreamModalTitle">
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
            { label: 'least_time first_byte', value: 'least_time first_byte' },
          ]"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Servers')">
        <n-flex vertical class="wh-full">
          <n-flex>
            <n-input
              v-model:value="upstreamServerAddr"
              type="text"
              class="flex-1 min-w-50"
              :placeholder="$gettext('Server address, e.g. 127.0.0.1:3306')"
            />
            <n-input
              v-model:value="upstreamServerOptions"
              type="text"
              class="flex-1 min-w-50"
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
                <th class="w-25">{{ $gettext('Actions') }}</th>
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
                <td colspan="3" class="text-center">
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
        <duration-input v-model:value="streamUpstreamModel.resolver_timeout" :min="1" :max="3600" />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleSaveStreamUpstream">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
