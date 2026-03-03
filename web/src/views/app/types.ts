export interface App {
  name: string
  description: string
  slug: string
  channels: Channel[]
  installed: boolean
  installed_channel: string
  installed_version: string
  update_exist: boolean
  show: boolean
}

export interface Channel {
  slug: string
  name: string
  panel: string
  version: string
  log: string
}

export interface TemplateEnvironment {
  name: string
  description: string
  type: 'text' | 'url' | 'password' | 'number' | 'port' | 'select'
  options?: Record<string, string>
  default: string
}

export interface Template {
  created_at: string
  updated_at: string
  slug: string
  icon: string
  name: string
  description: string
  website: string
  categories: string[]
  version: string
  compose: string
  environments: TemplateEnvironment[]
  local?: boolean
}
