<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import website from '@/api/panel/website'
import PathSelector from '@/components/common/PathSelector.vue'
import { generateRandomString } from '@/utils'

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
  { label: $gettext('Pure Static'), value: 'static' }
])
const createModel = ref({
  type: '',
  name: '',
  listens: [] as Array<string>,
  domains: [] as Array<string>,
  path: '',
  db: false,
  db_type: '0',
  db_name: '',
  db_user: '',
  db_password: '',
  remark: '',

  php: null,
  proxy: ''
})

const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace')
const loading = ref(false)

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    php: [
      {
        label: $gettext('Not used'),
        value: 0
      }
    ],
    db: [
      {
        label: '',
        value: ''
      }
    ]
  }
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

// 域名分隔符正则表达式（支持逗号、空格、换行分隔）
const DOMAIN_SEPARATORS_REGEX = /[\s,\n\r]+/

// 处理域名粘贴，支持批量添加
const handleDomainCreate = (index: number, value: string) => {
  if (DOMAIN_SEPARATORS_REGEX.test(value)) {
    // 解析多个域名并去除空白
    const domains = value.split(DOMAIN_SEPARATORS_REGEX).map((d) => d.trim()).filter((d) => d !== '')
    if (domains.length > 1) {
      // 移除当前空输入框
      createModel.value.domains.splice(index, 1)
      // 过滤掉已存在的域名，避免重复
      const existingDomains = new Set(createModel.value.domains.map((d) => d.trim()))
      const newDomains = domains.filter((d) => !existingDomains.has(d))
      // 将新域名添加到列表
      createModel.value.domains.push(...newDomains)
    }
  }
}

const handleCreate = async () => {
  createModel.value.type = effectiveType.value
  // 去除空的域名和端口
  createModel.value.domains = createModel.value.domains.filter((item) => item !== '')
  createModel.value.listens = createModel.value.listens.filter((item) => item !== '')
  // 端口为空自动添加 80 端口
  if (createModel.value.listens.length === 0) {
    createModel.value.listens.push('80')
  }
  // 端口中去掉 443 端口，不允许在未配置证书下监听 443 端口
  createModel.value.listens = createModel.value.listens.filter((item) => item !== '443')
  loading.value = true
  useRequest(website.create(createModel.value))
    .onSuccess(() => {
    window.$bus.emit('website:refresh')
    window.$message.success(
      $gettext('Website %{ name } created successfully', { name: createModel.value.name })
    )
    show.value = false
    createModel.value = {
      type: '',
      name: '',
      domains: [] as Array<string>,
      listens: [] as Array<string>,
      db: false,
      db_type: '0',
      db_name: '',
      db_user: '',
      db_password: '',
      path: '',
      remark: '',
      php: null,
      proxy: ''
    }
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
  <n-modal
    v-model:show="show"
    :title="modalTitle"
    preset="card"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
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
            $gettext('Must use English for the website name, it cannot be modified after setting')
          "
        />
      </n-form-item>
      <n-row :gutter="[0, 24]">
        <n-col :span="11">
          <n-form-item :label="$gettext('Domain')">
            <n-dynamic-input
              v-model:value="createModel.domains"
              placeholder="example.com"
              :min="1"
              show-sort-button
              @update:value="
                (value: any) => {
                  // 检查最后一个元素是否包含多个域名
                  if (value.length > 0) {
                    const lastIndex = value.length - 1
                    const lastValue = value[lastIndex]
                    if (lastValue && DOMAIN_SEPARATORS_REGEX.test(lastValue)) {
                      handleDomainCreate(lastIndex, lastValue)
                    }
                  }
                }
              "
            />
          </n-form-item>
        </n-col>
        <n-col :span="2"></n-col>
        <n-col :span="11">
          <n-form-item :label="$gettext('Port')">
            <n-dynamic-input
              v-model:value="createModel.listens"
              placeholder="80"
              :min="1"
              show-sort-button
            />
          </n-form-item>
        </n-col>
      </n-row>
      <n-row v-if="effectiveType == 'php'" :gutter="[0, 24]">
        <n-col :span="11">
          <n-form-item path="php" :label="$gettext('PHP Version')">
            <n-select
              v-model:value="createModel.php"
              :options="installedEnvironment.php"
              :placeholder="$gettext('Select PHP Version')"
              @keydown.enter.prevent
            >
            </n-select>
          </n-form-item>
        </n-col>
        <n-col :span="2"></n-col>
        <n-col :span="11">
          <n-form-item path="db" :label="$gettext('Database')">
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
          </n-form-item>
        </n-col>
      </n-row>
      <n-row v-if="effectiveType == 'php'" :gutter="[0, 24]">
        <n-col :span="7">
          <n-form-item v-if="createModel.db" path="db_name" :label="$gettext('Database Name')">
            <n-input
              v-model:value="createModel.db_name"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Database Name')"
            />
          </n-form-item>
        </n-col>
        <n-col :span="1"></n-col>
        <n-col :span="7">
          <n-form-item v-if="createModel.db" path="db_user" :label="$gettext('Database User')">
            <n-input
              v-model:value="createModel.db_user"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Database User')"
            />
          </n-form-item>
        </n-col>
        <n-col :span="1"></n-col>
        <n-col :span="8">
          <n-form-item
            v-if="createModel.db"
            path="db_password"
            :label="$gettext('Database Password')"
          >
            <n-input
              v-model:value="createModel.db_password"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Database Password')"
            />
          </n-form-item>
        </n-col>
      </n-row>
      <n-form-item v-if="effectiveType != 'proxy'" path="path" :label="$gettext('Directory')">
        <n-input-group>
          <n-input
            v-model:value="createModel.path"
            type="text"
            @keydown.enter.prevent
            :placeholder="
              $gettext(
                'Website root directory (if left empty, defaults to website directory/website name/public)'
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
  </n-modal>

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
