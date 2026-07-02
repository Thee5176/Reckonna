// AmountInput — right-aligned, tabular money entry (design §03). Keeps a 4dp
// value UNDER THE HOOD; the display rounds to 2dp (grouped) when blurred and
// reveals the full 4dp while focused ("4dp under the hood; display rounds to
// 2dp"). onChangeValue always emits the canonical 4dp string (IT7) — money
// never round-trips through a JS number. Non-positive amounts are invalid
// with the backend-mirrored message (AT3, plan 03 field validation).
import React, { useState } from 'react';
import type { StyleProp, ViewStyle } from 'react-native';
import { Field } from './Field';
import { toValue, formatGrouped } from '../lib/money';

export interface AmountInputProps {
  label?: string;
  value?: string; // canonical 4dp string
  onChangeValue?: (value: string) => void;
  invalid?: boolean;
  error?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

// A partial-but-parseable numeric draft (allows a lone '-' or '.' mid-typing).
function isNumericLike(text: string): boolean {
  return /^[+-]?\d*\.?\d*$/.test(text) && /\d/.test(text);
}

// Positive means strictly > 0 at 4dp. Empty is treated as "not yet invalid".
function isPositive(value: string): boolean {
  if (!isNumericLike(value)) return false;
  const v = toValue(value);
  return !v.startsWith('-') && v !== '0.0000';
}

// What the underlying <Field> shows: the 4dp draft while focused, the
// grouped 2dp display while blurred and numeric, or the raw value otherwise.
function resolveDisplayValue(
  focused: boolean,
  draft: string | null,
  canonical: string,
  value: string,
): string {
  if (focused) {
    // draft is always set in the same handler that sets focused=true (onFocus)
    // or is cleared alongside it (onBlur), so `draft` is never null while
    // `focused` is true — `?? canonical` is a defensive fallback only.
    /* istanbul ignore next */
    return draft ?? canonical;
  }
  if (value !== '' && isNumericLike(value)) return formatGrouped(value);
  return value;
}

export function AmountInput({
  label = 'Amount · JPY',
  value = '',
  onChangeValue,
  invalid,
  error,
  style,
  testID,
}: Readonly<AmountInputProps>) {
  const [focused, setFocused] = useState(false);
  const [draft, setDraft] = useState<string | null>(null);

  const nonPositive = value !== '' && !isPositive(value);
  const computedInvalid = invalid ?? nonPositive;
  const computedError = error ?? (nonPositive ? 'Amount must be positive.' : undefined);

  const canonical = value !== '' && isNumericLike(value) ? toValue(value) : value;
  const displayValue = resolveDisplayValue(focused, draft, canonical, value);

  return (
    <Field
      testID={testID}
      label={label}
      value={displayValue}
      rightAlign
      focused={focused}
      keyboardType="numeric"
      invalid={computedInvalid}
      error={computedError}
      style={style}
      onFocus={() => {
        setFocused(true);
        setDraft(canonical);
      }}
      onBlur={() => {
        setFocused(false);
        setDraft(null);
      }}
      onChangeText={(text) => {
        setDraft(text);
        if (isNumericLike(text)) onChangeValue?.(toValue(text));
      }}
    />
  );
}
