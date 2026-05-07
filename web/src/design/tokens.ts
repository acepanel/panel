/* === 中性色阶 === */
export const neutral = {
  50: '#f8fafc',
  100: '#f1f5f9',
  200: '#e2e8f0',
  300: '#cbd5e1',
  400: '#94a3b8',
  500: '#64748b',
  600: '#475569',
  700: '#334155',
  800: '#1e293b',
  900: '#0f172a',
  950: '#020617',
} as const

/* === 品牌色 === */
export const primary = {
  50: '#e8f7ee',
  100: '#d1f0dd',
  200: '#a8e0c1',
  300: '#7dd3a3',
  400: '#36ad6a',
  500: '#18a058',
  600: '#0e9249',
  700: '#0c7a43',
  800: '#0a5e35',
  900: '#073d23',
  950: '#04261a',
} as const

export const success = {
  50: '#f0fdf4',
  100: '#dcfce7',
  300: '#86efac',
  400: '#4ade80',
  500: '#22c55e',
  600: '#16a34a',
  700: '#15803d',
  800: '#166534',
} as const

export const warning = {
  50: '#fffbeb',
  100: '#fef3c7',
  300: '#fcd34d',
  400: '#fbbf24',
  500: '#f59e0b',
  600: '#d97706',
  700: '#b45309',
  800: '#92400e',
} as const

export const error = {
  50: '#fef2f2',
  100: '#fee2e2',
  300: '#fca5a5',
  400: '#f87171',
  500: '#ef4444',
  600: '#dc2626',
  700: '#b91c1c',
  800: '#991b1b',
} as const

export const info = {
  50: '#f0f9ff',
  100: '#e0f2fe',
  300: '#7dd3fc',
  400: '#38bdf8',
  500: '#0ea5e9',
  600: '#0284c7',
  700: '#0369a1',
  800: '#075985',
} as const

export interface SemanticPalette {
  textPrimary: string
  textSecondary: string
  textTertiary: string
  textDisabled: string
  textInverse: string

  bgBase: string
  bgElevated: string
  bgSubtle: string
  bgInverse: string
  bgTerminal: string

  borderDefault: string
  borderStrong: string
  borderSubtle: string

  brand: string
  brandHover: string
  brandPressed: string
  brandSubtle: string

  successFg: string
  successBg: string
  warningFg: string
  warningBg: string
  errorFg: string
  errorBg: string
  infoFg: string
  infoBg: string

  overlay: string
}

/* === 语义色(light) === */
export const lightSemantic: SemanticPalette = {
  textPrimary: neutral[900],
  textSecondary: neutral[600],
  textTertiary: neutral[400],
  textDisabled: neutral[300],
  textInverse: '#ffffff',

  bgBase: neutral[50],
  bgElevated: '#ffffff',
  bgSubtle: neutral[100],
  bgInverse: neutral[900],
  bgTerminal: '#0a0e1a',

  borderDefault: neutral[200],
  borderStrong: neutral[300],
  borderSubtle: neutral[100],

  brand: primary[500],
  brandHover: primary[600],
  brandPressed: primary[700],
  brandSubtle: primary[50],

  successFg: success[600],
  successBg: success[50],
  warningFg: warning[600],
  warningBg: warning[50],
  errorFg: error[600],
  errorBg: error[50],
  infoFg: info[600],
  infoBg: info[50],

  overlay: 'rgba(15,23,42,.45)',
}

/* === 语义色(dark) === */
export const darkSemantic: SemanticPalette = {
  textPrimary: neutral[50],
  textSecondary: neutral[300],
  textTertiary: neutral[400],
  textDisabled: neutral[600],
  textInverse: neutral[900],

  bgBase: '#0b0f17',
  bgElevated: '#151b27',
  bgSubtle: '#1e2533',
  bgInverse: neutral[50],
  bgTerminal: '#05080f',

  borderDefault: '#1f2937',
  borderStrong: '#374151',
  borderSubtle: '#151b27',

  brand: primary[400],
  brandHover: primary[300],
  brandPressed: primary[500],
  brandSubtle: 'rgba(54,173,106,.14)',

  successFg: success[400],
  successBg: 'rgba(34,197,94,.12)',
  warningFg: warning[400],
  warningBg: 'rgba(245,158,11,.12)',
  errorFg: error[400],
  errorBg: 'rgba(239,68,68,.12)',
  infoFg: info[400],
  infoBg: 'rgba(14,165,233,.12)',

  overlay: 'rgba(0,0,0,.6)',
}

/* === 圆角 === */
export const radius = {
  none: '0',
  sm: '4px',
  md: '6px',
  lg: '8px',
  xl: '12px',
  '2xl': '16px',
  full: '9999px',
} as const

/* === 阴影 === */
export const shadowLight = {
  none: 'none',
  xs: '0 1px 2px 0 rgba(15,23,42,.04)',
  sm: '0 1px 3px 0 rgba(15,23,42,.06),0 1px 2px -1px rgba(15,23,42,.04)',
  md: '0 4px 8px -2px rgba(15,23,42,.06),0 2px 4px -2px rgba(15,23,42,.04)',
  lg: '0 12px 20px -4px rgba(15,23,42,.08),0 4px 8px -4px rgba(15,23,42,.04)',
  xl: '0 24px 40px -8px rgba(15,23,42,.12),0 8px 16px -8px rgba(15,23,42,.06)',
  focus: '0 0 0 3px rgba(24,160,88,.20)',
} as const

export const shadowDark = {
  none: 'none',
  xs: '0 1px 2px 0 rgba(0,0,0,.30)',
  sm: '0 1px 3px 0 rgba(0,0,0,.40),0 1px 2px -1px rgba(0,0,0,.30)',
  md: '0 4px 8px -2px rgba(0,0,0,.45),0 2px 4px -2px rgba(0,0,0,.35)',
  lg: '0 12px 20px -4px rgba(0,0,0,.55),0 4px 8px -4px rgba(0,0,0,.40)',
  xl: '0 24px 40px -8px rgba(0,0,0,.65),0 8px 16px -8px rgba(0,0,0,.45)',
  focus: '0 0 0 3px rgba(54,173,106,.25)',
} as const

/* === 字体 === */
export const typography = {
  fontFamily: {
    base: '"Inter","PingFang SC","Microsoft YaHei",system-ui,sans-serif',
    mono: '"JetBrains Mono Variable","Consolas","Monaco",monospace',
  },
  fontSize: {
    xs: '12px',
    sm: '13px',
    base: '14px',
    md: '15px',
    lg: '16px',
    xl: '18px',
    '2xl': '20px',
    '3xl': '24px',
    '4xl': '32px',
  },
  fontWeight: {
    regular: 400,
    medium: 500,
    semibold: 600,
    bold: 700,
  },
  lineHeight: {
    tight: 1.25,
    snug: 1.4,
    normal: 1.5,
    relaxed: 1.6,
  },
  letterSpacing: {
    tight: '-0.01em',
    normal: '0',
    wide: '0.025em',
  },
} as const

/* === 动效 === */
export const transition = {
  duration: {
    instant: '80ms',
    fast: '150ms',
    base: '220ms',
    slow: '320ms',
  },
  easing: {
    standard: 'cubic-bezier(.2,0,0,1)',
    emphasized: 'cubic-bezier(.3,0,0,1.05)',
    decelerate: 'cubic-bezier(0,0,.2,1)',
    accelerate: 'cubic-bezier(.4,0,1,1)',
  },
} as const

/* === Z-Index === */
export const zIndex = {
  base: 0,
  dropdown: 1000,
  sticky: 1100,
  drawer: 1200,
  modal: 1300,
  popover: 1400,
  tooltip: 1500,
  notification: 1600,
  loadingBar: 1700,
} as const

/* === 响应式断点 === */
export const breakpoint = {
  sm: '640px',
  md: '768px',
  lg: '1024px',
  xl: '1280px',
  '2xl': '1536px',
} as const

export const tokens = {
  color: { neutral, primary, success, warning, error, info },
  semantic: { light: lightSemantic, dark: darkSemantic },
  radius,
  shadow: { light: shadowLight, dark: shadowDark },
  typography,
  transition,
  zIndex,
  breakpoint,
} as const

export type Tokens = typeof tokens
export type SemanticColors = SemanticPalette
