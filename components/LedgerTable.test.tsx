import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { LedgerTable, type LedgerRow } from './LedgerTable';

const ROWS: LedgerRow[] = [
  { id: '1', date: 'May 24', description: 'Stripe payout · 14 invoices', account: '4101 · Revenue', status: 'posted', amount: '4200.00', type: 'credit' },
  { id: '2', date: 'May 24', description: 'Linear · annual seat', account: '6210 · Software', status: 'review', amount: '384.00', type: 'debit' },
  { id: '3', date: 'May 23', description: 'J. Cho · contractor pay', account: '6010 · Contractors', status: 'flagged', amount: '1763.40', type: 'debit', flagged: true },
];

describe('LedgerTable (AT4 / IT6 — four states §06)', () => {
  it('AT4/IT6: loading renders Skeleton + Spinner', () => {
    const { getByTestId } = render(<LedgerTable testID="lt" state="loading" />);
    expect(getByTestId('lt').props.accessibilityLabel).toBe('ledger-loading');
    expect(getByTestId('lt-loading')).toBeTruthy();
  });

  it('AT4/IT6: empty renders the EmptyState with a + New entry action', () => {
    const onNewEntry = jest.fn();
    const { getByTestId, getByText } = render(
      <LedgerTable testID="lt" state="empty" rows={[]} onNewEntry={onNewEntry} />,
    );
    expect(getByTestId('lt').props.accessibilityLabel).toBe('ledger-empty');
    expect(getByText('No entries yet.')).toBeTruthy();
    fireEvent.press(getByTestId('lt-empty-action'));
    expect(onNewEntry).toHaveBeenCalledTimes(1);
  });

  it('IT6: error renders an Alert keyed by code with Retry', () => {
    const onRetry = jest.fn();
    const { getByTestId } = render(
      <LedgerTable testID="lt" state="error" errorCode="server_error" onRetry={onRetry} />,
    );
    expect(getByTestId('lt').props.accessibilityLabel).toBe('ledger-error');
    fireEvent.press(getByTestId('alert-server_error-retry'));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('AT4/IT6: ready renders the rows with signed, color-coded amounts', () => {
    const { getByTestId, getByText } = render(
      <LedgerTable testID="lt" state="ready" rows={ROWS} />,
    );
    expect(getByTestId('lt').props.accessibilityLabel).toBe('ledger-ready');
    expect(getByTestId('lt-row-1')).toBeTruthy();
    expect(getByText('+ 4,200.00')).toBeTruthy(); // credit
    expect(getByText('− 384.00')).toBeTruthy(); // debit
  });

  it('IT6: the four states produce distinct output (no missing branch)', () => {
    const states = ['loading', 'empty', 'error', 'ready'] as const;
    const labels = states.map((s) => {
      const { getByTestId, unmount } = render(<LedgerTable testID="lt" state={s} rows={ROWS} />);
      const label = getByTestId('lt').props.accessibilityLabel;
      unmount();
      return label;
    });
    expect(new Set(labels).size).toBe(4);
  });

  it('the subsidiary variant labels its status column "Ledger" instead of "Status"', () => {
    const { getByText } = render(
      <LedgerTable testID="lt" state="ready" rows={ROWS} variant="subsidiary" />,
    );
    expect(getByText('Ledger')).toBeTruthy();
  });

  it('falls back to the default "ledger" testID prefix when none is given', () => {
    const { getByTestId } = render(<LedgerTable state="ready" rows={ROWS} />);
    expect(getByTestId('ledger-row-1')).toBeTruthy();
  });

  it('falls back to the default "ledger" testID prefix in the loading/empty/error states too', () => {
    const loading = render(<LedgerTable state="loading" />);
    expect(loading.getByTestId('ledger-loading')).toBeTruthy();
    loading.unmount();

    const empty = render(<LedgerTable state="empty" />);
    expect(empty.getByTestId('ledger-empty')).toBeTruthy();
    empty.unmount();

    const error = render(<LedgerTable state="error" />);
    expect(error.getByTestId('ledger-error')).toBeTruthy();
  });
});
