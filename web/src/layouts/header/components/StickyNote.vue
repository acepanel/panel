<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import settingApi from '@/api/panel/setting'

const { $gettext } = useGettext()

const show = ref(false)
const content = ref('')
const saved = ref('')

const { loading } = useRequest(settingApi.getMemo(), {
  initialData: '',
  immediate: false,
}).onSuccess(({ data }: any) => {
  content.value = data || ''
  saved.value = content.value
})

const handleShow = (visible: boolean) => {
  if (visible) {
    loading.value = true
    useRequest(settingApi.getMemo()).onSuccess(({ data }: any) => {
      content.value = data || ''
      saved.value = content.value
      loading.value = false
    })
  }
}

const handleBlur = () => {
  if (content.value === saved.value) return
  useRequest(settingApi.updateMemo(content.value)).onSuccess(() => {
    saved.value = content.value
    window.$message.success($gettext('Sticky note saved'))
  })
}
</script>

<template>
  <n-popover
    trigger="click"
    placement="bottom-end"
    :show="show"
    @update:show="
      (v: boolean) => {
        show = v
        handleShow(v)
      }
    "
  >
    <template #trigger>
      <n-tooltip trigger="hover">
        <template #trigger>
          <n-icon mr-5 cursor-pointer size="20" @click="show = !show">
            <i-mdi-note-text-outline />
          </n-icon>
        </template>
        {{ $gettext('Sticky Note') }}
      </n-tooltip>
    </template>
    <n-spin :show="loading">
      <n-input
        v-model:value="content"
        type="textarea"
        :rows="8"
        :placeholder="$gettext('Write something...')"
        class="sticky-note-input"
        @blur="handleBlur"
      />
    </n-spin>
  </n-popover>
</template>

<style scoped>
.sticky-note-input {
  width: 400px;
}
</style>
