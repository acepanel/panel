<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import website from '@/api/panel/website'
import { generateRandomString, notifyWebsiteCreated } from '@/utils'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const type = defineModel<string>('type', { type: String, required: true })

const { $gettext } = useGettext()

// 内部选择的类型（当外部 type 为 'all' 时使用）
const selectedType = ref('proxy')

// 实际使用的网站类型
const effectiveType = computed(() => {
  if (type.value === 'all') {
    return selectedType.value
  }
  return type.value
})

// 类型选项
const typeOptions = computed(() => [
  { label: $gettext('Reverse Proxy'), value: 'proxy' },
  { label: $gettext('PHP'), value: 'php' },
  { label: $gettext('Pure Static'), value: 'static' },
])
const defaultModel = () => ({
  type: '',
  name: '',
  listens: ['80'],
  domains: [''],
  path: '',
  db: false,
  db_type: '0',
  db_name: '',
  db_user: '',
  db_password: '',
  remark: '',

  php: null,
  proxy: '',
})

const createModel = ref(defaultModel())

// 弹窗复用同一实例，每次打开时重置表单，避免残留在其他类型标签下填写的 PHP、数据库等字段
watch(show, (value) => {
  if (value) {
    createModel.value = defaultModel()
  }
})

const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace')
const loading = ref(false)

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    php: [],
    db: [
      {
        label: '',
        value: '',
      },
    ],
  },
})

// 获取模态框标题
const modalTitle = computed(() => {
  switch (effectiveType.value) {
    case 'proxy':
      return $gettext('Create Reverse Proxy Website')
    case 'php':
      return $gettext('Create PHP Website')
    case 'static':
      return $gettext('Create Pure Static Website')
    default:
      return $gettext('Create Website')
  }
})

const handleCreate = async () => {
  createModel.value.type = effectiveType.value
  // 端口为空自动添加 80 端口
  if (createModel.value.listens.length === 0) {
    createModel.value.listens.push('80')
  }
  // 端口中去掉 443 端口，不允许在未配置证书下监听 443 端口
  createModel.value.listens = createModel.value.listens.filter((item) => item !== '443')
  loading.value = true
  useRequest(website.create(createModel.value))
    .onSuccess(({ data }) => {
      window.$bus.emit('website:refresh')
      notifyWebsiteCreated(createModel.value.name, data.warnings)
      show.value = false
    })
    .onComplete(() => {
      loading.value = false
    })
}

const formatDbValue = (value: string) => {
  value = value.replace(/\./g, '_')
  value = value.replace(/-/g, '_')
  if (value.length > 16) {
    value = value.substring(0, 16)
  }

  return value
}

// 处理目录选择
const handleSelectPath = () => {
  pathSelectorPath.value = createModel.value.path || '/opt/ace'
  showPathSelector.value = true
}

// 目录选择完成
watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value && pathSelectorPath.value !== '/opt/ace') {
    createModel.value.path = pathSelectorPath.value
  }
})
</script>

<template>
  <app-modal v-model:show="show" :title="modalTitle">
    <n-form :model="createModel">
      <n-form-item v-if="type === 'all'" path="type" :label="$gettext('Website Type')">
        <n-select
          v-model:value="selectedType"
          :options="typeOptions"
          :placeholder="$gettext('Select Website Type')"
        />
      </n-form-item>
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="createModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="
            $gettext(
              'Must be unique, used for directory naming, database naming, etc., and only supports letters, numbers, hyphens, and underscores',
            )
          "
        />
      </n-form-item>
      <n-grid cols="1 m:24" :x-gap="24" responsive="screen">
        <n-form-item-gi :span="12" :label="$gettext('Domain')">
          <ListInput
            v-model:value="createModel.domains"
            placeholder="example.com"
            :min="1"
            show-sort-button
          />
        </n-form-item-gi>
        <n-form-item-gi :span="12" :label="$gettext('Port')">
          <ListInput
            v-model:value="createModel.listens"
            placeholder="80"
            :min="1"
            show-sort-button
          />
        </n-form-item-gi>
      </n-grid>
      <n-grid v-if="effectiveType == 'php'" cols="1 m:24" :x-gap="24" responsive="screen">
        <n-form-item-gi :span="12" path="php" :label="$gettext('PHP Version')">
          <n-select
            v-model:value="createModel.php"
            :options="installedEnvironment.php"
            :placeholder="$gettext('Select PHP Version')"
            clearable
            @keydown.enter.prevent
          >
          </n-select>
        </n-form-item-gi>
        <n-form-item-gi :span="12" path="db" :label="$gettext('Database')">
          <n-select
            v-model:value="createModel.db_type"
            :options="installedEnvironment.db"
            :placeholder="$gettext('Select Database')"
            @keydown.enter.prevent
            @update:value="
              () => {
                createModel.db = createModel.db_type != '0'
                createModel.db_name = formatDbValue(createModel.name)
                createModel.db_user = formatDbValue(createModel.name)
                createModel.db_password = generateRandomString(16)
              }
            "
          >
          </n-select>
        </n-form-item-gi>
      </n-grid>
      <n-grid
        v-if="effectiveType == 'php' && createModel.db"
        cols="1 m:24"
        :x-gap="24"
        responsive="screen"
      >
        <n-form-item-gi :span="8" path="db_name" :label="$gettext('Database Name')">
          <n-input
            v-model:value="createModel.db_name"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Database Name')"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="8" path="db_user" :label="$gettext('Database User')">
          <n-input
            v-model:value="createModel.db_user"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Database User')"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="8" path="db_password" :label="$gettext('Database Password')">
          <n-input
            v-model:value="createModel.db_password"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Database Password')"
          />
        </n-form-item-gi>
      </n-grid>
      <n-form-item v-if="effectiveType != 'proxy'" path="path" :label="$gettext('Directory')">
        <n-input-group>
          <n-input
            v-model:value="createModel.path"
            type="text"
            @keydown.enter.prevent
            :placeholder="
              $gettext(
                'Website root directory (if left empty, defaults to website directory/website name/public)',
              )
            "
          />
          <n-button @click="handleSelectPath">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
      </n-form-item>
      <n-form-item v-if="effectiveType == 'proxy'" path="path" :label="$gettext('Proxy Target')">
        <n-input
          v-model:value="createModel.proxy"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Proxy target address (e.g., http://127.0.0.1:3000)')"
        />
      </n-form-item>
      <n-form-item path="remark" :label="$gettext('Remark')">
        <n-input
          v-model:value="createModel.remark"
          type="textarea"
          @keydown.enter.prevent
          :placeholder="$gettext('Remark')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Create') }}
    </n-button>
  </app-modal>

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
