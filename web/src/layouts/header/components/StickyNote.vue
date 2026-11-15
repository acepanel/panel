<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import settingApi from '@/api/panel/setting'
import { useThemeStore } from '@/stores'

const { $gettext } = useGettext()
const themeStore = useThemeStore()

const show = ref(false)
const content = ref('')
const saved = ref('')

const { loading, send } = useRequest(settingApi.getMemo, { immediate: false }).onSuccess(
  ({ data }: any) => {
    content.value = data || ''
    saved.value = content.value
  },
)

watch(show, (visible) => {
  if (visible) send()
})

const handleBlur = () => {
  if (content.value === saved.value) return
  useRequest(settingApi.updateMemo(content.value)).onSuccess(() => {
    saved.value = content.value
    window.$message.success($gettext('Sticky note saved'))
  })
}
</script>

<template>
  <!-- naive 弹层不会为贴边做位移，手机上 bottom-end 会伸出屏幕左侧，改用弹窗 -->
  <!-- disabled 的弹层收到 show=true 会立刻回写 false，所以手机上只传 false -->
  <n-popover
    trigger="click"
    placement="bottom-end"
    style="width: 400px"
    :show="show && !themeStore.isMobile"
    :disabled="themeStore.isMobile"
    @update:show="(v: boolean) => (show = v)"
  >
    <template #trigger>
      <n-tooltip trigger="hover">
        <template #trigger>
          <n-icon cursor-pointer size="20" @click="show = !show">
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
        class="w-full"
        @blur="handleBlur"
      />
    </n-spin>
  </n-popover>
  <app-modal
    :show="show && themeStore.isMobile"
    size="sm"
    :title="$gettext('Sticky Note')"
    @update:show="(v: boolean) => (show = v)"
  >
    <n-spin :show="loading">
      <n-input
        v-model:value="content"
        type="textarea"
        :rows="8"
        :placeholder="$gettext('Write something...')"
        @blur="handleBlur"
      />
    </n-spin>
  </app-modal>
</template>
