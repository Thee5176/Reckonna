// LedgerTable — General Ledger + Subsidiary Ledger share one row (design
// §06). Four DISTINCT states — loading / empty / error / ready — because
// every async surface must ship all three non-ready states (IT6).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { LedgerTable, type LedgerRow } from './LedgerTable';

const ROWS: LedgerRow[] = [
  {
    id: '1',
    date: 'May 24',
    description: 'Stripe payout · 14 invoices',
    account: '4101 · Revenue',
    status: 'posted',
    amount: '4200.00',
    type: 'credit',
  },
  {
    id: '2',
    date: 'May 24',
    description: 'Linear · annual seat',
    account: '6210 · Software',
    status: 'review',
    amount: '384.00',
    type: 'debit',
  },
  {
    id: '3',
    date: 'May 23',
    description: 'J. Cho · contractor pay',
    account: '6010 · Contractors',
    status: 'flagged',
    amount: '1763.40',
    type: 'debit',
    flagged: true,
  },
];

const meta: Meta<typeof LedgerTable> = {
  title: 'Components/LedgerTable',
  component: LedgerTable,
};
export default meta;

type Story = StoryObj<typeof LedgerTable>;

export const Loading: Story = { args: { state: 'loading' } };
export const Empty: Story = { args: { state: 'empty', onNewEntry: () => {} } };
export const Error: Story = {
  args: { state: 'error', errorCode: 'server_error', onRetry: () => {} },
};
export const ReadyGeneral: Story = { args: { state: 'ready', rows: ROWS, variant: 'general' } };
export const ReadySubsidiary: Story = {
  args: { state: 'ready', rows: ROWS, variant: 'subsidiary' },
};
