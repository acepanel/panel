<script setup lang="ts">
defineOptions({
  name: 'config-tune-form',
})

import { useGettext } from 'vue3-gettext'

import { useResponsive } from '@/components/system/composables/useResponsive'
import {
  decodeNumber,
  decodeSize,
  encodeNumber,
  encodeSize,
  type TuneField,
  type TuneGroup,
  type TunePreset,
  unitOptions,
} from '@/utils/tune'
import type { TuneProfile } from '@/utils/tunepreset'

const props = defineProps<{
  fetch: () => any
  save: (data: Record<string, any>) => any
  // 只有一组时平铺，多组时按左侧标签页展示
  groups: TuneGroup[]
  preset?: TunePreset
}>()

const { $gettext } = useGettext()
const { isMobile } = useResponsive()
const [DefineGroup, ReuseGroup] = createReusableTemplate<{ group: TuneGroup }>()

const fields = props.groups.flatMap((group) => group.fields)
// 每个标签页各有一个保存按钮，共用状态才能在切换标签页后仍禁止重复保存
const saveLoading = ref(false)
const showPresetModal = ref(false)

// 选项数组只建一次，每次渲染都换新数组会让下拉反复重建
const units = Object.fromEntries(fields.map((field) => [field.key, unitOptions(field.units ?? [])]))

// 后端值转控件值，number 和 size 的控件表示不了时得到原文字符串
const decode = (field: TuneField, val: any) => {
  switch (field.type) {
    case 'number':
      return decodeNumber(val, field.min, field.max)
    case 'size':
      return decodeSize(val, field.defaultUnit ?? '', field.min, field.max)
    case 'select':
      return val || null
    case 'switch':
      return Boolean(val)
    default:
      return val ?? ''
  }
}

// 控件值转后端值，空串表示注释掉该配置项
const encode = (field: TuneField, val: any) => {
  switch (field.type) {
    case 'number':
      return encodeNumber(val)
    case 'size':
      return encodeSize(val)
    case 'select':
      return val ?? ''
    default:
      return val
  }
}

const decodeFields = (list: TuneField[], data: Record<string, any>) =>
  Object.fromEntries(list.map((field) => [field.key, decode(field, data[field.key])]))

const model = ref<Record<string, any>>(decodeFields(fields, {}))

// 保留着原文的 number / size 字段改用文本框编辑
const isRaw = (field: TuneField) => typeof model.value[field.key] === 'string'

useRequest(props.fetch()).onSuccess(({ data }: any) => {
  model.value = decodeFields(fields, data)
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(
    props.save(
      Object.fromEntries(fields.map((field) => [field.key, encode(field, model.value[field.key])])),
    ),
  )
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}

const handlePreset = (profile: TuneProfile) => {
  const values = props.preset!.apply(profile)
  Object.assign(
    model.value,
    decodeFields(
      fields.filter((field) => field.key in values),
      values,
    ),
  )
  window.$message.success($gettext('Generated, review the values and save'))
}
</script>

<template>
  <div>
    <DefineGroup v-slot="{ group }">
      <n-flex vertical>
        <n-alert v-if="group.tip" :type="group.tipType ?? 'info'">
          {{ group.tip }}
        </n-alert>
        <n-form>
          <template v-for="field in group.fields" :key="field.key">
            <n-h3 v-if="field.heading" prefix="bar">{{ field.heading }}</n-h3>
            <n-form-item :label="field.label">
              <n-input-number
                v-if="field.type === 'number' && !isRaw(field)"
                class="w-full"
                v-model:value="model[field.key]"
                :placeholder="field.placeholder"
                :min="field.min"
                :max="field.max"
              />
              <n-input-group v-else-if="field.type === 'size' && !isRaw(field)">
                <n-input-number
                  class="w-full"
                  v-model:value="model[field.key].num"
                  :placeholder="field.placeholder"
                  :min="field.min"
                  :max="field.max"
                  style="flex: 1"
                />
                <n-select
                  v-model:value="model[field.key].unit"
                  :options="units[field.key]"
                  class="w-20"
                />
              </n-input-group>
              <n-select
                v-else-if="field.type === 'select'"
                v-model:value="model[field.key]"
                :options="field.options"
                clearable
              />
              <n-switch v-else-if="field.type === 'switch'" v-model:value="model[field.key]" />
              <n-input
                v-else
                v-model:value="model[field.key]"
                :type="field.password ? 'password' : 'text'"
                :show-password-on="field.password ? 'click' : undefined"
                :placeholder="field.placeholder"
              />
            </n-form-item>
          </template>
        </n-form>
        <n-flex>
          <n-button
            type="primary"
            :loading="saveLoading"
            :disabled="saveLoading"
            @click="handleSave"
          >
            {{ $gettext('Save') }}
          </n-button>
          <n-button v-if="preset" type="info" @click="showPresetModal = true">
            {{ $gettext('Generate Recommended Configuration') }}
          </n-button>
        </n-flex>
      </n-flex>
    </DefineGroup>
    <n-tabs v-if="groups.length > 1" type="line" :placement="isMobile ? 'top' : 'left'" animated>
      <n-tab-pane v-for="group in groups" :key="group.name" :name="group.name" :tab="group.tab">
        <ReuseGroup :group="group" />
      </n-tab-pane>
    </n-tabs>
    <ReuseGroup v-else :group="groups[0]!" />
    <tune-preset-modal
      v-if="preset"
      v-model:show="showPresetModal"
      :fields="preset.fields"
      :scenario-options="preset.scenarioOptions"
      :memory-ratio="preset.memoryRatio"
      @generate="handlePreset"
    />
  </div>
</template>
