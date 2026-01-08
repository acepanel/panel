<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const year = new Date().getFullYear()
</script>

<template>
  <footer color="#6a6a6a"    text-14 f-c-c flex-col >
    <p>
      © 2022 - {{ year }}
      <a hover="decoration-primary color-primary" target="__blank" href="https://acepanel.net/">
        {{ $gettext('AcePanel') }}
      </a>
      {{ $gettext('All Rights Reserved.') }}
    </p>
    <p>
      <a
        hover="decoration-primary color-primary"
        target="_blank"
        href="https://jq.qq.com/?_wv=1027&k=I1oJKSTH"
      >
        {{ $gettext('QQ Group') }} 12370907
      </a>
      <n-divider vertical />
      <a
        hover="decoration-primary color-primary"
        target="_blank"
        href="https://acepanel.net/docs"
      >
        {{ $gettext('Documentation') }}
      </a>
      <n-divider vertical />
      <a
        hover="decoration-primary color-primary"
        target="_blank"
        href="https://tom.moe/c/technical/panel"
      >
        {{ $gettext('Community') }}
      </a>
      <n-divider vertical />
      <a
        hover="decoration-primary color-primary"
        target="_blank"
        href="https://afdian.com/a/tnborg"
      >
        {{ $gettext('Sponsor') }}
      </a>
    </p>
  </footer>
</template>
