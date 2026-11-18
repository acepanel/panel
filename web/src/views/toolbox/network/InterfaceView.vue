<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import toolboxNetwork, {
  type NetworkFamilyConfig,
  type NetworkInterface,
  type NetworkInterfaceConfig,
  type NetworkInterfaceState,
  type NetworkInterfaces,
} from '@/api/panel/toolbox-network'
import { useConfirm } from '@/components/system/composables/useConfirm'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const result = ref<NetworkInterfaces>({ manager: 'unsupported', items: [], pending: false })
const loading = ref(false)
const showModal = ref(false)
const editing = ref<NetworkInterfaceConfig>(createEmptyConfig())

// 变更后进入待确认状态，倒计时归零前未确认则由服务端自动回滚
const countdown = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const startCountdown = (seconds: number) => {
  stopCountdown()
  countdown.value = seconds
  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      stopCountdown()
      window.$message.warning($gettext('Change was not confirmed in time and has been rolled back'))
      loadInterfaces()
    }
  }, 1000)
}

const stopCountdown = () => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  countdown.value = 0
}

const confirmChange = async () => {
  await toolboxNetwork.confirmInterface()
  stopCountdown()
  window.$message.success($gettext('Change confirmed'))
  loadInterfaces()
}

const rollbackChange = async () => {
  await toolboxNetwork.rollbackInterface()
  stopCountdown()
  window.$message.success($gettext('Change rolled back'))
  loadInterfaces()
}

onUnmounted(stopCountdown)

const modeOptions = computed(() => [
  { label: $gettext('Automatic'), value: 'auto' },
  { label: $gettext('Manual'), value: 'manual' },
  { label: $gettext('Disabled'), value: 'disabled' },
])
const families = computed(() => [
  {
    key: 'ipv4' as const,
    title: $gettext('IPv4'),
    addressLabel: $gettext('IPv4 Addresses'),
    addressPlaceholder: $gettext('CIDR, e.g. 192.168.1.10/24'),
    gatewayPlaceholder: $gettext('e.g. 192.168.1.1'),
    dnsLabel: $gettext('IPv4 DNS Servers'),
    dnsPlaceholder: $gettext('e.g. 1.1.1.1'),
  },
  {
    key: 'ipv6' as const,
    title: $gettext('IPv6'),
    addressLabel: $gettext('IPv6 Addresses'),
    addressPlaceholder: $gettext('CIDR, e.g. 2001:db8::10/64'),
    gatewayPlaceholder: $gettext('e.g. 2001:db8::1'),
    dnsLabel: $gettext('IPv6 DNS Servers'),
    dnsPlaceholder: $gettext('e.g. 2606:4700:4700::1111'),
  },
])

const managerName = computed(() => {
  switch (result.value.manager) {
    case 'networkmanager':
      return 'NetworkManager'
    case 'netplan':
      return 'netplan'
    case 'ifupdown':
      return 'ifupdown'
    default:
      return $gettext('Unsupported')
  }
})

const interfaceTypeLabel = (type: string) => {
  switch (type) {
    case 'ethernet':
      return $gettext('Ethernet')
    case 'wifi':
      return $gettext('Wi-Fi')
    case 'bond':
      return $gettext('Bond')
    case 'bridge':
      return $gettext('Bridge')
    case 'vlan':
      return $gettext('VLAN')
    default:
      return type
  }
}

const interfaceReason = (reason: string) => {
  switch (reason) {
    case 'unsupported network manager':
      return $gettext('Unsupported network manager')
    case 'no active NetworkManager connection profile':
      return $gettext('No active NetworkManager connection profile')
    case 'failed to read NetworkManager connection profile':
      return $gettext('Failed to read NetworkManager connection profile')
    case 'no matching netplan interface definition':
      return $gettext('No matching netplan interface definition')
    case 'multiple netplan interface definitions match this interface':
      return $gettext('Multiple netplan interface definitions match this interface')
    case 'no matching ifupdown interface definition':
      return $gettext('No matching ifupdown interface definition')
    case 'ifupdown mapping configuration cannot be edited safely':
      return $gettext('The ifupdown mapping configuration cannot be edited safely')
    case 'ifupdown interface is defined in multiple files':
      return $gettext('The ifupdown interface is defined in multiple files')
    case 'ifupdown inheritance cannot be edited safely':
      return $gettext('The ifupdown inheritance configuration cannot be edited safely')
    case 'ifupdown interface has interleaved definitions':
      return $gettext('The ifupdown interface has interleaved definitions')
    default:
      return reason
  }
}

const columns = computed<DataTableColumns<NetworkInterface>>(() => [
  {
    title: $gettext('Interface'),
    key: 'name',
    width: 140,
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: (row) => interfaceTypeLabel(row.type),
  },
  {
    title: $gettext('Status'),
    key: 'state',
    width: 100,
    render(row) {
      return h(
        NTag,
        { type: row.state === 'up' ? 'success' : 'default', size: 'small' },
        { default: () => (row.state === 'up' ? $gettext('Up') : $gettext('Down')) },
      )
    },
  },
  {
    title: $gettext('MAC'),
    key: 'mac',
    width: 180,
    render: (row) => row.mac || '-',
  },
  {
    title: $gettext('IPv4'),
    key: 'current_ipv4',
    minWidth: 210,
    render: (row) => row.current_ipv4.addresses.join('\n') || '-',
    className: 'network-addresses',
  },
  {
    title: $gettext('IPv6'),
    key: 'current_ipv6',
    minWidth: 260,
    render: (row) => row.current_ipv6.addresses.join('\n') || '-',
    className: 'network-addresses',
  },
  {
    title: $gettext('MTU'),
    key: 'current_mtu',
    width: 100,
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 110,
    fixed: 'right',
    render(row) {
      return h('span', { title: interfaceReason(row.reason) }, [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            disabled: !row.editable,
            onClick: () => openConfig(row),
          },
          { default: () => $gettext('Configure') },
        ),
      ])
    },
  },
])

function createFamily(): NetworkFamilyConfig {
  return {
    mode: 'disabled',
    addresses: [],
    gateway: '',
    auto_dns: true,
    dns: [],
  }
}

function createEmptyConfig(): NetworkInterfaceConfig {
  return {
    name: '',
    mtu: 0,
    ipv4: createFamily(),
    ipv6: createFamily(),
  }
}

const loadInterfaces = () => {
  loading.value = true
  useRequest(toolboxNetwork.interfaces())
    .onSuccess(({ data }) => {
      result.value = data
    })
    .onComplete(() => {
      loading.value = false
    })
}

// 自动获取时配置文件里没有这些值，回填当前生效状态，切换为手动时无需另行查询
const fillFamily = (
  config: NetworkFamilyConfig,
  current: NetworkInterfaceState,
): NetworkFamilyConfig => ({
  ...config,
  addresses: config.addresses.length ? [...config.addresses] : [...current.addresses],
  gateway: config.gateway || current.gateway,
  dns: config.dns.length ? [...config.dns] : [...current.dns],
})

const openConfig = (item: NetworkInterface) => {
  editing.value = {
    name: item.name,
    mtu: item.configured_mtu || item.current_mtu,
    ipv4: fillFamily(item.ipv4, item.current_ipv4),
    ipv6: fillFamily(item.ipv6, item.current_ipv6),
  }
  showModal.value = true
}

const cleanConfig = (): NetworkInterfaceConfig => {
  const cleanFamily = (family: NetworkFamilyConfig): NetworkFamilyConfig => {
    if (family.mode === 'disabled') {
      return { mode: 'disabled', addresses: [], gateway: '', auto_dns: false, dns: [] }
    }
    return {
      ...family,
      addresses: family.addresses.map((value) => value.trim()).filter(Boolean),
      gateway: family.gateway.trim(),
      dns: family.dns.map((value) => value.trim()).filter(Boolean),
    }
  }
  return {
    name: editing.value.name,
    mtu: editing.value.mtu || 0,
    ipv4: cleanFamily(editing.value.ipv4),
    ipv6: cleanFamily(editing.value.ipv6),
  }
}

const saveConfig = async () => {
  const ok = await confirmAction({
    title: $gettext('Confirm Network Configuration Change'),
    content: $gettext(
      'Changing the primary IP address, gateway, or automatic address assignment may interrupt the panel connection. The change will be rolled back automatically unless you confirm it afterwards. Continue?',
    ),
  })
  if (!ok) return
  const data = await toolboxNetwork.updateInterface(cleanConfig())
  showModal.value = false
  startCountdown(data?.confirm_timeout ?? 30)
  loadInterfaces()
}

loadInterfaces()
</script>

<template>
  <n-flex vertical :size="16">
    <n-alert v-if="countdown > 0 || result.pending" type="warning" :show-icon="false">
      <n-flex justify="space-between" align="center">
        <span>
          {{
            countdown > 0
              ? $gettext(
                  'Network configuration changed. Confirm within %{ seconds }s to keep it, otherwise it will be rolled back automatically.',
                  { seconds: String(countdown) },
                )
              : $gettext('A network configuration change is waiting for confirmation.')
          }}
        </span>
        <n-flex :size="8">
          <app-button size="small" type="primary" @click="confirmChange">
            {{ $gettext('Keep change') }}
          </app-button>
          <app-button size="small" @click="rollbackChange">
            {{ $gettext('Roll back now') }}
          </app-button>
        </n-flex>
      </n-flex>
    </n-alert>

    <n-flex justify="space-between" align="center">
      <n-alert :type="result.manager === 'unsupported' ? 'warning' : 'info'" class="flex-1">
        {{
          $gettext('Current network manager: %{ manager }', {
            manager: managerName,
          })
        }}
      </n-alert>
      <n-button type="primary" ghost :loading="loading" @click="loadInterfaces">
        {{ $gettext('Refresh') }}
      </n-button>
    </n-flex>

    <n-data-table
      striped
      :loading="loading"
      :columns="columns"
      :data="result.items"
      :row-key="(row: NetworkInterface) => row.name"
      :scroll-x="1320"
    />
  </n-flex>

  <app-modal
    v-model:show="showModal"
    :title="$gettext('Configure Network Interface: %{ name }', { name: editing.name })"
  >
    <n-form :model="editing" label-placement="top">
      <n-form-item :label="$gettext('MTU')">
        <n-input-number
          v-model:value="editing.mtu"
          :min="0"
          :max="65535"
          :placeholder="$gettext('0 follows the system default')"
          class="w-full"
        />
      </n-form-item>

      <template v-for="family in families" :key="family.key">
        <n-divider>{{ family.title }}</n-divider>
        <n-grid cols="1 m:2" responsive="screen" :x-gap="16">
          <n-gi>
            <n-form-item :label="$gettext('Address Assignment')">
              <n-select v-model:value="editing[family.key].mode" :options="modeOptions" />
            </n-form-item>
          </n-gi>
          <n-gi>
            <n-form-item :label="$gettext('Default Gateway')">
              <n-input
                v-model:value="editing[family.key].gateway"
                :disabled="editing[family.key].mode === 'disabled'"
                :placeholder="family.gatewayPlaceholder"
              />
            </n-form-item>
          </n-gi>
        </n-grid>
        <n-form-item :label="family.addressLabel">
          <ListInput
            v-model:value="editing[family.key].addresses"
            :disabled="editing[family.key].mode === 'disabled'"
            :placeholder="family.addressPlaceholder"
          />
        </n-form-item>
        <n-form-item
          v-if="result.manager !== 'ifupdown'"
          :label="$gettext('Accept Automatically Assigned DNS')"
        >
          <n-switch
            v-model:value="editing[family.key].auto_dns"
            :disabled="editing[family.key].mode === 'disabled'"
          />
        </n-form-item>
        <n-form-item :label="family.dnsLabel">
          <ListInput
            v-model:value="editing[family.key].dns"
            :disabled="editing[family.key].mode === 'disabled'"
            :placeholder="family.dnsPlaceholder"
          />
        </n-form-item>
      </template>
    </n-form>

    <n-flex justify="end">
      <n-button @click="showModal = false">{{ $gettext('Cancel') }}</n-button>
      <app-button type="primary" @click="saveConfig">
        {{ $gettext('Save and Apply') }}
      </app-button>
    </n-flex>
  </app-modal>
</template>

<style scoped>
:deep(.network-addresses) {
  white-space: pre-line;
}
</style>
