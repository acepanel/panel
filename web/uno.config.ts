import type { UserConfig } from 'unocss'
import { defineConfig, presetAttributify, presetWind4 } from 'unocss'
import { breakpoint, radius, typography, zIndex } from './src/design/tokens'

const config: UserConfig = {
  content: {
    pipeline: {
      exclude: ['node_modules', '.git', '.github', '.vscode', 'build', 'dist', 'public', 'types']
    }
  },
  presets: [presetWind4({ dark: 'class' }), presetAttributify()],

  theme: {
    colors: {
      'text-primary': 'var(--color-text-primary)',
      'text-secondary': 'var(--color-text-secondary)',
      'text-tertiary': 'var(--color-text-tertiary)',
      'text-disabled': 'var(--color-text-disabled)',
      'text-inverse': 'var(--color-text-inverse)',

      'bg-base': 'var(--color-bg-base)',
      'bg-elevated': 'var(--color-bg-elevated)',
      'bg-subtle': 'var(--color-bg-subtle)',
      'bg-inverse': 'var(--color-bg-inverse)',
      'bg-terminal': 'var(--color-bg-terminal)',

      'border-default': 'var(--color-border-default)',
      'border-strong': 'var(--color-border-strong)',
      'border-subtle': 'var(--color-border-subtle)',

      brand: {
        DEFAULT: 'var(--color-brand)',
        hover: 'var(--color-brand-hover)',
        pressed: 'var(--color-brand-pressed)',
        subtle: 'var(--color-brand-subtle)'
      },
      success: {
        DEFAULT: 'var(--color-success-fg)',
        subtle: 'var(--color-success-bg)'
      },
      warning: {
        DEFAULT: 'var(--color-warning-fg)',
        subtle: 'var(--color-warning-bg)'
      },
      error: {
        DEFAULT: 'var(--color-error-fg)',
        subtle: 'var(--color-error-bg)'
      },
      info: {
        DEFAULT: 'var(--color-info-fg)',
        subtle: 'var(--color-info-bg)'
      }
    },
    borderRadius: {
      none: radius.none,
      sm: radius.sm,
      DEFAULT: radius.md,
      md: radius.md,
      lg: radius.lg,
      xl: radius.xl,
      '2xl': radius['2xl'],
      full: radius.full
    },
    boxShadow: {
      none: 'none',
      xs: 'var(--shadow-xs)',
      sm: 'var(--shadow-sm)',
      DEFAULT: 'var(--shadow-md)',
      md: 'var(--shadow-md)',
      lg: 'var(--shadow-lg)',
      xl: 'var(--shadow-xl)',
      focus: 'var(--shadow-focus)'
    },
    fontSize: typography.fontSize,
    fontWeight: {
      regular: String(typography.fontWeight.regular),
      medium: String(typography.fontWeight.medium),
      semibold: String(typography.fontWeight.semibold),
      bold: String(typography.fontWeight.bold)
    },
    lineHeight: {
      tight: String(typography.lineHeight.tight),
      snug: String(typography.lineHeight.snug),
      normal: String(typography.lineHeight.normal),
      relaxed: String(typography.lineHeight.relaxed)
    },
    breakpoints: breakpoint,
    zIndex: Object.fromEntries(Object.entries(zIndex).map(([k, v]) => [k, String(v)]))
  },

  shortcuts: [
    /* 兼容老 class */
    ['wh-full', 'w-full h-full'],
    ['f-c-c', 'flex justify-center items-center'],
    ['flex-col', 'flex flex-col'],

    /* 页面骨架 */
    ['page-container', 'flex flex-col gap-4 p-4 lg:p-6 min-h-full'],
    ['page-header', 'flex items-start justify-between gap-4 flex-wrap'],
    ['page-title', 'text-xl font-semibold text-text-primary leading-tight'],
    ['page-subtitle', 'text-sm text-text-secondary mt-1'],

    /* 区块 */
    ['section-card', 'bg-bg-elevated rounded-lg border border-border-subtle p-5 shadow-xs'],
    ['section-title', 'text-base font-semibold text-text-primary'],

    /* 工具栏 / 空态 / hover */
    ['toolbar', 'flex items-center justify-between gap-3 flex-wrap'],
    ['empty-state', 'flex flex-col items-center justify-center gap-3 py-16 text-text-tertiary'],
    ['hover-bg', 'hover:bg-bg-subtle transition-colors']
  ],

  rules: [
    ['card-shadow', { 'box-shadow': 'var(--shadow-sm)' }],

    [
      /^duration-(fast|base|slow|instant)$/,
      ([, k]) => ({ 'transition-duration': `var(--duration-${k})` })
    ]
  ]
}

export default defineConfig(config)
