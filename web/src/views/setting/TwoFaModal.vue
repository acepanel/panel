<script setup lang="ts">
import user from '@/api/panel/user'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

const model = ref({
  img: '',
  url: '',
  secret: ''
})
const code = ref('')
const loading = ref(false)
const qrCode = computed(() => {
  return `data:image/png;base64,${model.value.img}`
})

const handleUpdate = () => {
  loading.value = true
  useRequest(() => user.updateTwoFA(id.value, code.value, model.value.secret))
    .onSuccess(() => {
      show.value = false
      code.value = ''
      window.$message.success($gettext('Updated successfully'))
      window.$bus.emit('user:refresh')
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(
  () => show.value,
  (val) => {
    if (val) {
      useRequest(() => user.generateTwoFA(id.value)).onSuccess(({ data }) => {
        model.value = data
      })
    }
  },
  { immediate: true }
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Enable 2FA')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
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
                  'If you cannot scan the QR code, please enter the URL below in your 2FA app'
                )
              }}
            </n-text>
            <n-text>
              <a :href="model.url" target="_blank">{{ model.url }}</a>
            </n-text>
          </n-flex>
        </n-gi>
        <n-gi span="0 l:1"> </n-gi>>
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
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleUpdate">{{ $gettext('Submit') }}</n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
