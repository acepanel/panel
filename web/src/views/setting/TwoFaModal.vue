<script setup lang="ts">
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

const model = ref({
  img: '',
  url: '',
  secret: '',
})
const code = ref('')
// 新二维码返回前显示的还是上次的密钥，期间禁止提交
const loading = ref(false)
const qrCode = computed(() => {
  return `data:image/png;base64,${model.value.img}`
})

const handleUpdate = async () => {
  await user.updateTwoFA(id.value, code.value, model.value.secret)
  show.value = false
  code.value = ''
  window.$message.success($gettext('Updated successfully'))
  window.$bus.emit('user:refresh')
}

watch(
  () => show.value,
  (val) => {
    if (val) {
      loading.value = true
      useRequest(() => user.generateTwoFA(id.value))
        .onSuccess(({ data }) => {
          model.value = data
        })
        .onComplete(() => {
          loading.value = false
        })
    }
  },
  { immediate: true },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Enable 2FA')">
    <n-spin :show="loading">
      <n-flex vertical>
        <n-grid :cols="8" item-responsive responsive="screen">
          <n-gi span="0 l:1"></n-gi>
          <n-gi span="8 l:3">
            <n-image :src="qrCode" :alt="$gettext('QR Code')" />
          </n-gi>
          <n-gi span="8 l:3">
            <n-flex vertical>
              <n-text>
                {{ $gettext('Scan the QR code with your 2FA app and enter the code below') }}
              </n-text>
              <n-text>
                {{
                  $gettext(
                    'If you cannot scan the QR code, please enter the URL below in your 2FA app',
                  )
                }}
              </n-text>
              <n-text>
                <a :href="model.url" target="_blank">{{ model.url }}</a>
              </n-text>
            </n-flex>
          </n-gi>
          <n-gi span="0 l:1"></n-gi>
        </n-grid>
        <n-form>
          <n-form-item path="code" :label="$gettext('Code')">
            <n-input
              v-model:value="code"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter the code')"
            />
          </n-form-item>
        </n-form>
        <app-button type="info" block :disabled="loading" @click="handleUpdate">{{
          $gettext('Submit')
        }}</app-button>
      </n-flex>
    </n-spin>
  </app-modal>
</template>

<style scoped lang="scss"></style>
