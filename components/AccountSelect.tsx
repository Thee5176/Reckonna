// AccountSelect — the chart-of-accounts picker (design §03, "Account · CoA").
// RN has no native <select>, so this is a controlled disclosure: the field row
// shows the current "code · name" (or a placeholder) and toggles an option
// list. Presentational — the CoA list arrives as a prop (plan 03 seed); value
// in, onChange(code) out. Invalid state mirrors backend validation.
import React, { useState } from 'react';
import { View, Pressable, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';

export interface Account {
  code: string;
  name: string;
  element?: string; // Assets / Revenue / … (design shows this muted)
}

export interface AccountSelectProps {
  label: string;
  accounts: Account[];
  value?: string; // selected code
  onChange?: (code: string) => void;
  placeholder?: string;
  invalid?: boolean;
  error?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function AccountSelect({
  label,
  accounts,
  value,
  onChange,
  placeholder = '— select an account —',
  invalid = false,
  error,
  style,
  testID,
}: Readonly<AccountSelectProps>) {
  const [open, setOpen] = useState(false);
  const selected = accounts.find((a) => a.code === value);
  const display = selected ? `${selected.code} · ${selected.name}` : placeholder;

  return (
    <View style={[styles.field, style]}>
      <Text style={styles.lab}>{label}</Text>
      <Pressable
        testID={testID}
        accessibilityRole="button"
        accessibilityState={{ expanded: open }}
        onPress={() => setOpen((o) => !o)}
        style={[styles.control, invalid && styles.invalid]}
      >
        <Text style={[styles.value, !selected && styles.placeholder]}>{display}</Text>
        <Text style={styles.chevron}>{open ? '▲' : '▼'}</Text>
      </Pressable>
      {open ? (
        <View style={styles.menu}>
          {accounts.map((a) => (
            <Pressable
              key={a.code}
              testID={`${testID ?? 'account'}-opt-${a.code}`}
              accessibilityRole="button"
              onPress={() => {
                onChange?.(a.code);
                setOpen(false);
              }}
              style={styles.option}
            >
              <Text style={styles.optText}>
                {a.code} · {a.name}
              </Text>
              {a.element ? <Text style={styles.optElement}>{a.element}</Text> : null}
            </Pressable>
          ))}
        </View>
      ) : null}
      {invalid && error ? <Text style={styles.err}>{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  field: { flexDirection: 'column', gap: 5 },
  lab: { fontSize: 11, color: color.ink3, letterSpacing: 0.4, fontFamily: font.mono },
  control: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: color.surface,
    borderWidth: 1,
    borderColor: color.rule,
    borderRadius: radius.sm - 1,
    paddingVertical: 10,
    paddingHorizontal: 12,
  },
  invalid: { borderColor: color.debit },
  value: { fontFamily: font.mono, fontSize: 13.5, color: color.ink },
  placeholder: { color: color.ink3 },
  chevron: { fontFamily: font.mono, fontSize: 10, color: color.ink3 },
  menu: {
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.sm - 1,
    backgroundColor: color.surface,
    overflow: 'hidden',
  },
  option: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 10,
    paddingHorizontal: 12,
    borderBottomWidth: 1,
    borderBottomColor: color.hairline,
  },
  optText: { fontFamily: font.mono, fontSize: 13, color: color.ink },
  optElement: { fontFamily: font.mono, fontSize: 11, color: color.ink3 },
  err: { fontSize: 10.5, color: color.debit, fontFamily: font.mono },
});
