<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import type { FileInfo } from '@/views/file/types'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const fileInfo = defineModel<FileInfo | null>('fileInfo', { type: Object, default: null })

const title = computed(() => {
  if (!fileInfo.value) return $gettext('Properties')
  return $gettext('Properties - %{ name }', { name: fileInfo.value.name })
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title"
    style="width: 500px"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-descriptions bordered :column="1" label-placement="left" v-if="fileInfo">
      <n-descriptions-item :label="$gettext('Name')">
        {{ fileInfo.name }}
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Full Path')">
        {{ fileInfo.full }}
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Type')">
        {{ fileInfo.dir ? $gettext('Directory') : $gettext('File') }}
        <template v-if="fileInfo.symlink">
          ({{ $gettext('Symlink') }} -> {{ fileInfo.link }})
        </template>
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Size')" v-if="!fileInfo.dir">
        {{ fileInfo.size }}
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Permission')">
        {{ fileInfo.mode_str }} ({{ fileInfo.mode }})
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Owner')">
        {{ fileInfo.owner }} (UID: {{ fileInfo.uid }})
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Group')">
        {{ fileInfo.group }} (GID: {{ fileInfo.gid }})
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Modification Time')">
        {{ fileInfo.modify }}
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Hidden')">
        {{ fileInfo.hidden ? $gettext('Yes') : $gettext('No') }}
      </n-descriptions-item>
      <n-descriptions-item :label="$gettext('Immutable')">
        <n-tag :type="fileInfo.immutable ? 'warning' : 'default'" size="small">
          {{ fileInfo.immutable ? $gettext('Yes') : $gettext('No') }}
        </n-tag>
      </n-descriptions-item>
    </n-descriptions>
  </n-modal>
</template>

<style scoped lang="scss"></style>
