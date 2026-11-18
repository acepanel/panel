<script setup lang="ts">
defineOptions({
  name: 'tune-preset-modal',
})

import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import type { TuneProfile } from '@/utils/tunepreset'

const props = withDefaults(
  defineProps<{
    // 需要展示的画像输入项
    fields?: ('memory' | 'cpu' | 'disk' | 'connections')[]
    // 场景选项，空则不展示场景
    scenarioOptions?: { label: string; value: string }[]
    scenarioLabel?: string
    // 默认分配内存占主机内存的比例
    memoryRatio?: number
  }>(),
  {
    fields: () => ['memory', 'cpu', 'disk'],
    scenarioOptions: () => [],
    scenarioLabel: '',
    memoryRatio: 0.75,
  },
)

const emit = defineEmits<{
  generate: [profile: TuneProfile]
}>()

const show = defineModel<boolean>('show', { default: false })

const { $gettext } = useGettext()
const formLayout = useFormLayout('auto')

const memoryMB = ref<number | null>(null)
const cpuCores = ref<number | null>(null)
const disk = ref<'hdd' | 'ssd' | 'nvme'>('ssd')
const scenario = ref('')
const connections = ref<number | null>(200)
const detected = ref(false)
const detecting = ref(false)

const diskOptions = [
  { label: 'HDD', value: 'hdd' },
  { label: 'SSD', value: 'ssd' },
  { label: 'NVMe', value: 'nvme' },
]

// 打开时按主机实际配置填充默认值
watch(show, (val) => {
  if (!val) return
  if (!scenario.value && props.scenarioOptions.length) {
    scenario.value = props.scenarioOptions[0]!.value
  }
  if (detected.value || detecting.value) return
  detecting.value = true
  useRequest(home.current([], []))
    .onSuccess(({ data }: any) => {
      if (data?.mem?.total) {
        memoryMB.value = Math.round((data.mem.total / 1024 / 1024) * props.memoryRatio)
      }
      if (data?.cpus?.length) {
        cpuCores.value = data.cpus.length
      }
      detected.value = true
    })
    .onComplete(() => {
      detecting.value = false
    })
})

const handleGenerate = () => {
  emit('generate', {
    memoryMB: memoryMB.value ?? 1024,
    cpuCores: cpuCores.value ?? 1,
    disk: disk.value,
    scenario: scenario.value,
    connections: connections.value ?? 200,
  })
  show.value = false
}
</script>

<template>
  <app-modal v-model:show="show" size="sm" :title="$gettext('Generate Recommended Configuration')">
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext(
            'Generates recommended values based on the server profile and fills them into the form. Review the values and save manually.',
          )
        }}
      </n-alert>
      <n-spin :show="detecting">
        <n-form v-bind="formLayout">
          <n-form-item v-if="fields.includes('memory')" :label="$gettext('Memory (MB)')">
            <n-input-number
              class="w-full"
              v-model:value="memoryMB"
              :min="128"
              :placeholder="$gettext('Memory allocated to this service')"
            />
          </n-form-item>
          <n-form-item v-if="fields.includes('cpu')" :label="$gettext('CPU Cores')">
            <n-input-number class="w-full" v-model:value="cpuCores" :min="1" />
          </n-form-item>
          <n-form-item v-if="fields.includes('disk')" :label="$gettext('Disk Type')">
            <n-select v-model:value="disk" :options="diskOptions" />
          </n-form-item>
          <n-form-item v-if="scenarioOptions.length" :label="scenarioLabel || $gettext('Scenario')">
            <n-select v-model:value="scenario" :options="scenarioOptions" />
          </n-form-item>
          <n-form-item v-if="fields.includes('connections')" :label="$gettext('Max Connections')">
            <n-input-number class="w-full" v-model:value="connections" :min="10" />
          </n-form-item>
        </n-form>
      </n-spin>
      <n-flex>
        <!-- 探测完成前生成会用占位的默认值 -->
        <n-button type="primary" :disabled="detecting" @click="handleGenerate">
          {{ $gettext('Generate') }}
        </n-button>
      </n-flex>
    </n-flex>
  </app-modal>
</template>
