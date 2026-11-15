<script setup lang="ts">
defineOptions({
  name: 'app-index',
})

import { NDataTable, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import VersionModal from '@/views/app/VersionModal.vue'

import { useAppColumns } from './composables/useAppColumns'

const { $gettext } = useGettext()

const versionModalShow = ref(false)
const versionModalOperation = ref($gettext('Install'))
const versionModalInfo = ref<any>({})

// 当前选中的分类
const selectedCategory = ref<string>('')
const searchQuery = ref<string>('')

const columns = useAppColumns({
  onInstall: (row) => {
    versionModalShow.value = true
    versionModalOperation.value = $gettext('Install')
    versionModalInfo.value = row
  },
})

const { data: categories } = useRequest(app.categories, {
  initialData: [],
})

const { loading, data, page, pageSize, pagination } = usePagedTable(
  (page, pageSize) =>
    app.list(page, pageSize, selectedCategory.value || undefined, searchQuery.value || undefined),
  {
    watchingStates: [selectedCategory, searchQuery],
  },
)

// 处理分类切换
const handleCategoryChange = (category: string) => {
  selectedCategory.value = category
  page.value = 1
}
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between">
      <n-flex>
        <n-tag
          :type="selectedCategory === '' ? 'primary' : 'default'"
          :bordered="selectedCategory !== ''"
          class="cursor-pointer"
          @click="handleCategoryChange('')"
        >
          {{ $gettext('All') }}
        </n-tag>
        <n-tag
          v-for="cat in categories"
          :key="cat.value"
          :type="selectedCategory === cat.value ? 'primary' : 'default'"
          :bordered="selectedCategory !== cat.value"
          class="cursor-pointer"
          @click="handleCategoryChange(cat.value)"
        >
          {{ cat.label }}
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
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1380"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.slug"
      :pagination="pagination"
    />
  </n-flex>
  <version-modal
    v-model:show="versionModalShow"
    v-model:operation="versionModalOperation"
    v-model:info="versionModalInfo"
  />
</template>
