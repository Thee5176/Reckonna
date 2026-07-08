// JournalEntryForm — the two-step journal capture (design §05). Step 1 reads
// the entry "as a sentence" with a live BalanceBar; the CTA is dead until
// 借方=貸方 (AT1/AT2). Story-level `key` remounts so each variant starts at its
// own initial state (step is internal component state).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { JournalEntryForm, type JournalLineInput } from './JournalEntryForm';
import type { Account } from './AccountSelect';

const COA: Account[] = [
  { code: '1100', name: 'Cash', element: 'Assets' },
  { code: '4101', name: 'Sales revenue', element: 'Revenue' },
];

const BALANCED_LINES: JournalLineInput[] = [
  { account: '1100', amount: '1000.00', side: 'debit' },
  { account: '4101', amount: '1000.00', side: 'credit' },
];

const UNBALANCED_LINES: JournalLineInput[] = [
  { account: '1100', amount: '1000.00', side: 'debit' },
  { account: '4101', amount: '500.00', side: 'credit' },
];

const meta: Meta<typeof JournalEntryForm> = {
  title: 'Components/JournalEntryForm',
  component: JournalEntryForm,
  args: {
    accounts: COA,
    initialDate: '2026-05-24',
    initialDescription: 'Stripe payout · 14 invoices',
  },
};
export default meta;

type Story = StoryObj<typeof JournalEntryForm>;

export const BalancedEntry: Story = { args: { initialLines: BALANCED_LINES } };
export const UnbalancedEntry: Story = { args: { initialLines: UNBALANCED_LINES } };
export const WithServerError: Story = {
  args: {
    initialLines: BALANCED_LINES,
    errorCode: 'server_error',
    onRetry: () => {},
  },
};
