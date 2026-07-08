// Maps Forest Reserve tokens → a react-native-paper MD3 theme so Paper's
// built-in surfaces (used by AppNav's bottom bar, etc.) inherit the ledger
// palette instead of Material's defaults. Colors come only from tokens.ts —
// no new value is introduced here (design §01). Fonts point at the bundled
// Source Serif 4 (display/headings) + JetBrains Mono (body/data).
import { MD3LightTheme, configureFonts } from 'react-native-paper';
import type { MD3Theme } from 'react-native-paper';
import { color, font } from './tokens';

const fontConfig = {
  // Data + UI chrome default to mono; headings override to serif per-component.
  default: { fontFamily: font.mono, fontWeight: '400' as const },
  displayLarge: { fontFamily: font.serif, fontWeight: '500' as const },
  displayMedium: { fontFamily: font.serif, fontWeight: '500' as const },
  displaySmall: { fontFamily: font.serif, fontWeight: '500' as const },
  headlineLarge: { fontFamily: font.serif, fontWeight: '500' as const },
  headlineMedium: { fontFamily: font.serif, fontWeight: '500' as const },
  headlineSmall: { fontFamily: font.serif, fontWeight: '500' as const },
  titleLarge: { fontFamily: font.serif, fontWeight: '500' as const },
};

export const paperTheme: MD3Theme = {
  ...MD3LightTheme,
  roundness: 1, // MD3 multiplies by 4 → ~4px; radii stay sm/md (design §01)
  fonts: configureFonts({ config: fontConfig }),
  colors: {
    ...MD3LightTheme.colors,
    primary: color.ink,
    onPrimary: color.bg,
    secondary: color.accent,
    onSecondary: color.bg,
    tertiary: color.credit,
    error: color.debit,
    onError: color.bg,
    background: color.bg,
    onBackground: color.ink,
    surface: color.surface,
    onSurface: color.ink,
    surfaceVariant: color.bgElev,
    onSurfaceVariant: color.ink3,
    outline: color.rule,
    outlineVariant: color.hairline,
    elevation: {
      ...MD3LightTheme.colors.elevation,
      level0: 'transparent',
      level1: color.surface,
      level2: color.surface,
    },
  },
};
