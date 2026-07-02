import React from 'react';
import { render } from '@testing-library/react-native';
import { StatementTable, type StatementSection } from './StatementTable';
import { color } from '../theme/tokens';

const PROFIT_AND_LOSS: StatementSection[] = [
  {
    label: 'Revenue · 収益',
    rows: [{ label: '4101 · Sales revenue', amount: '42000.00', type: 'credit' }],
  },
  {
    label: 'Expenses · 費用',
    rows: [{ label: '6210 · Software', amount: '3840.00', type: 'debit' }],
  },
];

const BALANCE_SHEET: StatementSection[] = [
  {
    label: 'Assets · 資産',
    rows: [
      { label: '1100 · Cash', amount: '84210.18' },
      { label: '1200 · Receivables', amount: '12640.00' },
    ],
    total: { label: 'Total assets', amount: '96850.18' },
  },
  {
    label: 'Liabilities · 負債',
    rows: [{ label: '2100 · Payables', amount: '8650.18' }],
  },
  {
    label: 'Equity · 純資産',
    rows: [{ label: '3100 · Retained', amount: '88200.00' }],
    total: { label: 'Liabilities + equity', amount: '96850.18' },
  },
];

describe('StatementTable (AT6 / IT6 — §07)', () => {
  it('AT6: a balanced balance sheet shows the check bar Difference 0.00 + ✓ Balanced', () => {
    const { getByTestId, getByText, getAllByText } = render(
      <StatementTable
        testID="bs"
        state="ready"
        title="Balance sheet · 貸借対照表"
        sections={BALANCE_SHEET}
        check={{ debits: ['96850.18'], credits: ['96850.18'] }}
      />,
    );
    expect(getByTestId('bs-check').props.accessibilityLabel).toBe('balanced');
    expect(getByText('✓ Balanced')).toBeTruthy();
    // Both totals (assets, liab+equity) render grouped at 96,850.18.
    expect(getAllByText('96,850.18').length).toBe(2);
  });

  it('AT6: an out-of-balance sheet shows the check bar as unbalanced', () => {
    const { getByTestId } = render(
      <StatementTable
        testID="bs"
        state="ready"
        title="Balance sheet"
        sections={BALANCE_SHEET}
        check={{ debits: ['96850.18'], credits: ['96849.00'] }}
      />,
    );
    expect(getByTestId('bs-check').props.accessibilityLabel).toBe('unbalanced');
  });

  it('IT6: the four states produce distinct output (no missing branch)', () => {
    const states = ['loading', 'empty', 'error', 'ready'] as const;
    const labels = states.map((s) => {
      const { getByTestId, unmount } = render(
        <StatementTable testID="bs" state={s} title="Balance sheet" sections={BALANCE_SHEET} />,
      );
      const label = getByTestId('bs').props.accessibilityLabel;
      unmount();
      return label;
    });
    expect(new Set(labels).size).toBe(4);
  });

  it('IT6: loading + error branches render their primitives', () => {
    const loading = render(<StatementTable testID="bs" state="loading" title="P&L" />);
    expect(loading.getByTestId('bs-loading')).toBeTruthy();
    loading.unmount();
    const err = render(<StatementTable testID="bs" state="error" title="P&L" errorCode="server_error" />);
    expect(err.getByTestId('bs-error')).toBeTruthy();
  });

  it('a P&L colors revenue rows credit and expense rows debit', () => {
    const { getByText } = render(
      <StatementTable testID="pl" state="ready" title="P&L · 損益計算書" sections={PROFIT_AND_LOSS} />,
    );
    const revenue = flatten(getByText('42,000.00').props.style);
    expect(revenue.color).toBe(color.credit);
    const expense = flatten(getByText('3,840.00').props.style);
    expect(expense.color).toBe(color.debit);
  });

  it('falls back to the default "stmt" testID when none is given', () => {
    const { getByTestId } = render(
      <StatementTable state="ready" title="Balance sheet" sections={BALANCE_SHEET} />,
    );
    expect(getByTestId('stmt')).toBeTruthy();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
