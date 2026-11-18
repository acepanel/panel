<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps<{
  serverId: number
  index: string
  docId?: string
  mode: 'view' | 'create'
}>()

const emit = defineEmits<{
  (e: 'saved'): void
}>()

const loading = ref(false)
const model = ref({
  id: '',
  body: '',
})

watch(
  () => show.value,
  (val) => {
    if (val && props.mode === 'view' && props.docId) {
      loading.value = true
      useRequest(database.esDocumentGet(props.serverId, props.index, props.docId))
        .onSuccess(({ data }: { data: any }) => {
          model.value = {
            id: data.id,
            body: JSON.stringify(JSON.parse(data.source), null, 2),
          }
        })
        .onComplete(() => {
          loading.value = false
        })
    } else if (val && props.mode === 'create') {
      model.value = { id: '', body: '{\n  \n}' }
    }
  },
)

const handleSave = async () => {
  await database.esDocumentSet({
    server_id: props.serverId,
    index: props.index,
    id: model.value.id || undefined,
    body: model.value.body,
  })
  show.value = false
  window.$message.success($gettext('Saved successfully'))
  emit('saved')
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="mode === 'create' ? $gettext('Create Document') : $gettext('View Document')"
  >
    <n-spin :show="loading">
      <n-form :model="model">
        <n-form-item label="ID">
          <n-input
            v-model:value="model.id"
            :disabled="mode === 'view'"
            :placeholder="$gettext('Leave empty for auto-generated ID')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Document (JSON)')">
          <n-input
            v-model:value="model.body"
            type="textarea"
            :rows="15"
            :placeholder="$gettext('Enter JSON document')"
          />
        </n-form-item>
      </n-form>
    </n-spin>
    <app-button type="info" block :disabled="loading" @click="handleSave">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
