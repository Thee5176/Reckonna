// Token-derived color utility. The design system composites the ledger colors
// over transparent via CSS color-mix(... , transparent) — e.g. badge borders,
// segment fills, balance-bar washes. RN has no color-mix, so we derive the same
// intent as an rgba() of an EXISTING token hex. No NEW hue is introduced
// (design §01) — only alpha variants of tokens.color values.
import { color } from './tokens';

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#', '');
  return [
    parseInt(h.slice(0, 2), 16),
    parseInt(h.slice(2, 4), 16),
    parseInt(h.slice(4, 6), 16),
  ];
}

// Alpha variant of a token hex, e.g. withAlpha(color.debit, 0.3).
export function withAlpha(hex: string, alpha: number): string {
  const [r, g, b] = hexToRgb(hex);
  return `rgba(${r},${g},${b},${alpha})`;
}

// Semantic pair for a debit|credit type — the only place color is chosen
// (design §03). Kept here so every component reads the same source.
export function forType(type: 'debit' | 'credit'): string {
  return type === 'debit' ? color.debit : color.credit;
}
