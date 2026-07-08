// Forest Reserve design tokens — ported 1:1 from the feature design system
// :root block (design/02-frontend-ledger.design-system.html, lines 19–29).
// This module is the single typed source of truth for the RN/Expo app; the
// token-drift test (tokens.test.ts, IT1) fails the build if any value drifts
// from the HTML. NO new color or type is invented here (design §01 rule):
// the only additions are the state aliases mapped onto existing semantics.

export const color = {
  // surfaces
  bg: '#efece1',
  bgElev: '#e2dccb',
  surface: '#faf7ec',
  // ink (text)
  ink: '#11201a',
  ink2: '#324138',
  ink3: '#6d6b58',
  // rules / hairlines
  rule: '#1b2a23',
  ruleSoft: 'rgba(17,32,26,0.12)',
  hairline: 'rgba(17,32,26,0.07)',
  // ledger semantics
  debit: '#7a1d1d', // 借方 · outflow · over
  credit: '#2a5a2a', // 貸方 · inflow · under
  accent: '#8a5a1c', // primary CTA on warm surfaces
  highlight: '#e9d391', // focus ring · active row
  // state aliases — mapped onto existing semantics (design §01)
  ok: '#2a5a2a', // ≡ credit
  warn: '#8a5a1c', // ≡ accent
  bad: '#7a1d1d', // ≡ debit
} as const;

// Font family names as registered by @expo-google-fonts (bundled offline, not
// the Google CDN <link> the HTML preview uses). The design's --serif/--mono
// stacks fall back to these first entries.
export const font = {
  serif: 'Source Serif 4',
  mono: 'JetBrains Mono',
} as const;

// Radii — sm/md/lg only. No pill shapes; this is a ledger (design §01).
export const radius = {
  sm: 4,
  md: 6,
  lg: 10,
} as const;

// Shadows — mirror --sh-1 / --sh-2. Kept as web boxShadow strings (RN Web);
// native shadow props are derived per-component where needed.
export const shadow = {
  sh1: '0 1px 0 rgba(17,32,26,0.04), 0 1px 2px rgba(17,32,26,0.05)',
  sh2: '0 1px 0 rgba(17,32,26,0.06), 0 6px 18px -8px rgba(17,32,26,0.18)',
} as const;

export const tokens = { color, font, radius, shadow } as const;

export type Tokens = typeof tokens;
export type ColorToken = keyof typeof color;
