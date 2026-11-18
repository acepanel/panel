<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useResponsive } from '@/components/system/composables/useResponsive'

import ImagePullModal from './ImagePullModal.vue'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const formLayout = useFormLayout(120)
const formLayoutWide = useFormLayout(140)
const { isMobile } = useResponsive()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  editId?: string
}>()

const isEdit = computed(() => !!props.editId)
const doSubmit = ref(false)
const currentTab = ref('basic')

// 镜像拉取
const showPullModal = ref(false)

const defaultModel = () => ({
  name: '',
  image: '',
  hostname: '',
  static_ip: '',
  network_aliases: [] as string[],
  dns: [] as string[],
  extra_hosts: [] as string[],
  publish_all_ports: false,
  ports: [] as {
    container_start: number
    container_end: number
    host_start: number
    host_end: number
    host: string
    protocol: string
  }[],
  network: '',
  volumes: [] as {
    host: string
    container: string
    mode: string
  }[],
  tmpfs: [] as { key: string; value: string }[],
  cpus: 0,
  memory: 0,
  cpu_shares: 1024,
  shm_size: 0,
  ulimits: [] as { name: string; soft: number; hard: number }[],
  env: [] as { key: string; value: string }[],
  labels: [] as { key: string; value: string }[],
  command: [] as string[],
  entrypoint: [] as string[],
  working_dir: '',
  user: '',
  restart_policy: 'no',
  tty: false,
  open_stdin: false,
  auto_remove: false,
  privileged: false,
  init: false,
  readonly_rootfs: false,
  stop_signal: '',
  stop_timeout: 0,
  healthcheck: { test: [] as string[], interval: 0, timeout: 0, start_period: 0, retries: 0 },
  cap_add: [] as string[],
  cap_drop: [] as string[],
  security_opt: [] as string[],
  devices: [] as { host: string; container: string; permissions: string }[],
  sysctls: [] as { key: string; value: string }[],
})

const createModel = reactive(defaultModel())

const networks = ref<{ label: string; value: string }[]>([])

const restartPolicyOptions = [
  { label: $gettext('None'), value: 'no' },
  { label: $gettext('Always'), value: 'always' },
  { label: $gettext('On failure (default 5 retries)'), value: 'on-failure' },
  { label: $gettext('Unless stopped'), value: 'unless-stopped' },
]

const protocolOptions = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
]

const volumeModeOptions = [
  { label: $gettext('Read-Write'), value: 'rw' },
  { label: $gettext('Read-Only'), value: 'ro' },
]

const devicePermissionOptions = ['rwm', 'rw', 'r'].map((value) => ({ label: value, value }))

const ulimitOptions = [
  'core',
  'cpu',
  'data',
  'fsize',
  'locks',
  'memlock',
  'msgqueue',
  'nice',
  'nofile',
  'nproc',
  'rss',
  'rtprio',
  'rttime',
  'sigpending',
  'stack',
].map((value) => ({ label: value, value }))

const capabilityOptions = [
  'ALL',
  'AUDIT_CONTROL',
  'AUDIT_READ',
  'AUDIT_WRITE',
  'BLOCK_SUSPEND',
  'BPF',
  'CHECKPOINT_RESTORE',
  'CHOWN',
  'DAC_OVERRIDE',
  'DAC_READ_SEARCH',
  'FOWNER',
  'FSETID',
  'IPC_LOCK',
  'IPC_OWNER',
  'KILL',
  'LEASE',
  'LINUX_IMMUTABLE',
  'MAC_ADMIN',
  'MAC_OVERRIDE',
  'MKNOD',
  'NET_ADMIN',
  'NET_BIND_SERVICE',
  'NET_BROADCAST',
  'NET_RAW',
  'PERFMON',
  'SETFCAP',
  'SETGID',
  'SETPCAP',
  'SETUID',
  'SYS_ADMIN',
  'SYS_BOOT',
  'SYS_CHROOT',
  'SYS_MODULE',
  'SYS_NICE',
  'SYS_PACCT',
  'SYS_PTRACE',
  'SYS_RAWIO',
  'SYS_RESOURCE',
  'SYS_TIME',
  'SYS_TTY_CONFIG',
  'SYSLOG',
  'WAKE_ALARM',
].map((value) => ({ label: value, value }))

// 只按 CMD-SHELL 编辑命令，未改动时保留原始 Test，避免丢掉 CMD 与 NONE 形式
const healthcheckCommand = computed({
  get: () => {
    const [type, ...args] = createModel.healthcheck.test
    return type === 'CMD' || type === 'CMD-SHELL' ? args.join(' ') : ''
  },
  set: (value: string) => {
    createModel.healthcheck.test = value ? ['CMD-SHELL', value] : []
  },
})

// 端口映射操作
const onCreatePort = () => ({
  container_start: 80,
  container_end: 80,
  host_start: 80,
  host_end: 80,
  host: '',
  protocol: 'tcp',
})

// 挂载卷操作
const onCreateVolume = () => ({
  host: '/opt/ace',
  container: '/opt/ace',
  mode: 'rw',
})

// 环境变量、标签、内核参数、tmpfs 操作
const onCreateKV = () => ({ key: '', value: '' })

const onCreateUlimit = () => ({ name: 'nofile', soft: 65535, hard: 65535 })

const onCreateDevice = () => ({ host: '', container: '', permissions: 'rwm' })

const getNetworks = () => {
  useRequest(container.networkList(1, 1000)).onSuccess(({ data }) => {
    networks.value = data.items.map((item: any) => ({
      label: item.name,
      value: item.name,
    }))
    // 编辑模式下网络由 inspect 回填，避免竞态覆盖
    if (!isEdit.value && networks.value.length > 0) {
      createModel.network = networks.value[0]?.value ?? ''
    }
  })
}

// 创建/更新容器
const createContainer = (background = false) => {
  doSubmit.value = true
  const config = { ...createModel, background }
  const req = isEdit.value
    ? container.containerUpdate(props.editId!, config)
    : container.containerCreate(config)
  useRequest(req)
    .onSuccess(() => {
      if (background) {
        window.$message.success(
          $gettext('Task submitted, please check progress in background tasks'),
        )
      } else {
        window.$message.success(
          isEdit.value ? $gettext('Updated successfully') : $gettext('Created successfully'),
        )
      }
      show.value = false
    })
    .onComplete(() => {
      doSubmit.value = false
    })
}

// 镜像拉取成功后创建容器
const onPullSuccess = () => {
  createContainer()
}

const onPullBackground = () => {
  createContainer(true)
}

const onPullCancel = () => {
  doSubmit.value = false
}

// 提交处理
const handleSubmit = async () => {
  if (!createModel.image) {
    window.$message.warning($gettext('Please enter image name'))
    return
  }

  if (isEdit.value) {
    // 更新容器需要删除重建，二次确认
    const ok = await confirmAction({
      type: 'warning',
      title: $gettext('Confirm Update'),
      content: $gettext(
        'Updating will remove the current container and recreate it with the new configuration. Data not stored in mounted volumes will be lost. Continue?',
      ),
      positiveText: $gettext('Continue'),
      negativeText: $gettext('Cancel'),
    })
    if (!ok) return
  }

  checkImageAndSubmit()
}

// 检查镜像是否存在后提交
const checkImageAndSubmit = () => {
  doSubmit.value = true

  useRequest(container.imageExist(createModel.image))
    .onSuccess(({ data }) => {
      if (data) {
        // 镜像存在，直接创建容器
        createContainer()
      } else {
        // 镜像不存在，显示拉取弹窗
        showPullModal.value = true
      }
    })
    .onError(() => {
      doSubmit.value = false
    })
}

// 编辑模式：从 inspect 数据回填表单
const fillFromInspect = (info: any) => {
  createModel.name = String(info.Name || '').replace(/^\//, '')
  createModel.image = info.Config?.Image || ''
  // Docker 默认以容器 ID 前 12 位作主机名（旧版本还会加进网络别名），回填会让新容器沿用旧 ID
  const shortId = String(info.Id || '').slice(0, 12)
  const hostname = info.Config?.Hostname || ''
  createModel.hostname = hostname === shortId ? '' : hostname
  createModel.restart_policy = info.HostConfig?.RestartPolicy?.Name || 'no'
  createModel.tty = !!info.Config?.Tty
  createModel.open_stdin = !!info.Config?.OpenStdin
  createModel.auto_remove = !!info.HostConfig?.AutoRemove
  createModel.privileged = !!info.HostConfig?.Privileged
  createModel.init = !!info.HostConfig?.Init
  createModel.readonly_rootfs = !!info.HostConfig?.ReadonlyRootfs
  createModel.publish_all_ports = !!info.HostConfig?.PublishAllPorts
  createModel.cpus = (info.HostConfig?.NanoCpus || 0) / 1e9
  createModel.memory = Math.round((info.HostConfig?.Memory || 0) / 1024 / 1024)
  createModel.cpu_shares = info.HostConfig?.CpuShares || 1024
  createModel.shm_size = Math.round((info.HostConfig?.ShmSize || 0) / 1024 / 1024)
  createModel.command = info.Config?.Cmd || []
  createModel.entrypoint = info.Config?.Entrypoint || []
  createModel.working_dir = info.Config?.WorkingDir || ''
  createModel.user = info.Config?.User || ''
  createModel.stop_signal = info.Config?.StopSignal || ''
  createModel.stop_timeout = info.Config?.StopTimeout || 0
  createModel.dns = info.HostConfig?.Dns || []
  createModel.extra_hosts = info.HostConfig?.ExtraHosts || []
  createModel.security_opt = info.HostConfig?.SecurityOpt || []
  // Docker 回显的能力带 CAP_ 前缀
  createModel.cap_add = (info.HostConfig?.CapAdd || []).map((cap: string) =>
    cap.replace(/^CAP_/, ''),
  )
  createModel.cap_drop = (info.HostConfig?.CapDrop || []).map((cap: string) =>
    cap.replace(/^CAP_/, ''),
  )
  createModel.devices = (info.HostConfig?.Devices || []).map((device: any) => ({
    host: device.PathOnHost,
    container: device.PathInContainer,
    permissions: device.CgroupPermissions,
  }))
  createModel.ulimits = (info.HostConfig?.Ulimits || []).map((ulimit: any) => ({
    name: ulimit.Name,
    soft: ulimit.Soft,
    hard: ulimit.Hard,
  }))
  createModel.sysctls = Object.entries(info.HostConfig?.Sysctls || {}).map(([key, value]) => ({
    key,
    value: String(value),
  }))
  createModel.tmpfs = Object.entries(info.HostConfig?.Tmpfs || {}).map(([key, value]) => ({
    key,
    value: String(value),
  }))

  // 健康检查：时长为纳秒
  const healthcheck = info.Config?.Healthcheck || {}
  createModel.healthcheck = {
    test: healthcheck.Test || [],
    interval: Math.round((healthcheck.Interval || 0) / 1e9),
    timeout: Math.round((healthcheck.Timeout || 0) / 1e9),
    start_period: Math.round((healthcheck.StartPeriod || 0) / 1e9),
    retries: healthcheck.Retries || 0,
  }

  // 端口映射："80/tcp" -> [{HostIp, HostPort}]
  createModel.ports = []
  for (const [containerPort, bindings] of Object.entries(info.HostConfig?.PortBindings || {})) {
    const [port, protocol] = containerPort.split('/')
    for (const binding of (bindings as any[]) || []) {
      createModel.ports.push({
        container_start: Number(port),
        container_end: Number(port),
        host_start: Number(binding.HostPort),
        host_end: Number(binding.HostPort),
        host: binding.HostIp || '',
        protocol: protocol || 'tcp',
      })
    }
  }

  // 挂载："host:container[:mode]"
  createModel.volumes = (info.HostConfig?.Binds || []).map((bind: string) => {
    const parts = bind.split(':')
    return {
      host: parts[0] || '',
      container: parts[1] || '',
      mode: parts[2] || 'rw',
    }
  })

  // 环境变量："K=V"
  createModel.env = (info.Config?.Env || []).map((env: string) => {
    const idx = env.indexOf('=')
    return { key: env.slice(0, idx), value: env.slice(idx + 1) }
  })

  // 标签
  createModel.labels = Object.entries(info.Config?.Labels || {}).map(([key, value]) => ({
    key,
    value: String(value),
  }))

  // 网络：Networks 的 key 即网络名，接入多个网络时优先取 NetworkMode 指向的主网络
  const networkMap = info.NetworkSettings?.Networks || {}
  const networkNames = Object.keys(networkMap)
  const networkName = networkNames.includes(info.HostConfig?.NetworkMode)
    ? info.HostConfig.NetworkMode
    : networkNames[0]
  createModel.network = networkName ?? ''
  const endpoint = networkMap[networkName]
  // IPAddress 是动态分配的结果，只有 IPAMConfig 里的才是指定的静态 IP
  createModel.static_ip =
    endpoint?.IPAMConfig?.IPv4Address || endpoint?.IPAMConfig?.IPv6Address || ''
  createModel.network_aliases = (endpoint?.Aliases || []).filter(
    (alias: string) => alias !== shortId,
  )
}

// 编辑模式：加载容器当前配置，回填前表单还是默认值，期间禁止提交
const loading = ref(false)
const loadContainer = () => {
  loading.value = true
  useRequest(container.containerInspect(props.editId!))
    .onSuccess(({ data }: any) => {
      fillFromInspect(data)
    })
    .onComplete(() => {
      loading.value = false
    })
}

const resetForm = () => {
  Object.assign(createModel, defaultModel())
  currentTab.value = 'basic'
  showPullModal.value = false
}

watch(show, (val) => {
  if (val) {
    resetForm()
    getNetworks()
    if (isEdit.value) {
      loadContainer()
    }
  }
})
</script>

<template>
  <app-modal
    v-model:show="show"
    size="lg"
    :title="isEdit ? $gettext('Edit Container') : $gettext('Create Container')"
    :mask-closable="!doSubmit"
    :closable="!doSubmit"
  >
    <n-spin :show="loading">
      <n-tabs v-model:value="currentTab" type="line" animated>
        <!-- 基本设置 -->
        <n-tab-pane name="basic" :tab="$gettext('Basic Settings')">
          <n-form :model="createModel" v-bind="formLayout">
            <n-form-item path="name" :label="$gettext('Container Name')">
              <n-input
                v-model:value="createModel.name"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Optional, auto-generated if empty')"
              />
            </n-form-item>

            <n-form-item path="image" :label="$gettext('Image')">
              <n-input
                v-model:value="createModel.image"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('e.g., nginx, mysql:8.4, your_username/your_image:tag')"
              />
            </n-form-item>

            <n-form-item path="restart_policy" :label="$gettext('Restart Policy')">
              <n-select
                v-model:value="createModel.restart_policy"
                :options="restartPolicyOptions"
                :placeholder="$gettext('Select restart policy')"
              />
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Container Options') }}</n-divider>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="8" path="tty" :label="$gettext('TTY (-t)')">
                <n-switch v-model:value="createModel.tty" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="open_stdin" :label="$gettext('STDIN (-i)')">
                <n-switch v-model:value="createModel.open_stdin" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="auto_remove" :label="$gettext('Auto Remove')">
                <n-switch v-model:value="createModel.auto_remove" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="privileged" :label="$gettext('Privileged')">
                <n-switch v-model:value="createModel.privileged" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="init" :label="$gettext('Init Process')">
                <n-switch v-model:value="createModel.init" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="readonly_rootfs" :label="$gettext('Read-Only')">
                <n-switch v-model:value="createModel.readonly_rootfs" />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-tab-pane>

        <!-- 网络 -->
        <n-tab-pane name="network" :tab="$gettext('Network')">
          <n-form :model="createModel" v-bind="formLayout">
            <n-form-item path="network" :label="$gettext('Network')">
              <n-select
                v-model:value="createModel.network"
                :options="networks"
                :placeholder="$gettext('Select network')"
              />
            </n-form-item>

            <n-form-item path="static_ip" :label="$gettext('Static IP')">
              <n-input
                v-model:value="createModel.static_ip"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Optional, requires a custom network with a subnet')"
              />
            </n-form-item>

            <!-- 内核 HOST_NAME_MAX 为 64，超长时容器能创建但无法启动 -->
            <n-form-item path="hostname" :label="$gettext('Hostname')">
              <n-input
                v-model:value="createModel.hostname"
                type="text"
                :maxlength="64"
                @keydown.enter.prevent
                :placeholder="$gettext('Optional, auto-generated if empty')"
              />
            </n-form-item>

            <n-form-item path="network_aliases" :label="$gettext('Network Aliases')">
              <n-dynamic-input
                v-model:value="createModel.network_aliases"
                :placeholder="$gettext('Only for custom networks')"
              />
            </n-form-item>

            <n-form-item path="dns" :label="$gettext('DNS Servers')">
              <n-dynamic-input
                v-model:value="createModel.dns"
                :placeholder="$gettext('e.g., 8.8.8.8')"
              />
            </n-form-item>

            <n-form-item path="extra_hosts" :label="$gettext('Extra Hosts')">
              <n-dynamic-input
                v-model:value="createModel.extra_hosts"
                :placeholder="$gettext('e.g., example.com:192.168.1.10')"
              />
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 端口映射 -->
        <n-tab-pane name="ports" :tab="$gettext('Port Mapping')">
          <n-form :model="createModel" v-bind="formLayout">
            <n-form-item :label="$gettext('Port Mode')">
              <n-radio-group v-model:value="createModel.publish_all_ports">
                <n-radio-button :value="false">{{ $gettext('Map Ports') }}</n-radio-button>
                <n-radio-button :value="true">{{ $gettext('Expose All') }}</n-radio-button>
              </n-radio-group>
            </n-form-item>

            <n-form-item
              v-if="!createModel.publish_all_ports"
              :label="$gettext('Port Mapping')"
              :show-label="false"
            >
              <n-dynamic-input
                v-model:value="createModel.ports"
                :on-create="onCreatePort"
                show-sort-button
              >
                <template #default="{ value }">
                  <!-- 桌面保持单行（IP 框收缩）；手机按「IP / 主机端口段 / 容器端口段 / 协议」分行 -->
                  <n-flex align="center" :wrap="isMobile" style="width: 100%">
                    <n-input
                      v-model:value="value.host"
                      :placeholder="$gettext('IP (optional)')"
                      class="w-30"
                    />
                    <n-flex align="center" :wrap="false" class="w-full md:w-auto">
                      <span>:</span>
                      <n-input-number
                        v-model:value="value.host_start"
                        :min="1"
                        :max="65535"
                        :show-button="false"
                        :placeholder="$gettext('Host Start')"
                        class="flex-1 min-w-0 md:flex-none md:w-22.5"
                      />
                      <span>-</span>
                      <n-input-number
                        v-model:value="value.host_end"
                        :min="1"
                        :max="65535"
                        :show-button="false"
                        :placeholder="$gettext('Host End')"
                        class="flex-1 min-w-0 md:flex-none md:w-22.5"
                      />
                    </n-flex>
                    <n-flex align="center" :wrap="false" class="w-full md:w-auto">
                      <span>:</span>
                      <n-input-number
                        v-model:value="value.container_start"
                        :min="1"
                        :max="65535"
                        :show-button="false"
                        :placeholder="$gettext('Container Start')"
                        class="flex-1 min-w-0 md:flex-none md:w-22.5"
                      />
                      <span>-</span>
                      <n-input-number
                        v-model:value="value.container_end"
                        :min="1"
                        :max="65535"
                        :show-button="false"
                        :placeholder="$gettext('Container End')"
                        class="flex-1 min-w-0 md:flex-none md:w-22.5"
                      />
                    </n-flex>
                    <n-select
                      v-model:value="value.protocol"
                      :options="protocolOptions"
                      class="w-22.5"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>

            <n-alert v-if="createModel.publish_all_ports" type="info">
              {{
                $gettext(
                  'All exposed ports in the image will be automatically mapped to random host ports.',
                )
              }}
            </n-alert>
          </n-form>
        </n-tab-pane>

        <!-- 存储挂载 -->
        <n-tab-pane name="volumes" :tab="$gettext('Volumes')">
          <n-form :model="createModel" v-bind="formLayout">
            <n-form-item :label="$gettext('Volume Mounts')" :show-label="false">
              <n-dynamic-input
                v-model:value="createModel.volumes"
                :on-create="onCreateVolume"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-input
                      v-model:value="value.host"
                      :placeholder="$gettext('Host path')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">:</span>
                    <n-input
                      v-model:value="value.container"
                      :placeholder="$gettext('Container path')"
                      class="md:flex-1"
                    />
                    <n-select
                      v-model:value="value.mode"
                      :options="volumeModeOptions"
                      class="md:!w-30"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>

            <n-alert type="info" class="mt-4">
              {{
                $gettext(
                  'Mount host directories or volumes into the container. Use absolute paths for host directories.',
                )
              }}
            </n-alert>

            <n-divider title-placement="left">{{ $gettext('Tmpfs Mounts') }}</n-divider>

            <n-form-item :label="$gettext('Tmpfs Mounts')" :show-label="false">
              <n-dynamic-input
                v-model:value="createModel.tmpfs"
                :on-create="onCreateKV"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-input
                      v-model:value="value.key"
                      :placeholder="$gettext('Container path')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">:</span>
                    <n-input
                      v-model:value="value.value"
                      :placeholder="$gettext('Mount options, e.g., size=64m')"
                      class="md:flex-1"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 资源限制 -->
        <n-tab-pane name="resources" :tab="$gettext('Resource Limits')">
          <n-form :model="createModel" v-bind="formLayout">
            <n-alert type="info" class="mb-4">
              {{
                $gettext(
                  'Set resource limits to prevent the container from consuming too many system resources. Set to 0 for no limit.',
                )
              }}
            </n-alert>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="8" path="memory" :label="$gettext('Memory (MB)')">
                <n-input-number
                  v-model:value="createModel.memory"
                  :min="0"
                  style="width: 100%"
                  :placeholder="$gettext('0 = no limit')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="cpus" :label="$gettext('CPU Cores')">
                <n-input-number
                  v-model:value="createModel.cpus"
                  :min="0"
                  :precision="2"
                  :step="0.5"
                  style="width: 100%"
                  :placeholder="$gettext('0 = no limit')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="cpu_shares" :label="$gettext('CPU Shares')">
                <n-input-number
                  v-model:value="createModel.cpu_shares"
                  :min="0"
                  :max="262144"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="8" path="shm_size" :label="$gettext('Shm Size (MB)')">
                <n-input-number v-model:value="createModel.shm_size" :min="0" style="width: 100%" />
              </n-form-item-gi>
            </n-grid>

            <n-divider title-placement="left">{{ $gettext('Ulimits') }}</n-divider>

            <n-form-item :label="$gettext('Ulimits')" :show-label="false">
              <n-dynamic-input
                v-model:value="createModel.ulimits"
                :on-create="onCreateUlimit"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-select
                      v-model:value="value.name"
                      :options="ulimitOptions"
                      class="md:!w-40"
                    />
                    <n-input-number
                      v-model:value="value.soft"
                      :min="-1"
                      :show-button="false"
                      :placeholder="$gettext('Soft Limit')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">:</span>
                    <n-input-number
                      v-model:value="value.hard"
                      :min="-1"
                      :show-button="false"
                      :placeholder="$gettext('Hard Limit')"
                      class="md:flex-1"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>

            <n-collapse class="mt-4">
              <n-collapse-item :title="$gettext('Resource Limit Description')">
                <n-descriptions :column="1" label-placement="left">
                  <n-descriptions-item :label="$gettext('Memory')">
                    {{ $gettext('Maximum memory the container can use, in MB. 0 means no limit.') }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('CPU Cores')">
                    {{
                      $gettext(
                        'Number of CPU cores the container can use. 0.5 means half a core, 2 means 2 cores.',
                      )
                    }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('CPU Shares')">
                    {{
                      $gettext(
                        'Relative CPU weight. Default is 1024. Higher values get more CPU time when competing.',
                      )
                    }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Shm Size')">
                    {{ $gettext('Size of /dev/shm in MB. 0 means the Docker default (64 MB).') }}
                  </n-descriptions-item>
                  <n-descriptions-item :label="$gettext('Ulimits')">
                    {{
                      $gettext(
                        'Soft and hard limits for processes in the container, such as the number of open files (nofile). -1 means unlimited.',
                      )
                    }}
                  </n-descriptions-item>
                </n-descriptions>
              </n-collapse-item>
            </n-collapse>
          </n-form>
        </n-tab-pane>

        <!-- 环境与命令 -->
        <n-tab-pane name="environment" :tab="$gettext('Environment')">
          <n-form :model="createModel" v-bind="formLayoutWide">
            <n-form-item :label="$gettext('Environment Variables')">
              <n-dynamic-input
                v-model:value="createModel.env"
                :on-create="onCreateKV"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
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
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Startup Commands') }}</n-divider>

            <n-form-item path="command" :label="$gettext('Command')">
              <n-dynamic-input
                v-model:value="createModel.command"
                :placeholder="$gettext('Command argument')"
              />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Override the default CMD of the image') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="entrypoint" :label="$gettext('Entrypoint')">
              <n-dynamic-input
                v-model:value="createModel.entrypoint"
                :placeholder="$gettext('Entrypoint argument')"
              />
              <template #feedback>
                <span class="text-gray-400">
                  {{ $gettext('Override the default ENTRYPOINT of the image') }}
                </span>
              </template>
            </n-form-item>

            <n-form-item path="working_dir" :label="$gettext('Working Directory')">
              <n-input
                v-model:value="createModel.working_dir"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Optional, uses the image default if empty')"
              />
            </n-form-item>

            <n-form-item path="user" :label="$gettext('User')">
              <n-input
                v-model:value="createModel.user"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Optional, uses the image default if empty')"
              />
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Labels') }}</n-divider>

            <n-form-item :label="$gettext('Container Labels')">
              <n-dynamic-input
                v-model:value="createModel.labels"
                :on-create="onCreateKV"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-input
                      v-model:value="value.key"
                      :placeholder="$gettext('Label name')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">=</span>
                    <n-input
                      v-model:value="value.value"
                      :placeholder="$gettext('Label value')"
                      class="md:flex-[2]"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
        </n-tab-pane>

        <!-- 高级设置 -->
        <n-tab-pane name="advanced" :tab="$gettext('Advanced Settings')">
          <n-form :model="createModel" v-bind="formLayoutWide">
            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="12" path="stop_signal" :label="$gettext('Stop Signal')">
                <n-input
                  v-model:value="createModel.stop_signal"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('Optional, uses the image default if empty')"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" path="stop_timeout" :label="$gettext('Stop Timeout (s)')">
                <n-input-number
                  v-model:value="createModel.stop_timeout"
                  :min="0"
                  style="width: 100%"
                />
              </n-form-item-gi>
            </n-grid>

            <n-divider title-placement="left">{{ $gettext('Health Check') }}</n-divider>

            <n-form-item :label="$gettext('Command')">
              <n-input
                v-model:value="healthcheckCommand"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('e.g., curl -f http://localhost/ || exit 1')"
              />
              <template #feedback>
                <span class="text-gray-400">
                  {{
                    $gettext(
                      'Leave empty to use the health check defined in the image. Durations and retries set to 0 use the defaults.',
                    )
                  }}
                </span>
              </template>
            </n-form-item>

            <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
              <n-form-item-gi :span="12" :label="$gettext('Interval (s)')">
                <n-input-number
                  v-model:value="createModel.healthcheck.interval"
                  :min="0"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Timeout (s)')">
                <n-input-number
                  v-model:value="createModel.healthcheck.timeout"
                  :min="0"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Start Period (s)')">
                <n-input-number
                  v-model:value="createModel.healthcheck.start_period"
                  :min="0"
                  style="width: 100%"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" :label="$gettext('Retry Count')">
                <n-input-number
                  v-model:value="createModel.healthcheck.retries"
                  :min="0"
                  style="width: 100%"
                />
              </n-form-item-gi>
            </n-grid>

            <n-divider title-placement="left">{{ $gettext('Security') }}</n-divider>

            <n-form-item path="cap_add" :label="$gettext('Add Capabilities')">
              <n-select
                v-model:value="createModel.cap_add"
                :options="capabilityOptions"
                multiple
                filterable
                clearable
              />
            </n-form-item>

            <n-form-item path="cap_drop" :label="$gettext('Drop Capabilities')">
              <n-select
                v-model:value="createModel.cap_drop"
                :options="capabilityOptions"
                multiple
                filterable
                clearable
              />
            </n-form-item>

            <n-form-item path="security_opt" :label="$gettext('Security Options')">
              <n-dynamic-input
                v-model:value="createModel.security_opt"
                :placeholder="$gettext('e.g., no-new-privileges')"
              />
            </n-form-item>

            <n-form-item :label="$gettext('Devices')">
              <n-dynamic-input
                v-model:value="createModel.devices"
                :on-create="onCreateDevice"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-input
                      v-model:value="value.host"
                      :placeholder="$gettext('Host path')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">:</span>
                    <n-input
                      v-model:value="value.container"
                      :placeholder="$gettext('Container path, same as host if empty')"
                      class="md:flex-1"
                    />
                    <n-select
                      v-model:value="value.permissions"
                      :options="devicePermissionOptions"
                      class="md:!w-22.5"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>

            <n-divider title-placement="left">{{ $gettext('Kernel Parameters') }}</n-divider>

            <n-form-item :label="$gettext('Kernel Parameters')" :show-label="false">
              <n-dynamic-input
                v-model:value="createModel.sysctls"
                :on-create="onCreateKV"
                show-sort-button
              >
                <template #default="{ value }">
                  <n-flex align="center" style="width: 100%">
                    <n-input
                      v-model:value="value.key"
                      :placeholder="$gettext('e.g., net.core.somaxconn')"
                      class="md:flex-1"
                    />
                    <span class="hidden md:inline">=</span>
                    <n-input
                      v-model:value="value.value"
                      :placeholder="$gettext('Value')"
                      class="md:flex-1"
                    />
                  </n-flex>
                </template>
              </n-dynamic-input>
            </n-form-item>
          </n-form>
        </n-tab-pane>
      </n-tabs>
    </n-spin>

    <template #footer>
      <n-flex justify="end">
        <n-button @click="show = false" :disabled="doSubmit">
          {{ $gettext('Cancel') }}
        </n-button>
        <n-button
          type="primary"
          :loading="doSubmit"
          :disabled="doSubmit || loading"
          @click="handleSubmit"
        >
          {{ isEdit ? $gettext('Save') : $gettext('Create') }}
        </n-button>
      </n-flex>
    </template>
  </app-modal>

  <!-- 镜像拉取弹窗 -->
  <image-pull-modal
    v-model:show="showPullModal"
    :image="createModel.image"
    @success="onPullSuccess"
    @background="onPullBackground"
    @cancel="onPullCancel"
  />
</template>
