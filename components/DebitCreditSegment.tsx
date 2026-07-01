// DebitCreditSegment — the 借方/貸方 toggle (design §03). This is the ONLY place
// the entry type is chosen; it drives both the color and the balance sign, so
// AmountInput / BalanceBar downstream read off this value. Default 借方.
// Presentational: value in, onChange('debit'|'credit') out (AT8).
import React from 'react';
import { View, Pressable, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';

export type EntryType = 'debit' | 'credit';

export interface DebitCreditSegmentProps {
  value?: EntryType;
  onChange?: (value: EntryType) => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

const SEGMENTS: { key: EntryType; label: string }[] = [
  { key: 'debit', label: '借方 Debit' },
  { key: 'credit', label: '貸方 Credit' },
];

export function DebitCreditSegment({
  value = 'debit',
  onChange,
  style,
  testID,
}: DebitCreditSegmentProps) {
  return (
    <View testID={testID} style={[styles.seg, style]}>
      {SEGMENTS.map((s, i) => {
        const active = value === s.key;
        const tint = s.key === 'debit' ? color.debit : color.credit;
        return (
          <Pressable
            key={s.key}
            testID={`${testID ?? 'seg'}-${s.key}`}
            accessibilityRole="button"
            accessibilityState={{ selected: active }}
            onPress={() => onChange?.(s.key)}
            style={[
              styles.btn,
              i > 0 && styles.divider,
              active && { backgroundColor: withAlpha(tint, 0.14) },
            ]}
          >
            <Text
              style={[
                styles.label,
                { color: active ? tint : color.ink2 },
                active && styles.activeLabel,
              ]}
            >
              {s.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  seg: {
    flexDirection: 'row',
    alignSelf: 'flex-start',
    borderWidth: 1,
    borderColor: color.rule,
    borderRadius: radius.sm,
    overflow: 'hidden',
  },
  btn: { paddingVertical: 8, paddingHorizontal: 14 },
  divider: { borderLeftWidth: 1, borderLeftColor: color.rule },
  label: { fontFamily: font.mono, fontSize: 12, letterSpacing: 0.48 },
  activeLabel: { fontWeight: '600' },
});
