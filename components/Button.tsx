// Button — the design §02 controls. Five variants + a disabled state. The
// primary (ink) is the one-per-view action; accent (copper) is the step-forward
// CTA; disabled primary = an unbalanced entry that cannot post (design §02,
// AT2). Presentational: props in, onPress out. No data/network here.
import React from 'react';
import { Pressable, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'accent' | 'danger';

export interface ButtonProps {
  label: string;
  onPress?: () => void;
  variant?: ButtonVariant;
  disabled?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Button({
  label,
  onPress,
  variant = 'primary',
  disabled = false,
  style,
  testID,
}: ButtonProps) {
  const v = variantStyles[variant];
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      disabled={disabled}
      onPress={onPress}
      style={[styles.base, v.container, disabled && styles.disabled, style]}
    >
      <Text style={[styles.label, { color: v.text }]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    paddingVertical: 9,
    paddingHorizontal: 16,
    borderRadius: radius.sm,
    borderWidth: 1,
  },
  label: {
    fontFamily: font.mono,
    fontSize: 12.5,
    fontWeight: '500',
    letterSpacing: 0.25,
  },
  disabled: {
    opacity: 0.4,
  },
});

const variantStyles: Record<ButtonVariant, { container: ViewStyle; text: string }> = {
  primary: {
    container: { backgroundColor: color.ink, borderColor: color.ink },
    text: color.bg,
  },
  secondary: {
    container: { backgroundColor: 'transparent', borderColor: color.rule },
    text: color.ink,
  },
  ghost: {
    container: { backgroundColor: 'transparent', borderColor: color.hairline },
    text: color.ink,
  },
  accent: {
    container: { backgroundColor: color.accent, borderColor: color.accent },
    text: color.bg,
  },
  danger: {
    container: { backgroundColor: 'transparent', borderColor: withAlpha(color.debit, 0.4) },
    text: color.debit,
  },
};
