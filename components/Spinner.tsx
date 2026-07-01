// Spinner — indeterminate progress (design §04). Pairs with Skeleton in the
// loading state ("Fetching ledgers…"). Presentational.
import React from 'react';
import { View, Text, ActivityIndicator, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font } from '../theme/tokens';

export interface SpinnerProps {
  label?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Spinner({ label, style, testID }: SpinnerProps) {
  return (
    <View testID={testID} accessibilityLabel="loading" style={[styles.row, style]}>
      <ActivityIndicator color={color.ink} size="small" />
      {label ? <Text style={styles.label}>{label}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 10 },
  label: { fontFamily: font.mono, fontSize: 11.5, color: color.ink3 },
});
