<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import nodejsApi from '@/api/panel/environment/nodejs'

const route = useRoute()
const slug = route.params.slug as string

const { $gettext } = useGettext()

const registry = ref('')
const registryLoading = ref(false)

// 预设的镜像选项
const registryOptions = [
  { label: $gettext('Official (registry.npmjs.org)'), value: 'https://registry.npmjs.org/' },
  {
    label: $gettext('China - npmmirror (npmmirror.com)'),
    value: 'https://registry.npmmirror.com/',
  },
  {
    label: $gettext('China - Tencent (mirrors.tencent.com)'),
    value: 'https://mirrors.tencent.com/npm/',
  },
  {
    label: $gettext('China - Huawei (repo.huaweicloud.com)'),
    value: 'https://repo.huaweicloud.com/repository/npm/',
  },
]

// 获取当前镜像设置
const fetchRegistry = async () => {
  registryLoading.value = true
  useRequest(nodejsApi.getRegistry(slug))
    .onSuccess((res) => {
      registry.value = res.data
    })
    .onComplete(() => {
      registryLoading.value = false
    })
}

onMounted(() => {
  fetchRegistry()
})

const handleSetCli = async () => {
  useRequest(nodejsApi.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}

const handleSaveRegistry = async () => {
  useRequest(nodejsApi.setRegistry(slug, registry.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> Node.js {{ slug }} </template>
        <template #header-extra>
          <n-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </n-button>
        </template>
      </n-card>

      <n-card :title="$gettext('Registry Settings')">
        <n-spin :show="registryLoading">
          <n-flex vertical>
            <n-alert type="info" :show-icon="false">
              {{
                $gettext(
                  'npm registry is used to configure the npm package source. Using a domestic mirror can speed up package downloads.',
                )
              }}
            </n-alert>
            <n-form-item :label="$gettext('Registry Address')">
              <n-select
                v-model:value="registry"
                :options="registryOptions"
                filterable
                tag
                :placeholder="$gettext('Select or enter registry address')"
              />
            </n-form-item>
            <n-flex>
              <n-button type="primary" @click="handleSaveRegistry">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
          </n-flex>
        </n-spin>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
