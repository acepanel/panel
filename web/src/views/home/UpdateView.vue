<script setup lang="ts">
defineOptions({
  name: 'home-update',
})

import type { MessageReactive } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import { router } from '@/router'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { data: versions } = useRequest(home.updateInfo, {
  initialData: [],
})

let messageReactive: MessageReactive | null = null
const updateLoading = ref(false)

// 解析描述文本为列表项
const parseDescription = (text: string): string[] => {
  return text.split('\n').filter((line) => line.trim())
}

const handleUpdate = () => {
  window.$dialog.warning({
    title: $gettext('Update Panel'),
    content: $gettext('Are you sure you want to update the panel?'),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      updateLoading.value = true
      messageReactive = window.$message.loading($gettext('Panel updating...'), {
        duration: 0,
      })
      useRequest(home.update())
        .onSuccess(() => {
          setTimeout(() => {
            setTimeout(() => {
              window.location.reload()
            }, 400)
            router.push({ name: 'home-index' })
          }, 2500)
          window.$message.success($gettext('Panel updated successfully'))
        })
        .onComplete(() => {
          updateLoading.value = false
          messageReactive?.destroy()
        })
    },
    onNegativeClick: () => {
      window.$message.info($gettext('Update canceled'))
    },
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-list v-if="versions.length" hoverable>
      <n-list-item v-for="(item, index) in versions" :key="index">
        <n-thing>
          <template #header>
            <div class="flex gap-3 items-center">
              <span>v{{ item.version }}</span>
              <n-tag v-if="index === 0" type="success" size="small" :bordered="false">
                {{ $gettext('Latest') }}
              </n-tag>
              <n-tag size="small" :bordered="false">
                {{ item.type }}
              </n-tag>
            </div>
          </template>
          <template #header-extra>
            <n-button
              v-if="index === 0"
              type="primary"
              :loading="updateLoading"
              :disabled="updateLoading"
              @click="handleUpdate"
            >
              {{ $gettext('Update Now') }}
            </n-button>
          </template>
          <template #description>
            <n-text depth="3">
              {{ formatDateTime(item.updated_at) }}
            </n-text>
          </template>
          <n-ol p-0>
            <n-li v-for="(line, i) in parseDescription(item.description)" :key="i">
              {{ line }}
            </n-li>
          </n-ol>
        </n-thing>
      </n-list-item>
    </n-list>
    <div v-else pt-10>
      <n-result
        status="418"
        title="Loading..."
        :description="$gettext('Loading update information, please wait a moment')"
      />
    </div>
  </PageContainer>
</template>
