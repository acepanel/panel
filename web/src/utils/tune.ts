// 配置调优表单的字段定义与数值转换
import type { TuneProfile } from '@/utils/tunepreset'

export interface TuneField {
  // 后端请求体的键
  key: string
  label: string
  type: 'text' | 'number' | 'size' | 'select' | 'switch'
  placeholder?: string
  min?: number
  max?: number
  units?: string[]
  defaultUnit?: string
  options?: { label: string; value: string }[]
  password?: boolean
  // 字段上方的小标题
  heading?: string
}

export interface TuneGroup {
  name: string
  tab: string
  tip?: string
  tipType?: 'info' | 'warning'
  fields: TuneField[]
}

export interface TunePreset {
  fields: ('memory' | 'cpu' | 'disk' | 'connections')[]
  scenarioOptions?: { label: string; value: string }[]
  memoryRatio: number
  // 返回值与后端同格式，只需包含要覆盖的键
  apply: (profile: TuneProfile) => Record<string, any>
}

export interface SizeValue {
  num: number | null
  unit: string
}

// 值与显示文本相同的下拉选项
export const toOptions = (items: string[]) => items.map((item) => ({ label: item, value: item }))

// 容量单位选项，多出的空单位项用于原样回写不带单位的值；
// 列表外的单位（大小写不同、TB 之类）由下拉按当前值原样显示
export const unitOptions = (units: string[]) =>
  [...units, ''].map((unit) => ({ label: unit || '-', value: unit }))

// 只有空串算未设置，0 是有效值，否则保存时会被当成未设置注释掉
export const parseNumber = (val: unknown): number | null => {
  const str = String(val ?? '').trim()
  if (str === '') return null
  const num = Number(str)
  return Number.isFinite(num) ? num : null
}

// 拆出末尾的字母单位，如 "50M" -> { num: 50, unit: "M" }，单位不限于预设列表
export const parseSize = (val: unknown, defaultUnit: string): SizeValue => {
  const [, head, unit] = String(val ?? '').match(/^(.*?)([a-z]*)$/is)!
  const num = parseNumber(head)
  // 纯数字本身不带单位，补上默认单位会改变它的大小
  return { num, unit: num == null ? defaultUnit : unit! }
}

// 组合数值和单位，如 50 + "M" -> "50M"
export const composeSize = (num: number | null, unit: string): string => {
  return num == null ? '' : `${num}${unit}`
}

const within = (num: number | null, min?: number, max?: number) =>
  num == null || ((min == null || num >= min) && (max == null || num <= max))

// 后端值转控件值。控件写回去与原文不同、或超出范围会被控件钳制的值，
// 返回原文交给文本框，否则不改动直接保存也会改写它或把它清空（等于注释掉该项）
export const decodeNumber = (val: unknown, min?: number, max?: number): number | null | string => {
  const str = String(val ?? '')
  const num = parseNumber(str)
  return String(num ?? '') === str && within(num, min, max) ? num : str
}

export const decodeSize = (
  val: unknown,
  defaultUnit: string,
  min?: number,
  max?: number,
): SizeValue | string => {
  const str = String(val ?? '')
  const size = parseSize(str, defaultUnit)
  return composeSize(size.num, size.unit) === str && within(size.num, min, max) ? size : str
}

export const encodeNumber = (val: number | null | string): string => String(val ?? '')

export const encodeSize = (val: SizeValue | string): string =>
  typeof val === 'string' ? val : composeSize(val.num, val.unit)
