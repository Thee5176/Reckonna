// LedgerTable — General Ledger (all entries) and Subsidiary Ledger (grouped by
// account) share one row (design §06). Read-only; sourced from the query
// service in plan 05. A `state` discriminator drives four DISTINCT renders —
// loading / empty / error / ready — because every async surface must ship all
// three non-ready states (§04 sign-off rule, IT6). Presentational only.
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';
import { formatGrouped } from '../lib/money';
import { Badge, type BadgeStatus } from './Badge';
import { Skeleton } from './Skeleton';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { Alert, type ErrorCode } from './Alert';

export type AsyncState = 'loading' | 'empty' | 'error' | 'ready';

export interface LedgerRow {
  id: string;
  date: string; // already-formatted display date (design shows "May 24")
  description: string;
  account: string; // "4101 · Revenue"
  status: BadgeStatus;
  amount: string; // canonical 4dp value string
  type: 'debit' | 'credit'; // drives sign + color
  flagged?: boolean; // subtly washes the row (design §06)
}

export interface LedgerTableProps {
  state: AsyncState;
  rows?: LedgerRow[];
  variant?: 'general' | 'subsidiary';
  errorCode?: ErrorCode;
  onRetry?: () => void;
  onNewEntry?: () => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

function amountDisplay(type: 'debit' | 'credit', value: string): string {
  const grouped = formatGrouped(value);
  return type === 'credit' ? `+ ${grouped}` : `− ${grouped}`;
}

export function LedgerTable({
  state,
  rows = [],
  variant = 'general',
  errorCode = 'server_error',
  onRetry,
  onNewEntry,
  style,
  testID,
}: LedgerTableProps) {
  return (
    <View testID={testID} accessibilityLabel={`ledger-${state}`} style={[styles.ledger, style]}>
      {state === 'loading' ? (
        <View testID={`${testID ?? 'ledger'}-loading`} style={styles.pad}>
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} width={`${90 - i * 6}%`} style={styles.skelRow} />
          ))}
          <Spinner label="Fetching ledgers…" style={styles.spinner} />
        </View>
      ) : null}

      {state === 'empty' ? (
        <EmptyState
          testID={`${testID ?? 'ledger'}-empty`}
          actionLabel="+ New entry"
          onAction={onNewEntry}
        />
      ) : null}

      {state === 'error' ? (
        <View testID={`${testID ?? 'ledger'}-error`} style={styles.pad}>
          <Alert code={errorCode} onRetry={onRetry} />
        </View>
      ) : null}

      {state === 'ready' ? (
        <View testID={`${testID ?? 'ledger'}-ready`}>
          <View style={[styles.row, styles.head]}>
            <Text style={[styles.hcell, styles.cDate]}>Date</Text>
            <Text style={[styles.hcell, styles.cDesc]}>Description</Text>
            <Text style={[styles.hcell, styles.cAcct]}>Account</Text>
            <Text style={[styles.hcell, styles.cStatus]}>
              {variant === 'subsidiary' ? 'Ledger' : 'Status'}
            </Text>
            <Text style={[styles.hcell, styles.cAmt]}>Amount</Text>
          </View>
          {rows.map((r, i) => (
            <View
              key={r.id}
              testID={`${testID ?? 'ledger'}-row-${r.id}`}
              style={[styles.row, i > 0 && styles.rowBorder, r.flagged && styles.flagged]}
            >
              <Text style={[styles.cell, styles.cDate, styles.muted]}>{r.date}</Text>
              <Text style={[styles.cell, styles.cDesc]}>{r.description}</Text>
              <Text style={[styles.cell, styles.cAcct, styles.muted]}>{r.account}</Text>
              <View style={styles.cStatus}>
                <Badge label={r.status} status={r.status} />
              </View>
              <Text
                style={[
                  styles.cell,
                  styles.cAmt,
                  styles.num,
                  { color: r.type === 'credit' ? color.credit : color.debit },
                ]}
              >
                {amountDisplay(r.type, r.amount)}
              </Text>
            </View>
          ))}
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  ledger: {
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.md,
    backgroundColor: color.surface,
    overflow: 'hidden',
  },
  pad: { padding: 16 },
  skelRow: { marginBottom: 8 },
  spinner: { marginTop: 10 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 11, paddingHorizontal: 16 },
  head: { backgroundColor: color.bgElev },
  rowBorder: { borderTopWidth: 1, borderTopColor: color.hairline },
  flagged: { backgroundColor: withAlpha(color.debit, 0.04) },
  hcell: { fontFamily: font.mono, fontSize: 10.5, color: color.ink3, letterSpacing: 1, textTransform: 'uppercase' },
  cell: { fontFamily: font.mono, fontSize: 13, color: color.ink },
  muted: { color: color.ink3, fontSize: 11 },
  num: { textAlign: 'right', fontVariant: ['tabular-nums'] },
  cDate: { flex: 0.8 },
  cDesc: { flex: 2 },
  cAcct: { flex: 1.4 },
  cStatus: { flex: 0.9 },
  cAmt: { flex: 1.2, textAlign: 'right' },
});
