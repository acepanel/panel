<script setup lang="ts">
import { DateTime } from 'luxon'
import { useGettext } from 'vue3-gettext'

const props = defineProps<{
  // 服务器本地时间，格式 yyyy-MM-dd HH:mm:ss
  notBefore: string
  notAfter: string
  issuer: string
  domains: string[]
}>()

const { $gettext } = useGettext()

// 剩余天数不足时提醒，与证书自动续签提前的天数一致
const EXPIRING_DAYS = 30

const parse = (value: string) => DateTime.fromFormat(value, 'yyyy-MM-dd HH:mm:ss')
const start = computed(() => parse(props.notBefore))
const end = computed(() => parse(props.notAfter))

const state = computed(() => {
  const now = DateTime.now()
  const days = (date: DateTime) => Math.max(1, Math.ceil(Math.abs(date.diff(now, 'days').days)))
  if (now < start.value) {
    return {
      tone: 'warning',
      icon: 'mdi:shield-sync-outline',
      title: $gettext('Certificate is not yet valid'),
      detail: $gettext('Valid from %{ date }', { date: start.value.toFormat('yyyy-MM-dd') }),
    }
  }
  if (now >= end.value) {
    return {
      tone: 'error',
      icon: 'mdi:shield-off-outline',
      title: $gettext('Certificate has expired'),
      detail: $gettext('Expired %{ days } days ago', { days: days(end.value) }),
    }
  }
  const left = days(end.value)
  return {
    tone: left <= EXPIRING_DAYS ? 'warning' : 'success',
    icon: left <= EXPIRING_DAYS ? 'mdi:shield-alert-outline' : 'mdi:shield-check-outline',
    title:
      left <= EXPIRING_DAYS
        ? $gettext('Certificate expires soon')
        : $gettext('Certificate is valid'),
    detail: $gettext('Expires in %{ days } days', { days: left }),
  }
})

const toneClass = computed(
  () =>
    ({
      success: { badge: 'bg-success-subtle text-success', bar: 'success' },
      warning: { badge: 'bg-warning-subtle text-warning', bar: 'warning' },
      error: { badge: 'bg-error-subtle text-error', bar: 'error' },
    })[state.value.tone]!,
)

// 已过去的有效期占比
const elapsed = computed(() => {
  const total = end.value.diff(start.value).milliseconds
  if (total <= 0) return 100
  return Math.min(100, Math.max(0, (DateTime.now().diff(start.value).milliseconds / total) * 100))
})
</script>

<template>
  <n-card size="small" content-class="!p-0">
    <n-flex align="center" :size="12" :wrap="false" class="px-4 py-3.5">
      <div
        class="rounded-full flex flex-none size-10 items-center justify-center"
        :class="toneClass.badge"
      >
        <the-icon :icon="state.icon" :size="22" />
      </div>
      <div class="min-w-0">
        <div class="text-15px font-medium">{{ state.title }}</div>
        <div class="text-13px text-text-secondary">{{ state.detail }}</div>
      </div>
    </n-flex>
    <n-divider class="!my-0" />
    <n-grid cols="1 m:24" responsive="screen" :x-gap="24" :y-gap="14" class="px-4 py-3.5">
      <n-gi :span="9">
        <div class="text-12px text-text-tertiary mb-1">{{ $gettext('Validity Period') }}</div>
        <div class="text-13px tabular-nums" :title="`${notBefore} - ${notAfter}`">
          {{ start.toFormat('yyyy-MM-dd') }}
          <span class="text-text-tertiary mx-1">→</span>
          {{ end.toFormat('yyyy-MM-dd') }}
        </div>
        <n-progress
          type="line"
          :percentage="elapsed"
          :status="toneClass.bar as any"
          :show-indicator="false"
          :height="4"
          class="mt-2"
        />
      </n-gi>
      <n-gi :span="15">
        <div class="text-12px text-text-tertiary mb-1">{{ $gettext('Issuer') }}</div>
        <div class="text-13px truncate">{{ issuer }}</div>
      </n-gi>
      <n-gi :span="24">
        <div class="text-12px text-text-tertiary mb-1.5">
          {{ $gettext('Domains') }}
          <span class="ml-1 tabular-nums">{{ domains.length }}</span>
        </div>
        <n-flex :size="6">
          <n-tag v-for="domain in domains" :key="domain" size="small" :bordered="false">
            {{ domain }}
          </n-tag>
        </n-flex>
      </n-gi>
    </n-grid>
  </n-card>
</template>
