<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'
import { router } from '@/router'
import { useUserStore } from '@/stores'
import { renderIcon } from '@/utils'

const { $gettext } = useGettext()
const userStore = useUserStore()

const options = [
  {
    label: $gettext('Change Password'),
    key: 'changePassword',
    icon: renderIcon('mdi:key', { size: 14 }),
  },
  {
    label: $gettext('Logout'),
    key: 'logout',
    icon: renderIcon('mdi:exit-to-app', { size: 14 }),
  },
]

const handleSelect = (key: string) => {
  if (key === 'logout') {
    window.$dialog.info({
      content: $gettext('Confirm logout?'),
      title: $gettext('Prompt'),
      positiveText: $gettext('Confirm'),
      negativeText: $gettext('Cancel'),
      onPositiveClick() {
        user.logout().then(() => {
          userStore.logout()
        })
        window.$message.success($gettext('Logged out successfully!'))
      },
    })
  }
  if (key === 'changePassword') {
    router.push({ name: 'setting-index' })
  }
}

const username = computed(() => {
  if (userStore.username !== '') {
    return userStore.username
  }
  return $gettext('Unknown')
})
</script>

<template>
  <n-dropdown :options="options" @select="handleSelect">
    <div flex cursor-pointer items-center>
      <span text-lg>{{ username }}</span>
    </div>
  </n-dropdown>
</template>
