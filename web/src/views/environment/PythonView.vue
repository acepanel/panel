<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import pythonApi from '@/api/panel/environment/python'

const route = useRoute()
const slug = route.params.slug as string

const { $gettext } = useGettext()

const mirror = ref('')
const mirrorLoading = ref(false)

// 预设的镜像选项
const mirrorOptions = [
  { label: $gettext('Official (pypi.org)'), value: 'https://pypi.org/simple' },
  {
    label: $gettext('China - Alibaba (mirrors.aliyun.com)'),
    value: 'https://mirrors.aliyun.com/pypi/simple/',
  },
  {
    label: $gettext('China - Tencent (mirrors.tencent.com)'),
    value: 'https://mirrors.tencent.com/pypi/simple/',
  },
  {
    label: $gettext('China - Tsinghua (tuna.tsinghua.edu.cn)'),
    value: 'https://pypi.tuna.tsinghua.edu.cn/simple',
  },
  {
    label: $gettext('China - USTC (pypi.mirrors.ustc.edu.cn)'),
    value: 'https://pypi.mirrors.ustc.edu.cn/simple/',
  },
]

// 获取当前镜像设置
const fetchMirror = async () => {
  mirrorLoading.value = true
  useRequest(pythonApi.getMirror(slug))
    .onSuccess((res) => {
      mirror.value = res.data
    })
    .onComplete(() => {
      mirrorLoading.value = false
    })
}

onMounted(() => {
  fetchMirror()
})

const handleSetCli = async () => {
  useRequest(pythonApi.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}

const handleSaveMirror = async () => {
  useRequest(pythonApi.setMirror(slug, mirror.value)).onSuccess(() => {
    window.$message.success($gettext('Saved successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> Python {{ slug }} </template>
        <template #header-extra>
          <n-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </n-button>
        </template>
      </n-card>

      <n-card :title="$gettext('Mirror Settings')">
        <n-spin :show="mirrorLoading">
          <n-flex vertical>
            <n-alert type="info" :show-icon="false">
              {{
                $gettext(
                  'pip mirror is used to configure the Python package source. Using a domestic mirror can speed up package downloads.',
                )
              }}
            </n-alert>
            <n-form-item :label="$gettext('Mirror Address')">
              <n-select
                v-model:value="mirror"
                :options="mirrorOptions"
                filterable
                tag
                :placeholder="$gettext('Select or enter mirror address')"
              />
            </n-form-item>
            <n-flex>
              <n-button type="primary" @click="handleSaveMirror">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
          </n-flex>
        </n-spin>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
