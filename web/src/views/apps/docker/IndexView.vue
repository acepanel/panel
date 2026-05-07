<script setup lang="ts">
defineOptions({
  name: 'apps-docker-index',
})

import { useGettext } from 'vue3-gettext'

import docker from '@/api/apps/docker'
import ServiceStatus from '@/components/common/ServiceStatus.vue'

const { $gettext } = useGettext()
const currentTab = ref('status')

const { data: config } = useRequest(docker.config, {
  initialData: {
    config: '',
  },
})

// 基本设置
const settingsLoading = ref(false)
const settings = ref<any>({
  'registry-mirrors': [],
  'insecure-registries': [],
  'live-restore': false,
  'log-driver': 'json-file',
  'log-opts': {
    'max-size': '',
    'max-file': '',
  },
  'cgroup-driver': '',
  hosts: [],
  'data-root': '',
  'storage-driver': '',
  dns: [],
  'firewall-backend': '',
  'ip-forward': true,
  ipv6: false,
  bip: '',
})

// 镜像输入
const mirrorInput = ref('')
const insecureRegistryInput = ref('')
const dnsInput = ref('')
const hostInput = ref('')

// 日志驱动选项
const logDriverOptions = [
  { label: 'json-file', value: 'json-file' },
  { label: 'local', value: 'local' },
  { label: 'journald', value: 'journald' },
  { label: 'syslog', value: 'syslog' },
  { label: 'fluentd', value: 'fluentd' },
  { label: 'gelf', value: 'gelf' },
  { label: 'splunk', value: 'splunk' },
  { label: 'awslogs', value: 'awslogs' },
  { label: 'none', value: 'none' },
]

// cgroup 驱动选项
const cgroupDriverOptions = [
  { label: $gettext('Default'), value: '' },
  { label: 'systemd', value: 'systemd' },
  { label: 'cgroupfs', value: 'cgroupfs' },
]

// 存储驱动选项
const storageDriverOptions = [
  { label: $gettext('Default'), value: '' },
  { label: 'overlay2', value: 'overlay2' },
  { label: 'fuse-overlayfs', value: 'fuse-overlayfs' },
  { label: 'btrfs', value: 'btrfs' },
  { label: 'zfs', value: 'zfs' },
  { label: 'vfs', value: 'vfs' },
]

// 防火墙后端选项
const firewallBackendOptions = [
  { label: 'iptables (' + $gettext('Default') + ')', value: '' },
  { label: 'iptables', value: 'iptables' },
  { label: 'nftables (' + $gettext('Experimental') + ')', value: 'nftables' },
  { label: $gettext('None'), value: 'none' },
]

// 常用镜像源预设
const mirrorPresets = [
  { label: $gettext('China - Millisecond'), value: 'https://docker.1ms.run' },
  { label: $gettext('China - DaoCloud'), value: 'https://docker.m.daocloud.io' },
  {
    label: $gettext('China - Tencent (Internal only)'),
    value: 'https://mirror.ccs.tencentyun.com',
  },
]

// 获取设置
const fetchSettings = () => {
  settingsLoading.value = true
  useRequest(docker.settings())
    .onSuccess((res) => {
      settings.value = {
        'registry-mirrors': res.data['registry-mirrors'] || [],
        'insecure-registries': res.data['insecure-registries'] || [],
        'live-restore': res.data['live-restore'] || false,
        'log-driver': res.data['log-driver'] || 'json-file',
        'log-opts': {
          'max-size': res.data['log-opts']?.['max-size'] || '',
          'max-file': res.data['log-opts']?.['max-file'] || '',
        },
        'cgroup-driver': res.data['cgroup-driver'] || '',
        hosts: res.data.hosts || [],
        'data-root': res.data['data-root'] || '',
        'storage-driver': res.data['storage-driver'] || '',
        dns: res.data.dns || [],
        'firewall-backend': res.data['firewall-backend'] || '',
        'ip-forward': res.data['ip-forward'] ?? true,
        ipv6: res.data.ipv6 ?? false,
        bip: res.data.bip || '',
      }
    })
    .onComplete(() => {
      settingsLoading.value = false
    })
}

// 添加镜像
const addMirror = () => {
  if (mirrorInput.value && !settings.value['registry-mirrors']?.includes(mirrorInput.value)) {
    settings.value['registry-mirrors'] = [
      ...(settings.value['registry-mirrors'] || []),
      mirrorInput.value,
    ]
    mirrorInput.value = ''
  }
}

// 添加非安全镜像仓库
const addInsecureRegistry = () => {
  if (
    insecureRegistryInput.value &&
    !settings.value['insecure-registries']?.includes(insecureRegistryInput.value)
  ) {
    settings.value['insecure-registries'] = [
      ...(settings.value['insecure-registries'] || []),
      insecureRegistryInput.value,
    ]
    insecureRegistryInput.value = ''
  }
}

// 添加 DNS
const addDns = () => {
  if (dnsInput.value && !settings.value.dns?.includes(dnsInput.value)) {
    settings.value.dns = [...(settings.value.dns || []), dnsInput.value]
    dnsInput.value = ''
  }
}

// 添加 Host
const addHost = () => {
  if (hostInput.value && !settings.value.hosts?.includes(hostInput.value)) {
    settings.value.hosts = [...(settings.value.hosts || []), hostInput.value]
    hostInput.value = ''
  }
}

// 保存设置
const handleSaveSettings = () => {
  useRequest(docker.updateSettings(settings.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

const handleSaveConfig = () => {
  useRequest(docker.updateConfig(config.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}

// 监听 tab 切换，加载设置
watch(currentTab, (tab) => {
  if (tab === 'settings') {
    fetchSettings()
  }
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <service-status service="docker" />
      </n-tab-pane>
      <n-tab-pane name="settings" :tab="$gettext('Basic Settings')">
        <n-spin :show="settingsLoading">
          <n-flex vertical :size="20">
            <!-- 注册表镜像 -->
            <n-card :title="$gettext('Registry Mirrors')">
              <template #header-extra>
                <n-popover trigger="click">
                  <template #trigger>
                    <n-button size="small" quaternary>
                      {{ $gettext('Presets') }}
                    </n-button>
                  </template>
                  <n-flex vertical>
                    <n-button
                      v-for="preset in mirrorPresets"
                      :key="preset.value"
                      size="small"
                      @click="
                        () => {
                          mirrorInput = preset.value
                          addMirror()
                        }
                      "
                    >
                      {{ preset.label }}
                    </n-button>
                  </n-flex>
                </n-popover>
              </template>
              <n-flex vertical>
                <n-alert type="info" :show-icon="false">
                  {{
                    $gettext(
                      'Configure registry mirrors to speed up image downloads. Domestic users can configure domestic mirrors.',
                    )
                  }}
                </n-alert>
                <n-input-group>
                  <n-input
                    v-model:value="mirrorInput"
                    :placeholder="
                      $gettext('Enter mirror address, e.g., https://registry.example.com')
                    "
                    @keydown.enter.prevent="addMirror"
                  />
                  <n-button type="primary" @click="addMirror">{{ $gettext('Add') }}</n-button>
                </n-input-group>
                <n-dynamic-tags
                  :value="settings['registry-mirrors']"
                  @update:value="settings['registry-mirrors'] = $event"
                />
              </n-flex>
            </n-card>

            <!-- 日志切割 -->
            <n-card :title="$gettext('Log Configuration')">
              <n-flex vertical>
                <n-alert type="info" :show-icon="false">
                  {{
                    $gettext(
                      'Configure log driver and rotation settings. Setting max-size and max-file can prevent log files from growing indefinitely.',
                    )
                  }}
                </n-alert>
                <n-form label-placement="left" label-width="120">
                  <n-form-item :label="$gettext('Log Driver')">
                    <n-select
                      v-model:value="settings['log-driver']"
                      :options="logDriverOptions"
                      :placeholder="$gettext('Select log driver')"
                      class="w-50"
                    />
                  </n-form-item>
                  <n-row :gutter="[24, 0]">
                    <n-col :span="12">
                      <n-form-item :label="$gettext('Max Size')">
                        <n-input
                          v-model:value="settings['log-opts']!['max-size']"
                          :placeholder="$gettext('e.g., 10m, 100m, 1g')"
                        />
                      </n-form-item>
                    </n-col>
                    <n-col :span="12">
                      <n-form-item :label="$gettext('Max Files')">
                        <n-input
                          v-model:value="settings['log-opts']!['max-file']"
                          :placeholder="$gettext('e.g., 3, 5, 10')"
                        />
                      </n-form-item>
                    </n-col>
                  </n-row>
                </n-form>
              </n-flex>
            </n-card>

            <!-- 运行时选项 -->
            <n-card :title="$gettext('Runtime Options')">
              <n-form label-placement="left" label-width="120">
                <n-row :gutter="[24, 0]">
                  <n-col :span="12">
                    <n-form-item :label="$gettext('Live Restore')">
                      <n-switch v-model:value="settings['live-restore']" />
                      <span class="text-gray-400 ml-2">
                        {{ $gettext('Keep containers alive during daemon downtime') }}
                      </span>
                    </n-form-item>
                  </n-col>
                  <n-col :span="12">
                    <n-form-item :label="$gettext('Cgroup Driver')">
                      <n-select
                        v-model:value="settings['cgroup-driver']"
                        :options="cgroupDriverOptions"
                        :placeholder="$gettext('Select cgroup driver')"
                        class="w-50"
                      />
                    </n-form-item>
                  </n-col>
                </n-row>
                <n-row :gutter="[24, 0]">
                  <n-col :span="12">
                    <n-form-item :label="$gettext('IPv6')">
                      <n-switch v-model:value="settings.ipv6" />
                      <span class="text-gray-400 ml-2">
                        {{ $gettext('Requires additional configuration.') }}
                        <n-button
                          text
                          tag="a"
                          href="https://docs.docker.com/engine/daemon/ipv6/"
                          target="_blank"
                          type="info"
                        >
                          {{ $gettext('Docs') }}
                        </n-button>
                      </span>
                    </n-form-item>
                  </n-col>
                  <n-col :span="12">
                    <n-form-item :label="$gettext('IP Forward')">
                      <n-switch v-model:value="settings['ip-forward']" />
                      <span class="text-gray-400 ml-2">
                        {{ $gettext('Enable IP forwarding') }}
                      </span>
                    </n-form-item>
                  </n-col>
                </n-row>
              </n-form>
            </n-card>

            <!-- 防火墙配置 -->
            <n-card :title="$gettext('Firewall Configuration')">
              <n-flex vertical>
                <n-alert type="info" :show-icon="false">
                  {{
                    $gettext(
                      'Configure Docker firewall backend. nftables is experimental and does not support Swarm mode.',
                    )
                  }}
                </n-alert>
                <n-form label-placement="left" label-width="140">
                  <n-form-item :label="$gettext('Firewall Backend')">
                    <n-select
                      v-model:value="settings['firewall-backend']"
                      :options="firewallBackendOptions"
                      :placeholder="$gettext('Select firewall backend')"
                      class="w-70"
                    />
                  </n-form-item>
                </n-form>
              </n-flex>
            </n-card>

            <!-- 存储与路径 -->
            <n-card :title="$gettext('Storage & Paths')">
              <n-form label-placement="left" label-width="120">
                <n-form-item :label="$gettext('Storage Driver')">
                  <n-select
                    v-model:value="settings['storage-driver']"
                    :options="storageDriverOptions"
                    :placeholder="$gettext('Select storage driver')"
                    class="w-50"
                  />
                </n-form-item>
                <n-form-item :label="$gettext('Data Root')">
                  <n-input
                    v-model:value="settings['data-root']"
                    :placeholder="$gettext('Docker data directory, default is /var/lib/docker')"
                  />
                </n-form-item>
                <n-form-item :label="$gettext('Socket/Hosts')">
                  <n-flex vertical class="w-full">
                    <n-input-group>
                      <n-input
                        v-model:value="hostInput"
                        :placeholder="
                          $gettext('e.g., unix:///var/run/docker.sock, tcp://0.0.0.0:2375')
                        "
                        @keydown.enter.prevent="addHost"
                      />
                      <n-button type="primary" @click="addHost">{{ $gettext('Add') }}</n-button>
                    </n-input-group>
                    <n-dynamic-tags
                      :value="settings.hosts"
                      @update:value="settings.hosts = $event"
                    />
                  </n-flex>
                </n-form-item>
              </n-form>
            </n-card>

            <!-- 网络配置 -->
            <n-card :title="$gettext('Network Configuration')">
              <n-form label-placement="left" label-width="120">
                <n-form-item :label="$gettext('Bridge IP')">
                  <n-input
                    v-model:value="settings.bip"
                    :placeholder="$gettext('Default bridge network IP range, e.g., 172.17.0.1/16')"
                  />
                </n-form-item>
                <n-form-item :label="$gettext('DNS Servers')">
                  <n-flex vertical class="w-full">
                    <n-input-group>
                      <n-input
                        v-model:value="dnsInput"
                        :placeholder="$gettext('e.g., 8.8.8.8, 114.114.114.114')"
                        @keydown.enter.prevent="addDns"
                      />
                      <n-button type="primary" @click="addDns">{{ $gettext('Add') }}</n-button>
                    </n-input-group>
                    <n-dynamic-tags :value="settings.dns" @update:value="settings.dns = $event" />
                  </n-flex>
                </n-form-item>
              </n-form>
            </n-card>

            <!-- 非安全镜像仓库 -->
            <n-card :title="$gettext('Insecure Registries')">
              <n-flex vertical>
                <n-alert type="warning" :show-icon="false">
                  {{
                    $gettext(
                      'Insecure registries allow Docker to communicate with registries using HTTP or self-signed certificates. Use with caution.',
                    )
                  }}
                </n-alert>
                <n-input-group>
                  <n-input
                    v-model:value="insecureRegistryInput"
                    :placeholder="$gettext('e.g., 192.168.1.100:5000')"
                    @keydown.enter.prevent="addInsecureRegistry"
                  />
                  <n-button type="primary" @click="addInsecureRegistry">
                    {{ $gettext('Add') }}
                  </n-button>
                </n-input-group>
                <n-dynamic-tags
                  :value="settings['insecure-registries']"
                  @update:value="settings['insecure-registries'] = $event"
                />
              </n-flex>
            </n-card>

            <!-- 保存按钮 -->
            <n-flex>
              <n-button type="primary" @click="handleSaveSettings">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
          </n-flex>
        </n-spin>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Configuration File')">
        <n-flex vertical>
          <n-alert type="warning">
            {{ $gettext('This modifies the Docker configuration file (/etc/docker/daemon.json)') }}
          </n-alert>
          <common-editor v-model:value="config" height="60vh" />
          <n-flex>
            <n-button type="primary" @click="handleSaveConfig">
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="docker" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
