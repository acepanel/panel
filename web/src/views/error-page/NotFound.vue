<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const { replace } = useRouter()
</script>

<template>
  <PageContainer>
    <n-result
      class="m-auto"
      :description="$gettext('Sorry, the page you visited does not exist.')"
      status="404"
    >
      <template #icon>
        <img src="@/assets/images/404.webp" width="500" />
      </template>
      <template #footer>
        <n-button @click="replace('/')">{{ $gettext('Back to Home') }}</n-button>
      </template>
    </n-result>
  </PageContainer>
</template>
