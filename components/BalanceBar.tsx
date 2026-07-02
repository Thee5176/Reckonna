// BalanceBar — the 借方=貸方 gate (design §05). ok|bad is COMPUTED from
// money.isBalanced(debits, credits); there is deliberately NO "balanced"
// boolean prop, so the bar can never be told it is balanced when it isn't
// (IT3). Renders the three figures (借方 / 貸方 / Difference) + the status pill
// + an optional CTA slot that is disabled while unbalanced (AT1/AT2).
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';
import { sum, difference, isBalanced } from '../lib/money';
import { Button } from './Button';

export interface BalanceBarProps {
  debits: string[];
  credits: string[];
  // Optional CTA — the step-forward action, gated on balance (design §05).
  ctaLabel?: string;
  onCta?: () => void;
  // Compact "check" layout for statements (design §07): single cell + pill.
  variant?: 'full' | 'check';
  checkLabel?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

// Difference is shown as an absolute magnitude; sign is implied by ok|bad.
function absDisplay(value: string): string {
  const stripped = value.startsWith('-') ? value.slice(1) : value;
  return format2(stripped);
}

// Local 2dp grouped-free display for the big figures (design shows plain 2dp).
function format2(value: string): string {
  // value always comes from money.sum()/money.difference(), which render at
  // STORE_SCALE (4dp) and therefore always include a '.' — the `f = ''`
  // default is a defensive fallback unreachable through the public interface.
  /* istanbul ignore next */
  const [i, f = ''] = value.split('.');
  return `${i}.${(f + '00').slice(0, 2)}`;
}

export function BalanceBar({
  debits,
  credits,
  ctaLabel,
  onCta,
  variant = 'full',
  checkLabel = 'Check · assets − (liab + equity)',
  style,
  testID,
}: BalanceBarProps) {
  const balanced = isBalanced(debits, credits);
  const totalDebit = format2(sum(debits));
  const totalCredit = format2(sum(credits));
  const diff = absDisplay(difference(debits, credits));

  const tone = balanced ? styles.ok : styles.bad;

  if (variant === 'check') {
    return (
      <View
        testID={testID}
        accessibilityLabel={balanced ? 'balanced' : 'unbalanced'}
        style={[styles.bar, styles.check, tone, style]}
      >
        <View>
          <Text style={styles.k}>{checkLabel}</Text>
          <Text style={styles.v}>{diff}</Text>
        </View>
        <Pill balanced={balanced} />
      </View>
    );
  }

  return (
    <View
      testID={testID}
      accessibilityLabel={balanced ? 'balanced' : 'unbalanced'}
      style={[styles.bar, tone, style]}
    >
      <View style={styles.cell}>
        <Text style={styles.k}>借方 Debit</Text>
        <Text style={[styles.v, { color: color.debit }]}>{totalDebit}</Text>
      </View>
      <View style={styles.cell}>
        <Text style={styles.k}>貸方 Credit</Text>
        <Text style={[styles.v, { color: color.credit }]}>{totalCredit}</Text>
      </View>
      <View style={styles.cell}>
        <Text style={styles.k}>Difference</Text>
        <Text
          testID={`${testID ?? 'bar'}-diff`}
          style={[styles.v, !balanced && { color: color.debit }]}
        >
          {diff}
        </Text>
      </View>
      <View style={styles.tail}>
        <Pill balanced={balanced} />
        {ctaLabel ? (
          <Button
            testID={`${testID ?? 'bar'}-cta`}
            label={ctaLabel}
            variant="accent"
            disabled={!balanced}
            onPress={onCta}
          />
        ) : null}
      </View>
    </View>
  );
}

function Pill({ balanced }: { balanced: boolean }) {
  return (
    <View style={[styles.pill, balanced ? styles.pillOk : styles.pillBad]}>
      <Text style={[styles.pillText, { color: balanced ? color.credit : color.debit }]}>
        {balanced ? '✓ Balanced' : '✕ 借方≠貸方'}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 14,
    paddingVertical: 14,
    paddingHorizontal: 18,
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: color.hairline,
    backgroundColor: color.surface,
  },
  check: { justifyContent: 'space-between' },
  ok: {
    borderColor: withAlpha(color.credit, 0.4),
    backgroundColor: withAlpha(color.credit, 0.06),
  },
  bad: {
    borderColor: withAlpha(color.debit, 0.45),
    backgroundColor: withAlpha(color.debit, 0.06),
  },
  cell: { flex: 1 },
  k: { fontSize: 10, color: color.ink3, letterSpacing: 1, textTransform: 'uppercase', fontFamily: font.mono },
  v: { fontFamily: font.mono, fontSize: 18, fontWeight: '600', color: color.ink, marginTop: 2 },
  tail: { flexDirection: 'row', alignItems: 'center', gap: 10 },
  pill: {
    paddingVertical: 6,
    paddingHorizontal: 12,
    borderRadius: radius.sm,
    borderWidth: 1,
  },
  pillOk: { borderColor: withAlpha(color.credit, 0.4) },
  pillBad: { borderColor: withAlpha(color.debit, 0.45) },
  pillText: { fontFamily: font.mono, fontSize: 11, fontWeight: '600', letterSpacing: 0.4 },
});
