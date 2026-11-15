export type CronType =
  | 'every-second'
  | 'every-n-seconds'
  | 'every-minute'
  | 'every-n-minutes'
  | 'every-n-hours'
  | 'every-n-days'
  | 'every-hour'
  | 'every-day'
  | 'every-week'
  | 'every-month'
  | 'every-year'
  | 'reboot'
  | 'custom'

export interface CronPattern {
  type: CronType
  // every-n-* 的步长
  n?: number
  minute?: number
  hour?: number
  day?: number
  month?: number
  weekday?: number
}

// 字段形态：* 任意、n 纯数字、/ 步长（*/N），其余（列表、范围等）记作 ?
const shapeOf = (field: string) =>
  field === '*' ? '*' : /^\d+$/.test(field) ? 'n' : /^\*\/[1-9]\d*$/.test(field) ? '/' : '?'

// 各字段形态拼成的签名到模式的映射，不在表里的一律是 custom
const PATTERNS: Record<string, CronType> = {
  '* * * * * *': 'every-second',
  '/ * * * * *': 'every-n-seconds',
  '* * * * *': 'every-minute',
  '/ * * * *': 'every-n-minutes',
  'n / * * *': 'every-n-hours',
  'n n / * *': 'every-n-days',
  'n * * * *': 'every-hour',
  'n n * * *': 'every-day',
  'n n * * n': 'every-week',
  'n n n * *': 'every-month',
  'n n n n *': 'every-year',
}

const FIELDS = ['minute', 'hour', 'day', 'month', 'weekday'] as const

// 识别表达式属于哪种预设模式并取出其中的数值；表单无法无损表达的归为 custom
export function classifyCron(cron: string): CronPattern {
  const expr = cron.trim()
  if (expr === '@reboot') return { type: 'reboot' }

  const fields = expr.split(/\s+/)
  const type = PATTERNS[fields.map(shapeOf).join(' ')]
  if (!type) return { type: 'custom' }

  const pattern: CronPattern = { type }
  // 秒级表达式多一个前导字段，从末尾对齐到 分 时 日 月 周
  fields.slice(-5).forEach((field, i) => {
    const name = FIELDS[i]
    if (name && shapeOf(field) === 'n') pattern[name] = Number(field)
  })
  const step = fields.find((field) => shapeOf(field) === '/')
  if (step) pattern.n = Number(step.slice(2))
  return pattern
}
