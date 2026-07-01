// Alert — the error branch every async surface must ship (design §04). Keyed
// by the plan 03 RFC 7807 error `code` (NOT localized text — locale-fragile,
// IT5). unbalanced_entry → the inline 借方≠貸方 balance error (plan 03 AT2);
// server_error → a Retry alert (5xx); unauthorized → re-auth prompt (401 →
// Keycloak, wired in plan 05). Presentational: code/message in, onRetry out.
import React from 'react';
import { View, Text, Pressable, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';

export type ErrorCode =
  | 'unbalanced_entry'
  | 'validation_failed'
  | 'unauthorized'
  | 'server_error';

export interface AlertProps {
  code?: ErrorCode;
  message?: string;
  onRetry?: () => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

interface CodeSpec {
  icon: string;
  defaultMessage: string;
  retryable: boolean;
}

// code → presentation. Assertions target the code, never these strings (IT5).
const SPEC: Record<ErrorCode, CodeSpec> = {
  unbalanced_entry: {
    icon: '!',
    defaultMessage: '借方 ≠ 貸方 — the entry does not balance. Difference must be zero.',
    retryable: false,
  },
  validation_failed: {
    icon: '!',
    defaultMessage: 'Some fields need attention before this can be saved.',
    retryable: false,
  },
  unauthorized: {
    icon: '⤪',
    defaultMessage: 'Your session has expired. Sign in again to continue.',
    retryable: false,
  },
  server_error: {
    icon: '⤫',
    defaultMessage: "Couldn't reach the ledger service. Please try again.",
    retryable: true,
  },
};

export function Alert({ code = 'server_error', message, onRetry, style, testID }: AlertProps) {
  const spec = SPEC[code];
  const showRetry = spec.retryable && !!onRetry;
  return (
    <View
      testID={testID ?? `alert-${code}`}
      accessibilityRole="alert"
      accessibilityLabel={code}
      style={[styles.alert, styles.error, style]}
    >
      <Text style={styles.ico}>{spec.icon}</Text>
      <View style={styles.body}>
        <Text style={styles.text}>{message ?? spec.defaultMessage}</Text>
        {showRetry ? (
          <Pressable
            testID={`${testID ?? `alert-${code}`}-retry`}
            accessibilityRole="button"
            onPress={onRetry}
          >
            <Text style={styles.retry}>Retry</Text>
          </Pressable>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  alert: {
    flexDirection: 'row',
    gap: 12,
    paddingVertical: 14,
    paddingHorizontal: 16,
    borderRadius: radius.md,
    borderWidth: 1,
  },
  error: {
    backgroundColor: withAlpha(color.debit, 0.08),
    borderColor: withAlpha(color.debit, 0.35),
  },
  ico: { fontFamily: font.mono, fontSize: 13, fontWeight: '700', color: color.debit },
  body: { flex: 1, flexDirection: 'row', justifyContent: 'space-between', gap: 12 },
  text: { flex: 1, fontFamily: font.mono, fontSize: 12.5, lineHeight: 18, color: color.debit },
  retry: { fontFamily: font.mono, fontSize: 12.5, color: color.debit, textDecorationLine: 'underline' },
});
