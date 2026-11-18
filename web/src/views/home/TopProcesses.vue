<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import { formatBytes } from '@/utils/file'

import type { ProcessStat } from './types'

const props = defineProps<{
  type: 'cpu' | 'memory'
}>()

const { $gettext } = useGettext()

const meta = {
  cpu: {
    title: $gettext('CPU Top5 Processes'),
    column: 'CPU',
    format: (value: number) => `${value.toFixed(1)}%`,
  },
  memory: {
    title: $gettext('Memory Top5 Processes'),
    column: $gettext('Memory'),
    format: formatBytes,
  },
}[props.type]

const visible = ref(false)
const list = ref<ProcessStat[]>([])

const show = async () => {
  list.value = await home.topProcesses(props.type)
  visible.value = true
}

// 收起时不能返回 Promise，否则按钮会闪一下加载图标
const toggle = () => {
  if (!visible.value) return show()
  visible.value = false
}
</script>

<template>
  <n-flex justify="center">
    <app-button text type="primary" size="small" @click="toggle">
      {{ meta.title }} {{ visible ? '∧' : '>' }}
    </app-button>
  </n-flex>
  <n-table
    v-if="visible && list.length"
    :single-line="false"
    size="small"
    striped
    class="mt-2 whitespace-nowrap"
  >
    <thead>
      <tr>
        <th>PID</th>
        <th>{{ $gettext('Name') }}</th>
        <th>{{ meta.column }}</th>
        <th>{{ $gettext('User') }}</th>
        <th>{{ $gettext('Command') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="proc in list" :key="proc.pid">
        <td>{{ proc.pid }}</td>
        <td>
          <n-ellipsis style="max-width: 120px">
            {{ proc.name }}
          </n-ellipsis>
        </td>
        <td>{{ meta.format(proc.value) }}</td>
        <td>{{ proc.username }}</td>
        <td>
          <n-ellipsis style="max-width: 200px">
            {{ proc.command }}
          </n-ellipsis>
        </td>
      </tr>
    </tbody>
  </n-table>
</template>
