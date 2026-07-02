// Skeleton — loading placeholder bar (design §04). One of the three states
// every async surface must ship. Presentational: width/height in, no data.
import React from 'react';
import { View, StyleSheet } from 'react-native';
import type { DimensionValue, StyleProp, ViewStyle } from 'react-native';
import { color, radius } from '../theme/tokens';

export interface SkeletonProps {
  width?: DimensionValue;
  height?: number;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Skeleton({ width = '100%', height = 14, style, testID }: Readonly<SkeletonProps>) {
  return (
    <View
      testID={testID}
      accessibilityLabel="loading"
      style={[styles.skel, { width, height }, style]}
    />
  );
}

const styles = StyleSheet.create({
  skel: {
    backgroundColor: color.bgElev,
    borderRadius: radius.sm - 1,
  },
});
