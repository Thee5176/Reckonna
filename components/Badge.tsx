// Badge — entry-status chip (design §02). Maps to the ledger lifecycle:
// draft → review → posted (persisted & balanced) → flagged (failed a later
// reconciliation). Also used inline as the 借方/貸方 line marker. Presentational.
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';

export type BadgeStatus = 'draft' | 'review' | 'posted' | 'flagged';

export interface BadgeProps {
  label: string;
  status?: BadgeStatus;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Badge({ label, status = 'draft', style, testID }: Readonly<BadgeProps>) {
  const v = statusStyles[status];
  return (
    <View testID={testID} style={[styles.base, v.container, style]}>
      <Text style={[styles.label, { color: v.text }]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  base: {
    alignSelf: 'flex-start',
    paddingVertical: 2,
    paddingHorizontal: 7,
    borderRadius: radius.sm - 1,
    borderWidth: 1,
  },
  label: {
    fontFamily: font.mono,
    fontSize: 10,
    fontWeight: '500',
    letterSpacing: 1.2,
    textTransform: 'uppercase',
  },
});

const statusStyles: Record<BadgeStatus, { container: ViewStyle; text: string }> = {
  draft: {
    container: { borderColor: withAlpha(color.ink, 0.12) },
    text: color.ink3,
  },
  review: {
    container: { borderColor: withAlpha(color.accent, 0.3) },
    text: color.accent,
  },
  posted: {
    container: { borderColor: withAlpha(color.credit, 0.3) },
    text: color.credit,
  },
  flagged: {
    container: { borderColor: withAlpha(color.debit, 0.3) },
    text: color.debit,
  },
};
