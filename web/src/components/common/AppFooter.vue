<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const year = new Date().getFullYear()
</script>

<template>
  <footer class="text-sm text-text-tertiary f-c-c flex-col gap-1">
    <p>
      © 2022 - {{ year }}
      <a
        class="hover:text-brand-hover hover:underline"
        rel="noreferrer"
        target="__blank"
        href="https://acepanel.net/"
      >
        {{ $gettext('AcePanel') }}
      </a>
      {{ $gettext('All Rights Reserved.') }}
    </p>
    <p class="flex gap-1 items-center">
      <a
        class="hover:text-brand-hover hover:underline"
        rel="noreferrer"
        target="_blank"
        href="https://jq.qq.com/?_wv=1027&k=I1oJKSTH"
      >
        {{ $gettext('QQ Group') }} 12370907
      </a>
      <n-divider vertical />
      <a
        class="hover:text-brand-hover hover:underline"
        rel="noreferrer"
        target="_blank"
        href="https://acepanel.net"
      >
        {{ $gettext('Website') }}
      </a>
      <n-divider vertical />
      <a
        class="hover:text-brand-hover hover:underline"
        rel="noreferrer"
        target="_blank"
        href="https://tom.moe/c/technical/acepanel"
      >
        {{ $gettext('Community') }}
      </a>
      <n-divider vertical />
      <a
        class="hover:text-brand-hover hover:underline"
        rel="noreferrer"
        target="_blank"
        href="https://afdian.com/a/tnborg"
      >
        {{ $gettext('Sponsor') }}
      </a>
    </p>
  </footer>
</template>
