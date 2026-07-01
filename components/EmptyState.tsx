// EmptyState — the empty branch every async surface must ship (design §04).
// The CoA-grid mark is the design's inline <svg>, ported to react-native-svg.
// Presentational: copy + an optional accent action slot ("+ New entry").
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import Svg, { Rect, Line } from 'react-native-svg';
import { color, font } from '../theme/tokens';
import { Button } from './Button';

export interface EmptyStateProps {
  title?: string;
  message?: string;
  actionLabel?: string;
  onAction?: () => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

// The chart-of-accounts grid glyph (design §04 empty mark).
function CoaGridMark() {
  return (
    <Svg width={48} height={48} viewBox="0 0 44 44" fill="none" opacity={0.5}>
      <Rect x={1} y={1} width={42} height={42} rx={3} stroke={color.ink3} strokeWidth={1.2} />
      <Line x1={22} y1={6} x2={22} y2={38} stroke={color.ink3} strokeWidth={1.2} />
      <Line x1={6} y1={14} x2={38} y2={14} stroke={color.ink3} strokeWidth={1.2} />
    </Svg>
  );
}

export function EmptyState({
  title = 'No entries yet.',
  message = 'Your first balanced entry will appear here.',
  actionLabel,
  onAction,
  style,
  testID,
}: EmptyStateProps) {
  return (
    <View testID={testID} accessibilityLabel="empty" style={[styles.empty, style]}>
      <View style={styles.mark}>
        <CoaGridMark />
      </View>
      <Text style={styles.title}>{title}</Text>
      {message ? <Text style={styles.message}>{message}</Text> : null}
      {actionLabel ? (
        <Button
          testID={`${testID ?? 'empty'}-action`}
          label={actionLabel}
          variant="accent"
          onPress={onAction}
          style={styles.action}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  empty: { alignItems: 'center', paddingVertical: 40, paddingHorizontal: 24 },
  mark: { marginBottom: 14 },
  title: { fontFamily: font.serif, fontSize: 17, color: color.ink, marginBottom: 6 },
  message: {
    fontFamily: font.mono,
    fontSize: 11.5,
    color: color.ink3,
    textAlign: 'center',
    marginBottom: 14,
  },
  action: { alignSelf: 'center' },
});
