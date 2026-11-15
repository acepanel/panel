<script setup lang="ts">
import { NDataTable, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import environment from '@/api/panel/environment'
import { useConfirm } from '@/components/system/composables/useConfirm'
import CustomForm from '@/views/app/CustomForm.vue'

import { useEnvColumns } from './composables/useEnvColumns'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()

const selectedType = ref<string>('')
const searchQuery = ref<string>('')

// 编译型环境的安装弹窗（含自定义编译参数输入）
const installModalShow = ref(false)
const installModalRow = ref<any>(null)
const installSubmitting = ref(false)
const customFormRef = ref<{ save: () => Promise<any> } | null>(null)

const { data: types } = useRequest(environment.types, {
  initialData: [],
})

const columns = useEnvColumns({
  showLatest: true,
  onInstall: async (row) => {
    // 编译型环境走带编译参数输入的安装弹窗
    if (row.custom_supported) {
      installModalRow.value = row
      installModalShow.value = true
      return
    }
    const ok = await confirmAction({
      type: 'info',
      title: $gettext('Confirm Install'),
      content: $gettext('Are you sure to install environment %{ environment }?', {
        environment: row.name,
      }),
    })
    if (ok) handleInstall(row.type, row.slug)
  },
})

const { loading, data } = useWatcher(
  () => environment.list(selectedType.value || undefined, searchQuery.value || undefined),
  [selectedType, searchQuery],
  { initialData: [], immediate: true },
)

const handleInstall = (type: string, slug: string) => {
  useRequest(environment.install(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  })
}

// 编译型环境安装：先保存自定义参数再触发安装
const handleInstallWithCustom = async () => {
  const row = installModalRow.value
  if (!row) return
  installSubmitting.value = true
  try {
    if (customFormRef.value) {
      await customFormRef.value.save()
    }
  } catch {
    installSubmitting.value = false
    return
  }
  useRequest(environment.install(row.type, row.slug))
    .onSuccess(() => {
      installModalShow.value = false
      window.$message.success(
        $gettext('Task submitted, please check the progress in background tasks'),
      )
    })
    .onComplete(() => {
      installSubmitting.value = false
    })
}
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between">
      <n-flex>
        <n-tag
          :type="selectedType === '' ? 'primary' : 'default'"
          :bordered="selectedType !== ''"
          class="cursor-pointer"
          @click="selectedType = ''"
        >
          {{ $gettext('All') }}
        </n-tag>
        <n-tag
          v-for="type in types"
          :key="type.value"
          :type="selectedType === type.value ? 'primary' : 'default'"
          :bordered="selectedType !== type.value"
          class="cursor-pointer"
          @click="selectedType = type.value"
        >
          {{ type.label }}
        </n-tag>
      </n-flex>
      <n-input
        v-model:value="searchQuery"
        :placeholder="$gettext('Search')"
        clearable
        class="md:!w-60"
      />
    </n-flex>
    <n-data-table
      striped
      :scroll-x="1280"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.slug"
    />
  </n-flex>
  <app-modal
    v-model:show="installModalShow"
    :title="$gettext('Install') + ' ' + (installModalRow?.name ?? '')"
  >
    <n-flex vertical>
      <!-- 环境的自定义参数目录名为 type+slug(如 php83),与安装脚本约定一致 -->
      <custom-form
        v-if="installModalRow"
        ref="customFormRef"
        :slug="installModalRow.type + installModalRow.slug"
      />
      <n-button
        type="info"
        block
        :loading="installSubmitting"
        :disabled="installSubmitting"
        @click="handleInstallWithCustom"
      >
        {{ $gettext('Install') }}
      </n-button>
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
