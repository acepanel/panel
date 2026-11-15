<script setup lang="ts">
/**
 * 通用键值对编辑器组件
 * 用于编辑 Object<string, string> 类型的数据
 */
defineOptions({
  name: 'KeyValueEditor',
})

const props = withDefaults(
  defineProps<{
    /** 键值对数据 */
    modelValue: Record<string, string>
    /** 键的占位符 */
    keyPlaceholder?: string
    /** 值的占位符 */
    valuePlaceholder?: string
    /** 添加按钮文本 */
    addButtonText?: string
    /** 新增项的默认键前缀 */
    defaultKeyPrefix?: string
    /** 新增项的默认值 */
    defaultValue?: string
    /** 键值分隔符显示 */
    separator?: string
    /** 值输入框类型 */
    valueType?: 'text' | 'password'
    /** 是否显示密码切换按钮 */
    showPasswordToggle?: boolean
  }>(),
  {
    defaultKeyPrefix: 'key',
    defaultValue: '',
    separator: '=',
    valueType: 'text',
    showPasswordToggle: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, string>]
}>()

// 生成唯一键名，优先使用前缀本身，仅在冲突时追加序号
const generateUniqueKey = () => {
  const data = props.modelValue || {}
  const prefix = props.defaultKeyPrefix
  if (data[prefix] === undefined) return prefix
  let i = 1
  while (data[`${prefix}${i}`] !== undefined) {
    i++
  }
  return `${prefix}${i}`
}

// 添加新项
const addItem = () => {
  const data = { ...props.modelValue }
  const key = generateUniqueKey()
  data[key] = props.defaultValue
  emit('update:modelValue', data)
}

// 更新键名（失焦时）
const updateKey = (oldKey: string, newKey: string) => {
  if (!newKey || newKey === oldKey) return
  if (props.modelValue[newKey] !== undefined) return // 键已存在

  const data = { ...props.modelValue }
  data[newKey] = data[oldKey] ?? ''
  delete data[oldKey]
  emit('update:modelValue', data)
}

// 更新值（失焦时）
const updateValue = (key: string, value: string) => {
  const data = { ...props.modelValue }
  data[key] = value
  emit('update:modelValue', data)
}

// 删除项
const removeItem = (key: string) => {
  const data = { ...props.modelValue }
  delete data[key]
  emit('update:modelValue', data)
}
</script>

<template>
  <n-flex vertical :size="8" w-full>
    <!-- 手机上键独占一行，值和删除按钮一行，组间多留间距以免分不清归属 -->
    <n-flex
      v-for="(value, key) in modelValue"
      :key="String(key)"
      :size="8"
      align="center"
      class="pb-2 md:pb-0"
    >
      <n-input
        :default-value="String(key)"
        :placeholder="keyPlaceholder ?? $gettext('Key')"
        class="flex-[1_1_100%] md:flex-1"
        @blur="(e: FocusEvent) => updateKey(String(key), (e.target as HTMLInputElement).value)"
      />
      <span class="shrink-0 hidden md:inline">{{ separator }}</span>
      <n-input
        :default-value="String(value)"
        :type="valueType"
        :show-password-on="showPasswordToggle ? 'click' : undefined"
        :placeholder="valuePlaceholder ?? $gettext('Value')"
        flex-1
        @blur="(e: FocusEvent) => updateValue(String(key), (e.target as HTMLInputElement).value)"
      />
      <n-button type="error" secondary size="small" flex-shrink-0 @click="removeItem(String(key))">
        {{ $gettext('Remove') }}
      </n-button>
    </n-flex>
    <n-button dashed size="small" @click="addItem">
      {{ addButtonText ?? $gettext('Add') }}
    </n-button>
  </n-flex>
</template>
