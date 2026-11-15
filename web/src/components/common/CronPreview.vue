<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { classifyCron } from './cron'

const { $gettext } = useGettext()

const props = defineProps({
  cron: {
    type: String,
    required: true,
  },
})

// 星期名称映射
const weekdayNames: Record<string, string> = {
  '0': $gettext('Sunday'),
  '1': $gettext('Monday'),
  '2': $gettext('Tuesday'),
  '3': $gettext('Wednesday'),
  '4': $gettext('Thursday'),
  '5': $gettext('Friday'),
  '6': $gettext('Saturday'),
  '7': $gettext('Sunday'), // 有些系统用 7 表示周日
}

const pad = (value?: number) => String(value).padStart(2, '0')

// 生成人类可读描述
const description = computed((): string => {
  const cron = props.cron.trim()
  const p = classifyCron(cron)
  const n = String(p.n)
  const m = pad(p.minute)
  const time = `${pad(p.hour)}:${m}`
  const day = String(p.day)

  switch (p.type) {
    case 'reboot':
      return $gettext('Run once after system reboot')
    case 'every-second':
      return $gettext('Run every second')
    case 'every-n-seconds':
      return $gettext('Run every %{n} seconds', { n })
    case 'every-minute':
      return $gettext('Run every minute')
    case 'every-n-minutes':
      return $gettext('Run every %{n} minutes', { n })
    case 'every-n-hours':
      return $gettext('Run every %{n} hours at minute %{m}', { n, m })
    case 'every-n-days':
      return $gettext('Run every %{n} days at %{time}', { n, time })
    case 'every-hour':
      return $gettext('Run hourly at minute %{m}', { m })
    case 'every-day':
      return $gettext('Run daily at %{time}', { time })
    case 'every-week': {
      const weekday = String(p.weekday)
      return $gettext('Run weekly on %{weekday} at %{time}', {
        weekday: weekdayNames[weekday] ?? weekday,
        time,
      })
    }
    case 'every-month':
      return $gettext('Run monthly on day %{day} at %{time}', { day, time })
    case 'every-year':
      return $gettext('Run yearly on month %{month} day %{day} at %{time}', {
        month: String(p.month),
        day,
        time,
      })
    default:
      return $gettext('Cron expression: %{cron}', { cron })
  }
})
</script>

<template>
  <span>{{ description }}</span>
</template>

<style scoped lang="scss"></style>
