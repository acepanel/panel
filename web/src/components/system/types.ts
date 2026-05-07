import type { DataTableColumn, FormRules, PopoverPlacement } from 'naive-ui'
import type { VNode } from 'vue'

export type ActionType = 'default' | 'primary' | 'success' | 'warning' | 'error' | 'info'

export type StatusType =
  | 'success'
  | 'warning'
  | 'error'
  | 'info'
  | 'neutral'
  | 'running'
  | 'stopped'
  | 'partial'

export interface ActionItem<T = any> {
  key: string
  label: string
  type?: ActionType
  icon?: string
  variant?: 'button' | 'icon' | 'text'
  show?: (row: T) => boolean
  disabled?: boolean | ((row: T) => boolean)
  confirm?: {
    type?: 'normal' | 'danger' | 'delete'
    title?: string
    content: string | ((row: T) => string)
    countdown?: number
    placement?: PopoverPlacement
  }
  onClick: (row: T) => void | Promise<void>
}

export interface ToolbarAction {
  key: string
  label: string
  type?: ActionType
  icon?: string
  disabled?: boolean | ((selected: any[]) => boolean)
  loading?: boolean
  onClick: () => void | Promise<void>
}

export interface BatchAction {
  key: string
  label: string
  type?: ActionType
  icon?: string
  confirm?: {
    type?: 'normal' | 'danger' | 'delete'
    title?: string
    content: string
    countdown?: number
  }
}

export interface SearchOption {
  label: string
  value: any
}

export interface SearchField {
  key: string
  label?: string
  type: 'input' | 'select' | 'date-range'
  options?: SearchOption[]
  placeholder?: string
  width?: number | string
  defaultValue?: any
}

export interface PaginationFetchParams {
  page: number
  pageSize: number
  query: Record<string, any>
}

export interface PaginationFetchResult<T = any> {
  items: T[]
  total: number
}

export type FetchFn<T = any> = (params: PaginationFetchParams) => Promise<PaginationFetchResult<T>>

export interface MoreActionOption {
  key: string
  label: string
  icon?: string
  type?: ActionType
  disabled?: boolean
  divider?: boolean
}

export type FormSize = 'sm' | 'md' | 'lg' | 'xl'

export const formWidthMap: Record<FormSize, number> = {
  sm: 480,
  md: 640,
  lg: 840,
  xl: 1080,
}

export type { DataTableColumn, FormRules, PopoverPlacement, VNode }
