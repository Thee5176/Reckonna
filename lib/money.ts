// Decimal-exact money — strings end-to-end, mirroring the backend
// NUMERIC(20,4) column. NEVER parse money into a JS `number` for arithmetic:
// IEEE-754 float is exactly the precision bug plan 03 killed on the backend
// (Double → NUMERIC). All math here runs on BigInt fixed-point at STORE_SCALE.
//
// Rounding policy: HALF AWAY FROM ZERO (a.k.a. "half-up" for positives),
// matching Postgres NUMERIC ROUND(). Documented so IT2 / plan-03 AT11 are
// unambiguous. Values are stored at 4dp; display rounds to 2dp.

const STORE_SCALE = 4; // NUMERIC(20,4)
const DISPLAY_SCALE = 2;

// Parse a decimal string into a BigInt scaled to `scale` dp, rounding the
// discarded tail half away from zero. Throws on non-numeric input (fail at the
// boundary — plan invariant "validate early").
function parseScaled(input: string, scale: number): bigint {
  const trimmed = input.trim();
  const m = /^([+-]?)(\d*)(?:\.(\d+))?$/.exec(trimmed);
  if (!m || (m[2] === '' && (m[3] ?? '') === '')) {
    throw new Error(`money: not a decimal string: ${JSON.stringify(input)}`);
  }
  const negative = m[1] === '-';
  const intPart = m[2] || '0';
  const fracPart = m[3] || '';

  const kept = fracPart.slice(0, scale).padEnd(scale, '0');
  let value = BigInt(intPart + kept);

  // Round using the first discarded digit (half away from zero).
  const roundDigit = fracPart.charCodeAt(scale) - 48; // NaN→negative if absent
  if (roundDigit >= 5) {
    value += 1n;
  }
  return negative ? -value : value;
}

// Re-scale a BigInt from `from` dp to `to` dp, rounding half away from zero.
function rescale(value: bigint, from: number, to: number): bigint {
  // Both call sites (format/formatGrouped) only ever go STORE_SCALE(4) →
  // DISPLAY_SCALE(2), so the widen-scale branch is unreachable through the
  // public API. Kept as a defensive general-purpose guard.
  /* istanbul ignore next */
  if (to >= from) return value * 10n ** BigInt(to - from);
  const factor = 10n ** BigInt(from - to);
  const q = value / factor;
  const r = value % factor;
  const half = factor / 2n;
  const absR = r < 0n ? -r : r;
  if (absR >= half) return value < 0n ? q - 1n : q + 1n;
  return q;
}

// Render a scaled BigInt back to a fixed-point decimal string at `scale` dp.
function renderScaled(value: bigint, scale: number, grouped = false): string {
  const negative = value < 0n;
  const digits = (negative ? -value : value).toString().padStart(scale + 1, '0');
  const cut = digits.length - scale;
  let intPart = digits.slice(0, cut);
  const fracPart = digits.slice(cut);
  if (grouped) intPart = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  // renderScaled is only ever called at STORE_SCALE(4) or DISPLAY_SCALE(2) —
  // both > 0 — so the whole-number (scale === 0) branch is unreachable
  // through the public API. Kept as a defensive general-purpose guard.
  /* istanbul ignore next */
  const body = scale > 0 ? `${intPart}.${fracPart}` : intPart;
  return negative ? `-${body}` : body;
}

// Normalize an input to the 4dp store value string (e.g. '0.12345' → '0.1235').
export function toValue(input: string): string {
  return renderScaled(parseScaled(input, STORE_SCALE), STORE_SCALE);
}

// Display string at 2dp (no grouping) — the IT2-asserted core format.
export function format(input: string): string {
  const stored = parseScaled(input, STORE_SCALE);
  return renderScaled(rescale(stored, STORE_SCALE, DISPLAY_SCALE), DISPLAY_SCALE);
}

// Display string at 2dp with thousands separators (the design's 4,200.00 look).
export function formatGrouped(input: string): string {
  const stored = parseScaled(input, STORE_SCALE);
  return renderScaled(rescale(stored, STORE_SCALE, DISPLAY_SCALE), DISPLAY_SCALE, true);
}

// Exact sum of decimal strings → 4dp value string. '0.1'+'0.2' === '0.3000'.
export function sum(values: string[]): string {
  const total = values.reduce((acc, v) => acc + parseScaled(v, STORE_SCALE), 0n);
  return renderScaled(total, STORE_SCALE);
}

// Signed 借方 − 貸方 at 4dp. Zero exactly when balanced.
export function difference(debits: string[], credits: string[]): string {
  const d = debits.reduce((a, v) => a + parseScaled(v, STORE_SCALE), 0n);
  const c = credits.reduce((a, v) => a + parseScaled(v, STORE_SCALE), 0n);
  return renderScaled(d - c, STORE_SCALE);
}

// 借方 = 貸方 iff the exact 4dp sums are equal. A 0.0001 mismatch is unbalanced.
export function isBalanced(debits: string[], credits: string[]): boolean {
  const d = debits.reduce((a, v) => a + parseScaled(v, STORE_SCALE), 0n);
  const c = credits.reduce((a, v) => a + parseScaled(v, STORE_SCALE), 0n);
  return d === c;
}
