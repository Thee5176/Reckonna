// StatementTable — Balance Sheet (貸借対照表) and Profit & Loss (損益計算書),
// aggregated by CoA element (design §07). The balance check is SHOWN, not
// hidden — assets must equal liabilities + equity, rendered as a BalanceBar
// 'check' (AT6). Like every async surface it renders four DISTINCT states —
// loading / empty / error / ready (IT6). Presentational: rows arrive as props
// from the query service (plan 05).
import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { formatGrouped } from '../lib/money';
import { Skeleton } from './Skeleton';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';
import { Alert, type ErrorCode } from './Alert';
import { BalanceBar } from './BalanceBar';
import type { AsyncState } from './LedgerTable';

export interface StatementRow {
  label: string; // "1100 · Cash"
  amount: string; // decimal string
  type?: 'debit' | 'credit'; // optional color (P&L revenue/expense)
}

export interface StatementSection {
  label: string; // "Assets · 資産"
  rows: StatementRow[];
  total?: { label: string; amount: string };
}

export interface StatementTableProps {
  state: AsyncState;
  title: string; // "Balance sheet · 貸借対照表"
  sections?: StatementSection[];
  // Balance check (balance sheet): assets vs liabilities+equity → BalanceBar.
  check?: { label?: string; debits: string[]; credits: string[] };
  errorCode?: ErrorCode;
  onRetry?: () => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function StatementTable({
  state,
  title,
  sections = [],
  check,
  errorCode = 'server_error',
  onRetry,
  style,
  testID,
}: Readonly<StatementTableProps>) {
  const id = testID ?? 'stmt';
  return (
    <View testID={id} accessibilityLabel={`statement-${state}`} style={[styles.card, style]}>
      <View style={styles.header}>
        <Text style={styles.eyebrow}>{title}</Text>
      </View>

      {state === 'loading' ? (
        <View testID={`${id}-loading`} style={styles.pad}>
          {[0, 1, 2, 3, 4].map((i) => (
            <Skeleton key={i} width={`${85 - i * 5}%`} style={styles.skelRow} />
          ))}
          <Spinner label="Fetching statement…" style={styles.spinner} />
        </View>
      ) : null}

      {state === 'empty' ? (
        <EmptyState
          testID={`${id}-empty`}
          title="Nothing to report yet."
          message="Post a balanced entry and it will roll up here."
        />
      ) : null}

      {state === 'error' ? (
        <View testID={`${id}-error`} style={styles.pad}>
          <Alert code={errorCode} onRetry={onRetry} />
        </View>
      ) : null}

      {state === 'ready' ? (
        <View testID={`${id}-ready`}>
          {sections.map((section, si) => (
            <View key={section.label} testID={`${id}-section-${si}`}>
              <View style={[styles.row, styles.group]}>
                <Text style={styles.groupText}>{section.label}</Text>
              </View>
              {section.rows.map((r) => (
                <View key={`${section.label}-${r.label}`} style={styles.row}>
                  <Text style={styles.rowLabel}>{r.label}</Text>
                  <Text
                    style={[
                      styles.amt,
                      r.type ? { color: r.type === 'credit' ? color.credit : color.debit } : null,
                    ]}
                  >
                    {formatGrouped(r.amount)}
                  </Text>
                </View>
              ))}
              {section.total ? (
                <View style={[styles.row, styles.total]}>
                  <Text style={styles.totalLabel}>{section.total.label}</Text>
                  <Text style={styles.totalAmt}>{formatGrouped(section.total.amount)}</Text>
                </View>
              ) : null}
            </View>
          ))}

          {check ? (
            <BalanceBar
              testID={`${id}-check`}
              variant="check"
              checkLabel={check.label ?? 'Check · assets − (liab + equity)'}
              debits={check.debits}
              credits={check.credits}
              style={styles.check}
            />
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.md,
    backgroundColor: color.surface,
    overflow: 'hidden',
  },
  header: { padding: 14, borderBottomWidth: 1, borderBottomColor: color.hairline },
  eyebrow: { fontFamily: font.mono, fontSize: 10.5, letterSpacing: 1.8, textTransform: 'uppercase', color: color.ink3 },
  pad: { padding: 16 },
  skelRow: { marginBottom: 8 },
  spinner: { marginTop: 10 },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 8,
    paddingHorizontal: 10,
    borderBottomWidth: 1,
    borderBottomColor: color.hairline,
  },
  group: { backgroundColor: color.bgElev },
  groupText: { fontFamily: font.mono, fontSize: 10.5, letterSpacing: 1, textTransform: 'uppercase', color: color.ink3 },
  rowLabel: { fontFamily: font.mono, fontSize: 12.5, color: color.ink },
  amt: { fontFamily: font.mono, fontSize: 12.5, color: color.ink, textAlign: 'right', fontVariant: ['tabular-nums'] },
  total: { borderTopWidth: 1, borderTopColor: color.rule },
  totalLabel: { fontFamily: font.mono, fontSize: 12.5, fontWeight: '600', color: color.ink },
  totalAmt: { fontFamily: font.mono, fontSize: 12.5, fontWeight: '600', color: color.ink, textAlign: 'right', fontVariant: ['tabular-nums'] },
  check: { margin: 12 },
});
