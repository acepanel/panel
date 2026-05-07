<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import website from '@/api/panel/website'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const type = defineModel<string>('type', { type: String, required: true })

const { $gettext } = useGettext()

const bulkCreate = ref('')
const loading = ref(false)

// 内部选择的类型
const selectedType = ref('proxy')

// 实际使用的网站类型
const effectiveType = computed(() => {
  if (type.value === 'all') {
    return selectedType.value
  }
  return type.value
})

// 已安装环境列表
const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    php: [
      {
        label: $gettext('Not used'),
        value: 0,
      },
    ],
    db: [
      {
        label: '',
        value: '',
      },
    ],
  },
})

// 批量创建网站请求模型
interface BulkCreateModel {
  type: string
  name: string
  listens: Array<string>
  domains: Array<string>
  path: string
  proxy: string
  php: number
  remark: string
}

// 类型选项
const typeOptions = computed(() => [
  { label: $gettext('Reverse Proxy'), value: 'proxy' },
  { label: $gettext('PHP'), value: 'php' },
  { label: $gettext('Pure Static'), value: 'static' },
])

// 获取模态框标题
const modalTitle = computed(() => {
  switch (effectiveType.value) {
    case 'proxy':
      return $gettext('Bulk Create Reverse Proxy Website')
    case 'php':
      return $gettext('Bulk Create PHP Website')
    case 'static':
      return $gettext('Bulk Create Pure Static Website')
    default:
      return $gettext('Bulk Create Website')
  }
})

// 获取占位符文本
const placeholderText = computed(() => {
  if (effectiveType.value === 'proxy') {
    return $gettext('name|domain|port|proxy_target|remark')
  }
  if (effectiveType.value === 'php') {
    return $gettext('name|domain|port|path|php_version|remark')
  }
  return $gettext('name|domain|port|path|remark')
})

// 获取第四列的说明文本
const fourthColumnHelp = computed(() => {
  if (effectiveType.value === 'proxy') {
    return $gettext(
      'Proxy Target: The target address for reverse proxy (e.g., http://127.0.0.1:3000).',
    )
  }
  return $gettext('Path: The path of the website, can be empty to use the default path.')
})

// 已安装 PHP 版本的提示文本
const phpVersionHelp = computed(() => {
  const items = (installedEnvironment.value?.php ?? [])
    .map((item: { label: string; value: number }) => `${item.label}=${item.value}`)
    .join(', ')
  return $gettext('PHP Version: %{ items }', { items })
})

const handleCreate = async () => {
  // 按行分割
  const lines = bulkCreate.value.split('\n')
  // 去除空行
  const filteredLines = lines.filter((line) => line.trim() !== '')
  if (filteredLines.length === 0) return
  // PHP 类型需要额外的 php_version 列，最少 5 列
  const minParts = effectiveType.value === 'php' ? 5 : 4
  loading.value = true
  let remaining = filteredLines.length
  // 解析每一行
  for (const line of filteredLines) {
    const parts = line.split('|')
    if (parts.length < minParts) {
      window.$message.error($gettext('The format is incorrect, please check'))
      loading.value = false
      return
    }
    // 去除空格
    const name = (parts[0] ?? '').trim()
    const domains = (parts[1] ?? '')
      .trim()
      .split(',')
      .map((item) => item.trim())
    const listens = (parts[2] ?? '')
      .trim()
      .split(',')
      .map((item) => item.trim())
    const fourthColumn = (parts[3] ?? '').trim()
    // PHP 类型：第五列是 php 版本，第六列是备注；其他类型：第五列是备注
    const isPHP = effectiveType.value === 'php'
    const php = isPHP ? Number((parts[4] ?? '').trim()) || 0 : 0
    const remark = (parts[isPHP ? 5 : 4] ?? '').trim()

    // 构建请求模型
    const model: BulkCreateModel = {
      type: effectiveType.value,
      name: name,
      listens: listens,
      domains: domains,
      path: effectiveType.value === 'proxy' ? '' : fourthColumn,
      proxy: effectiveType.value === 'proxy' ? fourthColumn : '',
      php: php,
      remark: remark,
    }

    // 去除空的域名和端口
    model.domains = model.domains.filter((item) => item !== '')
    model.listens = model.listens.filter((item) => item !== '')
    // 端口为空自动添加 80 端口
    if (model.listens.length === 0) {
      model.listens.push('80')
    }
    // 端口中去掉 443 端口，nginx 不允许在未配置证书下监听 443 端口
    model.listens = model.listens.filter((item) => item !== '443')
    useRequest(website.create(model))
      .onSuccess(() => {
        window.$message.success(
          $gettext('Website %{ name } created successfully', { name: model.name }),
        )
        window.$bus.emit('website:refresh')
      })
      .onComplete(() => {
        remaining--
        if (remaining <= 0) loading.value = false
      })
  }
}
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
    <n-flex vertical>
      <n-form-item v-if="type === 'all'" :label="$gettext('Website Type')">
        <n-select
          v-model:value="selectedType"
          :options="typeOptions"
          :placeholder="$gettext('Select Website Type')"
        />
      </n-form-item>
      <n-alert type="info">
        {{
          $gettext(
            'Please enter the website name, domain, port, path, and remark in the text area below, one per line.',
          )
        }}
      </n-alert>
      <n-input
        type="textarea"
        :autosize="{ minRows: 10, maxRows: 15 }"
        :placeholder="placeholderText"
        v-model:value="bulkCreate"
      />
      <n-text>
        {{
          $gettext(
            'Name: The name of the website, which will be displayed in the website list, must be unique.',
          )
        }}
      </n-text>
      <n-text>
        {{
          $gettext(
            'Domain: The domain name of the website, multiple domains can be separated by commas.',
          )
        }}
      </n-text>
      <n-text>
        {{
          $gettext(
            'Port: The port number of the website, multiple ports can be separated by commas.',
          )
        }}
      </n-text>
      <n-text>
        {{ fourthColumnHelp }}
      </n-text>
      <n-text v-if="effectiveType === 'php'">
        {{ phpVersionHelp }}
      </n-text>
      <n-text>
        {{ $gettext('Remark: The remark of the website, can be empty.') }}
      </n-text>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
        {{ $gettext('Create') }}
      </n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
