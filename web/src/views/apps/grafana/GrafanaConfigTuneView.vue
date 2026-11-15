<script setup lang="ts">
defineOptions({
  name: 'grafana-config-tune',
})

import { useGettext } from 'vue3-gettext'

import grafana from '@/api/apps/grafana'
import { toOptions, type TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const boolOptions = toOptions(['true', 'false'])

const groups: TuneGroup[] = [
  {
    name: 'server',
    tab: $gettext('Server'),
    tip: $gettext('Grafana server settings.'),
    fields: [
      {
        key: 'http_port',
        label: $gettext('HTTP Port (http_port)'),
        type: 'number',
        placeholder: $gettext('e.g. 3000'),
        min: 1,
        max: 65535,
      },
      {
        key: 'domain',
        label: $gettext('Domain (domain)'),
        type: 'text',
        placeholder: $gettext('e.g. localhost'),
      },
      {
        key: 'root_url',
        label: $gettext('Root URL (root_url)'),
        type: 'text',
        placeholder: $gettext('e.g. %(protocol)s://%(domain)s:%(http_port)s/'),
      },
      {
        key: 'protocol',
        label: $gettext('Protocol (protocol)'),
        type: 'select',
        options: toOptions(['http', 'https']),
      },
    ],
  },
  {
    name: 'database',
    tab: $gettext('Database'),
    tip: $gettext('Grafana database settings.'),
    fields: [
      {
        key: 'db_type',
        label: $gettext('Type (type)'),
        type: 'select',
        options: toOptions(['sqlite3', 'mysql', 'postgres']),
      },
      {
        key: 'db_host',
        label: $gettext('Host (host)'),
        type: 'text',
        placeholder: $gettext('e.g. 127.0.0.1:3306'),
      },
      {
        key: 'db_name',
        label: $gettext('Name (name)'),
        type: 'text',
        placeholder: $gettext('e.g. grafana'),
      },
      { key: 'db_user', label: $gettext('User (user)'), type: 'text' },
      { key: 'db_password', label: $gettext('Password (password)'), type: 'text', password: true },
    ],
  },
  {
    name: 'security',
    tab: $gettext('Security'),
    tip: $gettext('Grafana security settings.'),
    fields: [
      {
        key: 'admin_user',
        label: $gettext('Admin User (admin_user)'),
        type: 'text',
        placeholder: $gettext('e.g. admin'),
      },
      {
        key: 'admin_password',
        label: $gettext('Admin Password (admin_password)'),
        type: 'text',
        password: true,
      },
    ],
  },
  {
    name: 'users',
    tab: $gettext('Users'),
    tip: $gettext('Grafana user management settings.'),
    fields: [
      {
        key: 'allow_sign_up',
        label: $gettext('Allow Sign Up (allow_sign_up)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'auto_assign_org_role',
        label: $gettext('Auto Assign Org Role (auto_assign_org_role)'),
        type: 'select',
        options: toOptions(['Viewer', 'Editor', 'Admin']),
      },
    ],
  },
  {
    name: 'smtp',
    tab: 'SMTP',
    tip: $gettext('Grafana SMTP email settings.'),
    fields: [
      {
        key: 'smtp_enabled',
        label: $gettext('Enabled (enabled)'),
        type: 'select',
        options: boolOptions,
      },
      {
        key: 'smtp_host',
        label: $gettext('Host (host)'),
        type: 'text',
        placeholder: $gettext('e.g. smtp.example.com:587'),
      },
      { key: 'smtp_user', label: $gettext('User (user)'), type: 'text' },
      {
        key: 'smtp_password',
        label: $gettext('Password (password)'),
        type: 'text',
        password: true,
      },
      {
        key: 'smtp_from_address',
        label: $gettext('From Address (from_address)'),
        type: 'text',
        placeholder: $gettext('e.g. admin@example.com'),
      },
    ],
  },
  {
    name: 'log',
    tab: $gettext('Log'),
    tip: $gettext('Grafana log settings.'),
    fields: [
      {
        key: 'log_mode',
        label: $gettext('Mode (mode)'),
        type: 'select',
        options: toOptions(['console', 'file', 'console file']),
      },
      {
        key: 'log_level',
        label: $gettext('Level (level)'),
        type: 'select',
        options: toOptions(['debug', 'info', 'warn', 'error', 'critical']),
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="grafana.configTune" :save="grafana.saveConfigTune" :groups="groups" />
</template>
