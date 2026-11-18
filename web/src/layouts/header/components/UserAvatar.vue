<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { router } from '@/router'
import { useUserStore } from '@/stores'
import { renderIcon } from '@/utils'

const { $gettext } = useGettext()
const userStore = useUserStore()
const { confirmAction } = useConfirm()

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
    confirmAction({
      type: 'info',
      title: $gettext('Prompt'),
      content: $gettext('Confirm logout?'),
      onConfirm: async () => {
        await user.logout()
        window.$message.success($gettext('Logged out successfully!'))
        userStore.logout()
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
      <span class="text-lg max-w-30 truncate">{{ username }}</span>
    </div>
  </n-dropdown>
</template>
