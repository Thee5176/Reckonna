import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { JournalEntryForm, type JournalLineInput } from './JournalEntryForm';
import type { Account } from './AccountSelect';

const COA: Account[] = [
  { code: '1100', name: 'Cash', element: 'Assets' },
  { code: '4101', name: 'Sales revenue', element: 'Revenue' },
];

const balanced: JournalLineInput[] = [
  { account: '1100', amount: '1000.00', side: 'debit' },
  { account: '4101', amount: '1000.00', side: 'credit' },
];

const unbalanced: JournalLineInput[] = [
  { account: '1100', amount: '1000.00', side: 'debit' },
  { account: '4101', amount: '500.00', side: 'credit' },
];

describe('JournalEntryForm (AT1 / AT2 / IT4 — §05)', () => {
  it('AT1: balanced (1000/1000) → BalanceBar ok, ✓ Balanced, CTA ENABLED', () => {
    const { getByTestId, getByText } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={balanced} />,
    );
    expect(getByTestId('jef-balance').props.accessibilityLabel).toBe('balanced');
    expect(getByText('✓ Balanced')).toBeTruthy();
    expect(getByTestId('jef-balance-cta').props.accessibilityState.disabled).toBe(false);
  });

  it('AT2: unbalanced (1000/500) → BalanceBar bad, ✕ 借方≠貸方, Difference 500.00, CTA DISABLED', () => {
    const { getByTestId, getByText } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={unbalanced} />,
    );
    expect(getByTestId('jef-balance').props.accessibilityLabel).toBe('unbalanced');
    expect(getByText('✕ 借方≠貸方')).toBeTruthy();
    expect(getByTestId('jef-balance-diff').props.children).toBe('500.00');
    expect(getByTestId('jef-balance-cta').props.accessibilityState.disabled).toBe(true);
  });

  it('IT4: onSubmit payload matches the plan-03 POST /command/journal-entries shape', () => {
    const onSubmit = jest.fn();
    const { getByTestId } = render(
      <JournalEntryForm
        testID="jef"
        accounts={COA}
        initialDate="2026-05-24"
        initialDescription="Stripe payout · 14 invoices"
        initialLines={balanced}
        onSubmit={onSubmit}
      />,
    );
    // Step 1 CTA (enabled because balanced) → review; then Post.
    fireEvent.press(getByTestId('jef-balance-cta'));
    fireEvent.press(getByTestId('jef-post'));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith({
      date: '2026-05-24',
      description: 'Stripe payout · 14 invoices',
      book: 'base',
      lines: [
        { account: '1100', amount: '1000.0000', side: 'debit' },
        { account: '4101', amount: '1000.0000', side: 'credit' },
      ],
    });
  });

  it('IT4: an unbalanced entry cannot advance to post (CTA disabled → no onSubmit)', () => {
    const onSubmit = jest.fn();
    const { getByTestId, queryByTestId } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={unbalanced} onSubmit={onSubmit} />,
    );
    fireEvent.press(getByTestId('jef-balance-cta')); // disabled → no-op
    expect(queryByTestId('jef-post')).toBeNull();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
