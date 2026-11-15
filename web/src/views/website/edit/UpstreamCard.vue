<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { useFormLayout } from '@/components/system/composables/useFormLayout'
import type { WebServerFeatures } from '@/utils'

import DurationInput from './DurationInput.vue'

const upstream = defineModel<any>('upstream', { required: true })
const props = defineProps<{ features: WebServerFeatures }>()
const emit = defineEmits<{ remove: [] }>()

const { $gettext } = useGettext()
const formLayout = useFormLayout(140)

// 上游负载均衡算法选项，纯算法名不翻译
const algoLabels: Record<string, () => string> = {
  '': () => $gettext('Round Robin (default)'),
}
const algoOptions = computed(() =>
  props.features.upstreamAlgos.map((value) => ({
    label: algoLabels[value]?.() ?? value,
    value,
  })),
)

// 服务器不支持附加参数时只编辑地址列表
const serverAddresses = computed({
  get: () => Object.keys(upstream.value.servers ?? {}),
  set: (addresses: string[]) => {
    upstream.value.servers = Object.fromEntries(addresses.map((address) => [address, '']))
  },
})
</script>

<template>
  <n-card closable @close="emit('remove')" mb-4>
    <template #header>
      <n-flex align="center" :size="8">
        <!-- 拖拽手柄 -->
        <div class="drag-handle" cursor-grab>
          <the-icon icon="mdi:drag" :size="20" />
        </div>
        <span>{{ $gettext('Upstream') }}</span>
        <n-input
          v-model:value="upstream.name"
          :placeholder="$gettext('Upstream name')"
          size="small"
          class="md:!w-50"
        />
      </n-flex>
    </template>
    <n-form v-bind="formLayout">
      <n-grid cols="1 m:24" :x-gap="16" responsive="screen">
        <n-form-item-gi
          v-if="algoOptions.length > 1"
          :span="12"
          :label="$gettext('Load Balancing Algorithm')"
        >
          <n-select v-model:value="upstream.algo" :options="algoOptions" />
        </n-form-item-gi>
        <n-form-item-gi
          v-if="features.upstreamKeepalive"
          :span="12"
          :label="$gettext('Keepalive Connections')"
        >
          <n-input-number
            :value="upstream.keepalive || null"
            :min="0"
            :max="1000"
            w-full
            :placeholder="$gettext('Disabled')"
            @update:value="(v: number | null) => (upstream.keepalive = v ?? 0)"
          />
        </n-form-item-gi>
        <n-form-item-gi v-if="features.resolver" :span="12" :label="$gettext('DNS Resolver')">
          <n-dynamic-tags
            v-model:value="upstream.resolver"
            :placeholder="$gettext('e.g., 8.8.8.8')"
          />
        </n-form-item-gi>
        <n-form-item-gi
          v-if="features.resolver && upstream.resolver?.length"
          :span="12"
          :label="$gettext('Resolver Timeout')"
        >
          <duration-input v-model:value="upstream.resolver_timeout" :min="1" :max="3600" />
        </n-form-item-gi>
      </n-grid>
      <n-form-item :label="$gettext('Backend Servers')">
        <n-dynamic-tags
          v-if="features.upstreamOptions === 'none'"
          v-model:value="serverAddresses"
          :placeholder="$gettext('Server address, e.g., 127.0.0.1:8080')"
        />
        <key-value-editor
          v-else
          v-model="upstream.servers"
          :key-placeholder="$gettext('Server address, e.g., 127.0.0.1:8080')"
          :value-placeholder="
            features.upstreamOptions === 'weight'
              ? $gettext('Options, e.g., weight=5')
              : $gettext('Options, e.g., weight=5 backup')
          "
          :add-button-text="$gettext('Add Server')"
          default-key-prefix="127.0.0.1:8080"
        />
      </n-form-item>
    </n-form>
  </n-card>
</template>
