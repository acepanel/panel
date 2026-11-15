<script setup lang="ts">
import {
  NAvatar,
  NButton,
  NCard,
  NEllipsis,
  NFlex,
  NGrid,
  NGridItem,
  NInput,
  NPagination,
  NSpin,
  NTag,
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import template from '@/api/panel/template'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

import TemplateDeployModal from './TemplateDeployModal.vue'
import type { Template } from './types'

const { $gettext } = useGettext()

const selectedCategory = ref<string>('')
const searchQuery = ref<string>('')
const deployModalShow = ref(false)
const selectedTemplate = ref<Template | null>(null)

const { loading, data, page, total, pageSize, refresh, pagination } = usePagedTable(
  (page, pageSize) =>
    template.list(
      page,
      pageSize,
      selectedCategory.value || undefined,
      searchQuery.value || undefined,
    ),
  {
    initialPageSize: 12,
    pageSizes: [12, 24, 48, 96],
  },
)

// 获取所有分类
const { data: categories } = useRequest(app.categories, {
  initialData: [],
})

const getCategoryLabel = (catValue: string) => {
  const cat = categories.value.find((c: any) => c.value === catValue)
  return cat ? cat.label : catValue
}

const handleCategoryChange = (category: string) => {
  selectedCategory.value = category
  page.value = 1
  refresh()
}

watch(searchQuery, () => {
  page.value = 1
  refresh()
})

const handleDeploy = (tpl: Template) => {
  selectedTemplate.value = tpl
  deployModalShow.value = true
}
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex justify="space-between">
      <n-flex>
        <n-tag
          :type="selectedCategory === '' ? 'primary' : 'default'"
          :bordered="selectedCategory !== ''"
          style="cursor: pointer"
          @click="handleCategoryChange('')"
        >
          {{ $gettext('All') }}
        </n-tag>
        <n-tag
          v-for="cat in categories"
          :key="cat.value"
          :type="selectedCategory === cat.value ? 'primary' : 'default'"
          :bordered="selectedCategory !== cat.value"
          style="cursor: pointer"
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

    <n-spin :show="loading">
      <n-grid :x-gap="16" :y-gap="16" cols="1 s:2 m:3 l:4" responsive="screen">
        <n-grid-item v-for="tpl in data" :key="tpl.slug">
          <n-card hoverable style="height: 100%">
            <n-flex vertical :size="12">
              <n-flex justify="space-between" align="center">
                <n-flex align="center" :size="8">
                  <n-avatar
                    v-if="tpl.icon"
                    :src="tpl.icon"
                    :size="24"
                    style="background: transparent"
                  />
                  <span>{{ tpl.name }}</span>
                </n-flex>
                <n-button
                  v-if="tpl.website"
                  text
                  tag="a"
                  :href="tpl.website"
                  target="_blank"
                  type="primary"
                  size="small"
                >
                  <template #icon>
                    <i-mdi-open-in-new />
                  </template>
                </n-button>
              </n-flex>
              <n-ellipsis :line-clamp="2" :tooltip="{ width: 300 }">
                {{ tpl.description }}
              </n-ellipsis>
              <n-flex :size="4" class="mt-auto">
                <n-tag v-if="tpl.local" size="small" type="warning">
                  {{ $gettext('Local') }}
                </n-tag>
                <n-tag v-for="cat in tpl.categories" :key="cat" size="small">
                  {{ getCategoryLabel(cat) }}
                </n-tag>
              </n-flex>
            </n-flex>
            <template #action>
              <n-flex justify="end">
                <n-button size="small" type="primary" @click="handleDeploy(tpl)">
                  {{ $gettext('Deploy') }}
                </n-button>
              </n-flex>
            </template>
          </n-card>
        </n-grid-item>
      </n-grid>

      <n-empty v-if="!loading && data.length === 0" />
    </n-spin>

    <n-flex justify="end" v-if="total > 12">
      <n-pagination v-model:page="page" v-model:page-size="pageSize" v-bind="pagination" />
    </n-flex>
  </n-flex>

  <template-deploy-modal
    v-model:show="deployModalShow"
    :template="selectedTemplate"
    @success="refresh"
  />
</template>
