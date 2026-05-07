<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import RefreshButton from './RefreshButton.vue'
import type { SearchField, ToolbarAction } from './types'

interface Props {
  searchFields?: SearchField[]
  primaryAction?: ToolbarAction
  toolbarActions?: ToolbarAction[]
  refreshable?: boolean
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  searchFields: () => [],
  primaryAction: undefined,
  toolbarActions: () => [],
  refreshable: true,
  loading: false,
})

const emit = defineEmits<{
  (e: 'search', params: Record<string, any>): void
  (e: 'refresh'): void
}>()

const { $gettext } = useGettext()

const queryModel = reactive<Record<string, any>>({})

watchEffect(() => {
  for (const f of props.searchFields ?? []) {
    if (queryModel[f.key] === undefined) queryModel[f.key] = f.defaultValue ?? null
  }
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null
const triggerSearch = () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    emit('search', { ...queryModel })
  }, 300)
}

const onAction = (action: ToolbarAction) => {
  void action.onClick()
}
</script>

<template>
  <div class="toolbar">
    <div class="toolbar-left">
      <template v-for="f in searchFields" :key="f.key">
        <n-input
          v-if="f.type === 'input'"
          v-model:value="queryModel[f.key]"
          :placeholder="f.placeholder ?? $gettext('Search')"
          :style="{ width: typeof f.width === 'number' ? `${f.width}px` : (f.width ?? '240px') }"
          clearable
          @update:value="triggerSearch"
        >
          <template #prefix>
            <i-mdi-magnify />
          </template>
        </n-input>
        <n-select
          v-else-if="f.type === 'select'"
          v-model:value="queryModel[f.key]"
          :placeholder="f.placeholder ?? f.label"
          :options="(f.options as any) ?? []"
          :style="{ width: typeof f.width === 'number' ? `${f.width}px` : (f.width ?? '180px') }"
          clearable
          @update:value="triggerSearch"
        />
        <n-date-picker
          v-else-if="f.type === 'date-range'"
          v-model:value="queryModel[f.key]"
          type="daterange"
          clearable
          @update:value="triggerSearch"
        />
      </template>
      <slot name="prefix" />
    </div>
    <div class="toolbar-right">
      <slot name="suffix" />
      <n-button
        v-for="action in toolbarActions"
        :key="action.key"
        :type="action.type ?? 'default'"
        :loading="action.loading"
        size="medium"
        @click="onAction(action)"
      >
        <template v-if="action.icon" #icon>
          <Icon :icon="action.icon" />
        </template>
        {{ action.label }}
      </n-button>
      <RefreshButton
        v-if="refreshable"
        :loading="loading"
        variant="icon"
        @refresh="emit('refresh')"
      />
      <n-button
        v-if="primaryAction"
        :type="primaryAction.type ?? 'primary'"
        :loading="primaryAction.loading"
        size="medium"
        @click="onAction(primaryAction)"
      >
        <template v-if="primaryAction.icon" #icon>
          <Icon :icon="primaryAction.icon" />
        </template>
        {{ primaryAction.label }}
      </n-button>
    </div>
  </div>
</template>

<script lang="ts">
import { Icon } from '@iconify/vue'
export default { components: { Icon } }
</script>
