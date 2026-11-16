<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

type PathField = 'cert_path' | 'key_path'

// 上传证书的表单对象，读写其中的 cert、key、cert_path、key_path、auto_renewal
const model = defineModel<any>('value', { required: true })

const source = ref<'paste' | 'path'>('paste')
// 父组件每次打开都会换一个新对象，据此还原来源
watch(
  model,
  (value) => {
    source.value = value.cert_path ? 'path' : 'paste'
  },
  { immediate: true },
)

const handleSource = (value: 'paste' | 'path') => {
  source.value = value
  // 只有从路径导入的证书能跟随文件更新
  model.value.auto_renewal = value == 'path'
  if (value == 'paste') {
    model.value.cert_path = ''
    model.value.key_path = ''
  }
}

const showPathSelector = ref(false)
const selectorPath = ref('/')
const selectorTarget = ref<PathField>('cert_path')

const handleSelectPath = (target: PathField) => {
  const current: string = model.value[target] || ''
  selectorTarget.value = target
  // 选择器从目录打开，已填的文件路径取其所在目录
  selectorPath.value = current.slice(0, current.lastIndexOf('/')) || '/'
  showPathSelector.value = true
}
</script>

<template>
  <n-form-item :label="$gettext('Import Method')">
    <n-radio-group :value="source" @update:value="handleSource">
      <n-radio-button value="paste">{{ $gettext('Paste Content') }}</n-radio-button>
      <n-radio-button value="path">{{ $gettext('From Path') }}</n-radio-button>
    </n-radio-group>
  </n-form-item>
  <template v-if="source == 'paste'">
    <n-form-item path="cert" :label="$gettext('Certificate')">
      <n-input
        v-model:value="model.cert"
        type="textarea"
        :placeholder="$gettext('Enter the content of the PEM certificate file')"
        :autosize="{ minRows: 10, maxRows: 15 }"
      />
    </n-form-item>
    <n-form-item path="key" :label="$gettext('Private Key')">
      <n-input
        v-model:value="model.key"
        type="textarea"
        :placeholder="$gettext('Enter the content of the KEY private key file')"
        :autosize="{ minRows: 10, maxRows: 15 }"
      />
    </n-form-item>
  </template>
  <template v-else>
    <n-alert type="info" class="mb-4">
      {{
        $gettext(
          'The panel re-reads both files every day and redeploys the certificate to associated websites when the content changes. Turn off Auto Renewal in the list to stop following the files.',
        )
      }}
    </n-alert>
    <n-form-item path="cert_path" :label="$gettext('Certificate Path')">
      <n-input-group>
        <n-input
          v-model:value="model.cert_path"
          placeholder="/path/to/fullchain.pem"
          @keydown.enter.prevent
        />
        <n-button type="primary" ghost @click="handleSelectPath('cert_path')">
          {{ $gettext('Select') }}
        </n-button>
      </n-input-group>
    </n-form-item>
    <n-form-item path="key_path" :label="$gettext('Private Key Path')">
      <n-input-group>
        <n-input
          v-model:value="model.key_path"
          placeholder="/path/to/private.key"
          @keydown.enter.prevent
        />
        <n-button type="primary" ghost @click="handleSelectPath('key_path')">
          {{ $gettext('Select') }}
        </n-button>
      </n-input-group>
    </n-form-item>
  </template>
  <path-selector
    v-model:show="showPathSelector"
    v-model:path="selectorPath"
    :dir="false"
    @select="(path: string) => (model[selectorTarget] = path)"
  />
</template>
