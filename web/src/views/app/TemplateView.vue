<script setup lang="ts">
import {
  NAvatar,
  NButton,
  NCard,
  NEllipsis,
  NFlex,
  NGrid,
  NGridItem,
  NPagination,
  NSpin,
  NTag
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import template from '@/api/panel/template'
import TemplateDeployModal from './TemplateDeployModal.vue'
import type { Template } from './types'

const { $gettext } = useGettext()

const selectedCategory = ref<string>('')
const deployModalShow = ref(false)
const selectedTemplate = ref<Template | null>(null)

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => template.list(page, pageSize, selectedCategory.value || undefined),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 12,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

// 获取所有分类
const { data: categories } = useRequest(app.categories, {
  initialData: []
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

const handleDeploy = (tpl: Template) => {
  selectedTemplate.value = tpl
  deployModalShow.value = true
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
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
              <n-flex :size="4" style="margin-top: auto">
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
      <n-pagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :page-count="pageCount"
        :item-count="total"
        show-quick-jumper
        show-size-picker
        :page-sizes="[12, 24, 48, 96]"
      />
    </n-flex>
  </n-flex>

  <template-deploy-modal
    v-model:show="deployModalShow"
    :template="selectedTemplate"
    @success="refresh"
  />
</template>
