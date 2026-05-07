import type { GlobalThemeOverrides } from 'naive-ui'

import { darkSemantic, lightSemantic, typography, type SemanticPalette } from './tokens'

const buildOverrides = (sem: SemanticPalette): GlobalThemeOverrides => ({
  common: {
    fontFamily: typography.fontFamily.base,
    fontFamilyMono: typography.fontFamily.mono,
    primaryColor: sem.brand,
    primaryColorHover: sem.brandHover,
    primaryColorPressed: sem.brandPressed,
    primaryColorSuppl: sem.brandHover,
  },
})

export const lightThemeOverrides = buildOverrides(lightSemantic)
export const darkThemeOverrides = buildOverrides(darkSemantic)
