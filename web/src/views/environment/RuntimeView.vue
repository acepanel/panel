<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import dotnetApi from '@/api/panel/environment/dotnet'
import goApi from '@/api/panel/environment/go'
import javaApi from '@/api/panel/environment/java'
import nodejsApi from '@/api/panel/environment/nodejs'
import pythonApi from '@/api/panel/environment/python'

interface Runtime {
  title: string
  subtitle?: string
  description?: string
  setCli: (slug: string) => any
  // 依赖下载源配置，没有的语言不渲染对应卡片
  mirror?: {
    get: (slug: string) => any
    set: (slug: string, value: string) => any
    title: string
    tip: string
    label: string
    placeholder: string
    options: { label: string; value: string }[]
  }
}

const { $gettext } = useGettext()

const runtimes: Record<'go' | 'nodejs' | 'python' | 'java' | 'dotnet', Runtime> = {
  go: {
    title: 'Go',
    setCli: goApi.setCli,
    mirror: {
      get: goApi.getProxy,
      set: goApi.setProxy,
      title: $gettext('Proxy Settings'),
      tip: $gettext(
        'GOPROXY is used to configure the Go module proxy. Using a domestic mirror can speed up dependency downloads.',
      ),
      label: $gettext('Proxy Address'),
      placeholder: $gettext('Select or enter proxy address'),
      options: [
        {
          label: $gettext('Official (proxy.golang.org)'),
          value: 'https://proxy.golang.org,direct',
        },
        { label: $gettext('China - Qiniu (goproxy.cn)'), value: 'https://goproxy.cn,direct' },
        {
          label: $gettext('China - Alibaba (mirrors.aliyun.com)'),
          value: 'https://mirrors.aliyun.com/goproxy/,direct',
        },
        {
          label: $gettext('China - Tencent (mirrors.tencent.com)'),
          value: 'https://mirrors.tencent.com/go/,direct',
        },
      ],
    },
  },
  nodejs: {
    title: 'Node.js',
    setCli: nodejsApi.setCli,
    mirror: {
      get: nodejsApi.getRegistry,
      set: nodejsApi.setRegistry,
      title: $gettext('Registry Settings'),
      tip: $gettext(
        'npm registry is used to configure the npm package source. Using a domestic mirror can speed up package downloads.',
      ),
      label: $gettext('Registry Address'),
      placeholder: $gettext('Select or enter registry address'),
      options: [
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
      ],
    },
  },
  python: {
    title: 'Python',
    setCli: pythonApi.setCli,
    mirror: {
      get: pythonApi.getMirror,
      set: pythonApi.setMirror,
      title: $gettext('Mirror Settings'),
      tip: $gettext(
        'pip mirror is used to configure the Python package source. Using a domestic mirror can speed up package downloads.',
      ),
      label: $gettext('Mirror Address'),
      placeholder: $gettext('Select or enter mirror address'),
      options: [
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
      ],
    },
  },
  java: {
    title: 'Java',
    subtitle: '(Amazon Corretto)',
    description: $gettext(
      'Amazon Corretto is a no-cost, multiplatform, production-ready distribution of the Open Java Development Kit (OpenJDK).',
    ),
    setCli: javaApi.setCli,
  },
  dotnet: {
    title: '.NET',
    description: $gettext(
      '.NET is a free, open-source, cross-platform framework for building modern apps and powerful cloud services.',
    ),
    setCli: dotnetApi.setCli,
  },
}

// 各语言路由共用本组件，AppMain 以 route.path 为 key，换语言或版本都会重建实例
const route = useRoute()
const slug = route.params.slug as string
const runtime = runtimes[route.meta.runtime as keyof typeof runtimes]

const mirror = ref('')
const mirrorLoading = ref(false)

if (runtime.mirror) {
  mirrorLoading.value = true
  useRequest(runtime.mirror.get(slug))
    .onSuccess(({ data }) => {
      mirror.value = data
    })
    .onComplete(() => {
      mirrorLoading.value = false
    })
}

const handleSetCli = async () => {
  await runtime.setCli(slug)
  window.$message.success($gettext('Set successfully'))
}

const handleSaveMirror = async () => {
  await runtime.mirror!.set(slug, mirror.value)
  window.$message.success($gettext('Saved successfully'))
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> {{ runtime.title }} {{ slug }} {{ runtime.subtitle }} </template>
        <template #header-extra>
          <app-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </app-button>
        </template>
        <n-alert v-if="runtime.description" type="info" :show-icon="false">
          {{ runtime.description }}
        </n-alert>
      </n-card>

      <n-card v-if="runtime.mirror" :title="runtime.mirror.title">
        <n-spin :show="mirrorLoading">
          <n-flex vertical>
            <n-alert type="info" :show-icon="false">
              {{ runtime.mirror.tip }}
            </n-alert>
            <n-form-item :label="runtime.mirror.label">
              <n-select
                v-model:value="mirror"
                :options="runtime.mirror.options"
                filterable
                tag
                :placeholder="runtime.mirror.placeholder"
              />
            </n-form-item>
            <n-flex>
              <app-button type="primary" @click="handleSaveMirror">
                {{ $gettext('Save') }}
              </app-button>
            </n-flex>
          </n-flex>
        </n-spin>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
