<script setup lang="ts">
import { NButton, NInput, NInputNumber, NSelect } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps<{
  serverId: number
  db: number
  keyName?: string
  mode: 'view' | 'create'
}>()

const emit = defineEmits<{
  (e: 'saved'): void
}>()

const loading = ref(false)
const model = ref({
  key: '',
  value: '',
  type: 'string',
  ttl: -1 as number,
})

const typeOptions = [
  { label: 'String', value: 'string' },
  { label: 'List', value: 'list' },
  { label: 'Set', value: 'set' },
  { label: 'ZSet', value: 'zset' },
  { label: 'Hash', value: 'hash' },
]

// 弹窗打开时初始化
watch(
  () => show.value,
  (val) => {
    if (val && props.mode === 'view' && props.keyName) {
      loading.value = true
      useRequest(database.redisKeyGet(props.serverId, props.db, props.keyName))
        .onSuccess(({ data }: { data: any }) => {
          model.value = {
            key: data.key,
            value: data.value,
            type: data.type,
            ttl: data.ttl,
          }
        })
        .onComplete(() => {
          loading.value = false
        })
    } else if (val && props.mode === 'create') {
      model.value = { key: '', value: '', type: 'string', ttl: -1 }
    }
  },
)

const saveLoading = ref(false)

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    database.redisKeySet({
      server_id: props.serverId,
      db: props.db,
      key: model.value.key,
      value: model.value.value,
      type: model.value.type,
      ttl: model.value.ttl > 0 ? model.value.ttl : 0,
    }),
  )
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Saved successfully'))
      emit('saved')
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="mode === 'create' ? $gettext('Create Key') : $gettext('View Key')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-spin :show="loading">
      <n-form :model="model">
        <n-form-item :label="$gettext('Type')">
          <n-select v-model:value="model.type" :options="typeOptions" :disabled="mode === 'view'" />
        </n-form-item>
        <n-form-item :label="$gettext('Key')">
          <n-input
            v-model:value="model.key"
            :disabled="mode === 'view'"
            :placeholder="$gettext('Enter key name')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Value')">
          <n-input
            v-model:value="model.value"
            type="textarea"
            :rows="10"
            :placeholder="
              model.type === 'string' ? $gettext('Enter value') : $gettext('Enter JSON value')
            "
          />
        </n-form-item>
        <n-form-item label="TTL">
          <n-input-number
            v-model:value="model.ttl"
            w-full
            :min="-1"
            :placeholder="$gettext('-1 means no expiration')"
          />
        </n-form-item>
      </n-form>
    </n-spin>
    <n-button type="info" block :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>
