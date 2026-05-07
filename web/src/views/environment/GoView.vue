<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import goApi from '@/api/panel/environment/go'

const route = useRoute()
const slug = route.params.slug as string

const { $gettext } = useGettext()

const proxy = ref('')
const proxyLoading = ref(false)

// 预设的代理选项
const proxyOptions = [
  { label: $gettext('Official (proxy.golang.org)'), value: 'https://proxy.golang.org,direct' },
  { label: $gettext('China - Qiniu (goproxy.cn)'), value: 'https://goproxy.cn,direct' },
  {
    label: $gettext('China - Alibaba (mirrors.aliyun.com)'),
    value: 'https://mirrors.aliyun.com/goproxy/,direct',
  },
  {
    label: $gettext('China - Tencent (mirrors.tencent.com)'),
    value: 'https://mirrors.tencent.com/go/,direct',
  },
]

// 获取当前代理设置
const fetchProxy = async () => {
  proxyLoading.value = true
  useRequest(goApi.getProxy(slug))
    .onSuccess((res) => {
      proxy.value = res.data
    })
    .onComplete(() => {
      proxyLoading.value = false
    })
}

onMounted(() => {
  fetchProxy()
})

const handleSetCli = async () => {
  useRequest(goApi.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}

const handleSaveProxy = async () => {
  useRequest(goApi.setProxy(slug, proxy.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> Go {{ slug }} </template>
        <template #header-extra>
          <n-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </n-button>
        </template>
      </n-card>

      <n-card :title="$gettext('Proxy Settings')">
        <n-spin :show="proxyLoading">
          <n-flex vertical>
            <n-alert type="info" :show-icon="false">
              {{
                $gettext(
                  'GOPROXY is used to configure the Go module proxy. Using a domestic mirror can speed up dependency downloads.',
                )
              }}
            </n-alert>
            <n-form-item :label="$gettext('Proxy Address')">
              <n-select
                v-model:value="proxy"
                :options="proxyOptions"
                filterable
                tag
                :placeholder="$gettext('Select or enter proxy address')"
              />
            </n-form-item>
            <n-flex>
              <n-button type="primary" @click="handleSaveProxy">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
          </n-flex>
        </n-spin>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
