// Field — the label + input + error wrapper (design §03). Field errors mirror
// backend validation (plan 03). When `invalid`, the input border turns debit
// and the error line renders. Presentational: value in, onChangeText out.
import React from 'react';
import { View, Text, TextInput, StyleSheet } from 'react-native';
import type { KeyboardTypeOptions, StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';

export interface FieldProps {
  label: string;
  value?: string;
  onChangeText?: (text: string) => void;
  placeholder?: string;
  error?: string;
  invalid?: boolean;
  rightAlign?: boolean;
  keyboardType?: KeyboardTypeOptions;
  onFocus?: () => void;
  onBlur?: () => void;
  focused?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Field({
  label,
  value,
  onChangeText,
  placeholder,
  error,
  invalid = false,
  rightAlign = false,
  keyboardType,
  onFocus,
  onBlur,
  focused = false,
  style,
  testID,
}: FieldProps) {
  const showError = invalid && !!error;
  return (
    <View style={[styles.field, style]}>
      <Text style={styles.lab}>{label}</Text>
      <TextInput
        testID={testID}
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={color.ink3}
        keyboardType={keyboardType}
        onFocus={onFocus}
        onBlur={onBlur}
        accessibilityState={{ disabled: false }}
        aria-invalid={invalid}
        style={[
          styles.input,
          rightAlign && styles.right,
          focused && styles.focused,
          invalid && styles.invalid,
        ]}
      />
      {showError ? <Text style={styles.err}>{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  field: { flexDirection: 'column', gap: 5 },
  lab: { fontSize: 11, color: color.ink3, letterSpacing: 0.4, fontFamily: font.mono },
  input: {
    fontFamily: font.mono,
    fontSize: 13.5,
    color: color.ink,
    backgroundColor: color.surface,
    borderWidth: 1,
    borderColor: color.rule,
    borderRadius: radius.sm - 1,
    paddingVertical: 10,
    paddingHorizontal: 12,
  },
  right: { textAlign: 'right' },
  focused: {
    borderColor: color.accent,
    // focus ring — highlight at ~45% (design §03)
    boxShadow: `0 0 0 3px ${withAlpha(color.highlight, 0.45)}`,
  } as ViewStyle,
  invalid: { borderColor: color.debit },
  err: { fontSize: 10.5, color: color.debit, fontFamily: font.mono },
});
