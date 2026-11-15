<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import storage from '@/api/panel/backup-storage'
import { endpointOf, matchProvider, presetOf, s3Providers } from '@/views/backup/s3-providers'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  editItem?: any
}>()
const emit = defineEmits(['saved'])

const isEdit = computed(() => !!props.editItem)
const loading = ref(false)

const typeOptions = [
  { label: 'S3', value: 's3' },
  { label: 'SFTP', value: 'sftp' },
  { label: 'WebDAV', value: 'webdav' },
]

const styleOptions = [
  { label: 'Virtual Hosted', value: 'virtual-hosted' },
  { label: 'Path', value: 'path' },
]

const schemeOptions = [
  { label: 'HTTPS', value: 'https' },
  { label: 'HTTP', value: 'http' },
]

const sftpAuthOptions = [
  { label: $gettext('Password'), value: 'password' },
  { label: $gettext('Private Key'), value: 'private_key' },
]

const defaultInfo = () => ({
  access_key: '',
  secret_key: '',
  style: 'virtual-hosted',
  region: 'us-east-1',
  endpoint: '',
  scheme: 'https',
  bucket: '',
  url: '',
  host: '',
  port: 22,
  username: '',
  password: '',
  private_key: '',
  path: '',
})

const model = ref({ type: 's3', name: '', info: defaultInfo() })

// 后端不存 SFTP 的认证方式，私钥非空就是私钥认证
const sftpAuth = ref('password')

// S3 服务商预置只决定 style / region / endpoint / scheme 怎么填，自身不提交也不存储
const provider = ref('custom')
const region = ref<string | null>(null)
const regionSearch = ref('')

const providerOptions = [
  { label: $gettext('Custom'), value: 'custom' },
  ...s3Providers.map((p) => ({ label: $gettext(p.name), value: p.key })),
]

const preset = computed(() => s3Providers.find((p) => p.key === provider.value))

// 列表里没有的地域可以直接输入。不用 tag 模式：它把输入的文字排在第一项，
// 搜索后回车选中的是输入的片段而不是匹配到的地域
const regionOptions = computed(() => {
  const regions = preset.value?.regions ?? []
  const typed = regionSearch.value.trim()
  const extra = typed && !regions.includes(typed) ? [typed] : []
  return [...regions, ...extra].map((r) => ({ label: r, value: r }))
})

// 拼在 Bucket 输入框两侧的端点，让用户看到最终访问的地址
const bucketAddress = computed(() => {
  if (!preset.value) return null
  const host = endpointOf(preset.value, region.value ?? `<${$gettext('Region')}>`)
  return preset.value.style === 'path'
    ? { prefix: `${host}/`, suffix: '' }
    : { prefix: '', suffix: `.${host}` }
})

const handleProviderChange = (key: string) => {
  provider.value = key
  region.value = null
  regionSearch.value = ''
}

const handleRegionChange = (value: string) => {
  region.value = value
  Object.assign(model.value.info, presetOf(preset.value!, value))
}

watch(show, (val) => {
  if (!val) return
  const item = props.editItem
  model.value = item
    ? { type: item.type, name: item.name, info: { ...defaultInfo(), ...item.info } }
    : { type: 's3', name: '', info: defaultInfo() }
  // 编辑时由端点反推服务商，对不上的一律按自定义处理，原字段不动
  const matched = matchProvider(model.value.info)
  provider.value = matched?.provider.key ?? 'custom'
  region.value = matched?.region ?? null
  regionSearch.value = ''
  sftpAuth.value = model.value.info.private_key ? 'private_key' : 'password'
})

// 密码和私钥都非空时后端会两种一起用，所以 SFTP 只提交当前认证方式的凭据
const buildPayload = () => {
  if (model.value.type !== 'sftp') return model.value
  const byPassword = sftpAuth.value === 'password'
  return {
    ...model.value,
    info: {
      ...model.value.info,
      password: byPassword ? model.value.info.password : '',
      private_key: byPassword ? '' : model.value.info.private_key,
    },
  }
}

const handleSubmit = () => {
  if (model.value.type === 's3' && preset.value && !region.value) {
    window.$message.warning($gettext('Please select a region'))
    return
  }
  loading.value = true
  const req = isEdit.value
    ? storage.update(props.editItem.id, buildPayload())
    : storage.create(buildPayload())
  useRequest(req)
    .onSuccess(() => {
      show.value = false
      emit('saved')
      window.$message.success(
        isEdit.value ? $gettext('Updated successfully') : $gettext('Created successfully'),
      )
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Storage') : $gettext('Add Storage')"
  >
    <n-form :model="model">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="model.name" :placeholder="$gettext('Enter storage name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Type')" required>
        <n-select v-model:value="model.type" :options="typeOptions" />
      </n-form-item>

      <!-- S3 Fields -->
      <template v-if="model.type === 's3'">
        <n-form-item :label="$gettext('Provider')">
          <n-select
            :value="provider"
            :options="providerOptions"
            @update:value="handleProviderChange"
          />
        </n-form-item>
        <n-form-item v-if="preset" :label="$gettext('Region')" required>
          <n-select
            :value="region"
            filterable
            :options="regionOptions"
            :placeholder="$gettext('Select a region or enter a region ID')"
            @search="(value: string) => (regionSearch = value)"
            @update:value="handleRegionChange"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Access Key')" required>
          <n-input
            v-model:value="model.info.access_key"
            :placeholder="$gettext('Enter access key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Secret Key')" required>
          <n-input
            v-model:value="model.info.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter secret key')"
          />
        </n-form-item>
        <template v-if="!preset">
          <n-form-item :label="$gettext('Style')">
            <n-select v-model:value="model.info.style" :options="styleOptions" />
          </n-form-item>
          <n-form-item :label="$gettext('Region')">
            <n-input
              v-model:value="model.info.region"
              :placeholder="$gettext('Enter region (e.g., us-east-1)')"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Endpoint')" required>
            <n-input
              v-model:value="model.info.endpoint"
              :placeholder="$gettext('Enter endpoint URL')"
            />
          </n-form-item>
          <n-form-item :label="$gettext('Scheme')">
            <n-select v-model:value="model.info.scheme" :options="schemeOptions" />
          </n-form-item>
        </template>
        <n-form-item
          :label="$gettext('Bucket')"
          required
          :feedback="preset?.bucketHint && $gettext(preset.bucketHint)"
        >
          <n-input-group v-if="bucketAddress">
            <n-input-group-label v-if="bucketAddress.prefix" class="shrink min-w-0 truncate">
              {{ bucketAddress.prefix }}
            </n-input-group-label>
            <n-input
              v-model:value="model.info.bucket"
              class="flex-1 min-w-32"
              :placeholder="$gettext('Enter bucket name')"
            />
            <n-input-group-label v-if="bucketAddress.suffix" class="shrink min-w-0 truncate">
              {{ bucketAddress.suffix }}
            </n-input-group-label>
          </n-input-group>
          <n-input
            v-else
            v-model:value="model.info.bucket"
            :placeholder="$gettext('Enter bucket name')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="model.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>

      <!-- SFTP Fields -->
      <template v-if="model.type === 'sftp'">
        <n-form-item :label="$gettext('Host')" required>
          <n-input v-model:value="model.info.host" :placeholder="$gettext('Enter host')" />
        </n-form-item>
        <n-form-item :label="$gettext('Port')" required>
          <n-input-number
            v-model:value="model.info.port"
            :min="1"
            :max="65535"
            :placeholder="$gettext('Enter port')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input v-model:value="model.info.username" :placeholder="$gettext('Enter username')" />
        </n-form-item>
        <n-form-item :label="$gettext('Auth Type')" required>
          <n-select v-model:value="sftpAuth" :options="sftpAuthOptions" />
        </n-form-item>
        <n-form-item v-if="sftpAuth === 'password'" :label="$gettext('Password')" required>
          <n-input
            v-model:value="model.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item v-if="sftpAuth === 'private_key'" :label="$gettext('Private Key')" required>
          <n-input
            v-model:value="model.info.private_key"
            type="textarea"
            :rows="5"
            :placeholder="$gettext('Enter private key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')" required>
          <n-input v-model:value="model.info.path" :placeholder="$gettext('Enter remote path')" />
        </n-form-item>
      </template>

      <!-- WebDAV Fields -->
      <template v-if="model.type === 'webdav'">
        <n-form-item :label="$gettext('URL')" required>
          <n-input v-model:value="model.info.url" :placeholder="$gettext('Enter WebDAV URL')" />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input v-model:value="model.info.username" :placeholder="$gettext('Enter username')" />
        </n-form-item>
        <n-form-item :label="$gettext('Password')" required>
          <n-input
            v-model:value="model.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="model.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
</template>
