<script setup lang="ts">
defineOptions({
  name: 'home-update'
})

import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import { router } from '@/router'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { data: versions } = useRequest(home.updateInfo, {
  initialData: []
})
let messageReactive: MessageReactive | null = null
const updateLoading = ref(false)

const handleUpdate = () => {
  window.$dialog.warning({
    title: $gettext('Update Panel'),
    content: $gettext('Are you sure you want to update the panel?'),
    positiveText: $gettext('Confirm'),
    negativeText: $gettext('Cancel'),
    onPositiveClick: () => {
      updateLoading.value = true
      messageReactive = window.$message.loading($gettext('Panel updating...'), {
        duration: 0
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
    }
  })
}
</script>

<template>
  <common-page show-footer>
    <n-timeline v-if="versions" pt-10>
      <n-timeline-item
        v-for="(item, index) in versions"
        :type="Number(index) == 0 ? 'info' : 'default'"
        :key="index"
        :title="item.version"
        :time="formatDateTime(item.updated_at)"
      >
        <MdPreview
          v-model="item.description"
          noMermaid
          noKatex
          noIconfont
          noHighlight
          noImgZoomIn
        />
      </n-timeline-item>
      <n-button class="ml-16" type="primary" :loading="updateLoading" :disabled="updateLoading" @click="handleUpdate">
        {{ $gettext('Update Now') }}
      </n-button>
    </n-timeline>
    <div v-else pt-40>
      <n-result
        status="418"
        title="Loading..."
        :description="$gettext('Loading update information, please wait a moment')"
      />
    </div>
  </common-page>
</template>
