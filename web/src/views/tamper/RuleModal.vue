<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import tamper from '@/api/panel/tamper'
import website from '@/api/panel/website'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  rule?: any
}>()
const emit = defineEmits(['saved'])

const isEdit = computed(() => !!props.rule)

const model = ref({
  name: '',
  path: '',
  exts: [] as string[],
  excludes: [] as string[],
  enabled: true,
})

const websites = ref<{ label: string; value: string; path: string }[]>([])

const loadWebsites = () => {
  useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: any) => {
    websites.value = (data.items || []).map((item: any) => ({
      label: item.name,
      value: item.name,
      path: item.path,
    }))
  })
}

// 选择网站自动回填名称与路径
const handlePickWebsite = (value: string) => {
  const site = websites.value.find((w) => w.value === value)
  if (site) {
    model.value.name = site.value
    model.value.path = site.path
  }
}

watch(show, (val) => {
  if (!val) return
  loadWebsites()
  if (props.rule) {
    model.value = {
      name: props.rule.name,
      path: props.rule.path,
      exts: [...(props.rule.exts || [])],
      excludes: [...(props.rule.excludes || [])],
      enabled: props.rule.enabled,
    }
  } else {
    model.value = {
      name: '',
      path: '',
      exts: ['php', 'html', 'htm', 'js'],
      excludes: [],
      enabled: true,
    }
  }
})

const handleSubmit = async () => {
  const req = isEdit.value
    ? tamper.updateRule(props.rule.id, model.value)
    : tamper.createRule(model.value)
  await req
  window.$message.success($gettext('Saved successfully'))
  show.value = false
  emit('saved')
}
</script>

<template>
  <app-modal
    size="sm"
    v-model:show="show"
    :title="isEdit ? $gettext('Edit Rule') : $gettext('Add Rule')"
  >
    <n-form>
      <n-form-item v-if="!isEdit" :label="$gettext('Quick Select Website')">
        <n-select
          :options="websites"
          filterable
          clearable
          :placeholder="$gettext('Pick a website to fill name and path')"
          @update:value="handlePickWebsite"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Name')" required>
        <n-input
          v-model:value="model.name"
          :disabled="isEdit"
          :placeholder="$gettext('Rule identifier, usually the website name')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Protected Directory')" required>
        <n-input v-model:value="model.path" placeholder="/opt/ace/sites/example" />
      </n-form-item>
      <n-form-item :label="$gettext('Protected Extensions')">
        <n-dynamic-tags v-model:value="model.exts" />
      </n-form-item>
      <n-form-item :label="$gettext('Exclude Paths')">
        <n-dynamic-tags v-model:value="model.excludes" />
      </n-form-item>
      <n-form-item :label="$gettext('Enabled')">
        <n-switch v-model:value="model.enabled" />
      </n-form-item>
    </n-form>
    <n-alert type="info" :bordered="false">
      {{
        $gettext(
          'Leave extensions empty to protect all files (directories become append-only). Excludes accept a path segment name (e.g. cache) or an absolute path.',
        )
      }}
    </n-alert>
    <template #footer>
      <n-flex justify="end">
        <n-button @click="show = false">{{ $gettext('Cancel') }}</n-button>
        <app-button type="primary" @click="handleSubmit">
          {{ $gettext('Save') }}
        </app-button>
      </n-flex>
    </template>
  </app-modal>
</template>
