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

  it('editing a line (side, account, amount) recomputes the live BalanceBar, leaving other lines untouched', () => {
    const { getByTestId } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={[{ ...unbalanced[0] }, { ...unbalanced[1] }]} />,
    );
    // Flip line 0 from debit to credit; with line 1 (500 credit) untouched
    // this becomes 2000 total credit vs 0 debit — still unbalanced, proving
    // both the toggle AND the "leave other lines alone" map branch ran.
    fireEvent.press(getByTestId('jef-line-0-side-credit'));
    expect(getByTestId('jef-balance').props.accessibilityLabel).toBe('unbalanced');

    fireEvent.press(getByTestId('jef-line-0-account'));
    fireEvent.press(getByTestId('jef-line-0-account-opt-4101'));

    const amount = getByTestId('jef-line-0-amount');
    fireEvent(amount, 'focus');
    fireEvent.changeText(amount, '250');
    fireEvent(amount, 'blur');
    // Line 0 now credits 250; line 1 is still its original 500 credit → 750 vs 0.
    expect(getByTestId('jef-balance-diff').props.children).toBe('750.00');
  });

  it('"＋ Add line…" appends a fresh empty line', () => {
    const { getByTestId, queryByTestId } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={balanced} />,
    );
    expect(queryByTestId('jef-line-2')).toBeNull();
    fireEvent.press(getByTestId('jef-add-line'));
    expect(getByTestId('jef-line-2')).toBeTruthy();
  });

  it('Save draft fires onSaveDraft with the built payload from step 2', () => {
    const onSaveDraft = jest.fn();
    const { getByTestId } = render(
      <JournalEntryForm
        testID="jef"
        accounts={COA}
        initialDate="2026-05-24"
        initialDescription="Stripe payout"
        initialLines={balanced}
        onSaveDraft={onSaveDraft}
      />,
    );
    fireEvent.press(getByTestId('jef-balance-cta')); // → step 2
    fireEvent.press(getByTestId('jef-save-draft'));
    expect(onSaveDraft).toHaveBeenCalledWith({
      date: '2026-05-24',
      description: 'Stripe payout',
      book: 'base',
      lines: [
        { account: '1100', amount: '1000.0000', side: 'debit' },
        { account: '4101', amount: '1000.0000', side: 'credit' },
      ],
    });
  });

  it('"← Back to entry" and the "1 · Entry" step tab both return to step 1', () => {
    const { getByTestId, getByText, queryByTestId } = render(
      <JournalEntryForm testID="jef" accounts={COA} initialLines={balanced} />,
    );
    fireEvent.press(getByTestId('jef-balance-cta')); // → step 2
    expect(queryByTestId('jef-post')).toBeTruthy();

    fireEvent.press(getByText('← Back to entry'));
    expect(getByTestId('jef-balance')).toBeTruthy(); // back on step 1

    fireEvent.press(getByTestId('jef-balance-cta')); // → step 2 again
    fireEvent.press(getByText('1 · Entry'));
    expect(getByTestId('jef-balance')).toBeTruthy(); // back on step 1 via the tab
  });

  it('with no initialLines and no testID, starts from a single empty line under the default "jef" id', () => {
    const { getByTestId } = render(<JournalEntryForm accounts={COA} />);
    expect(getByTestId('jef-line-0')).toBeTruthy();
    expect(getByTestId('jef-balance').props.accessibilityLabel).toBe('balanced'); // 0 debits === 0 credits
  });

  it('review step shows "—" for a line whose account was left blank', () => {
    const { getByTestId, getByText } = render(
      <JournalEntryForm
        testID="jef"
        accounts={COA}
        initialLines={[
          { account: '', amount: '1000.00', side: 'debit' }, // blank account, still balances
          { account: '1100', amount: '1000.00', side: 'credit' },
        ]}
      />,
    );
    fireEvent.press(getByTestId('jef-balance-cta')); // balanced → advances to step 2
    expect(getByTestId('jef-review-0')).toBeTruthy();
    expect(getByText('—')).toBeTruthy();
  });

  it('renders an inline Alert when errorCode is set, with Retry wired to onRetry', () => {
    const onRetry = jest.fn();
    const { getByTestId } = render(
      <JournalEntryForm
        testID="jef"
        accounts={COA}
        initialLines={balanced}
        errorCode="server_error"
        onRetry={onRetry}
      />,
    );
    fireEvent.press(getByTestId('alert-server_error-retry'));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
