// StatementTable — Balance Sheet (貸借対照表) and P&L (損益計算書), aggregated by
// CoA element (design §07). The balance check is SHOWN, not hidden (AT6).
// Four DISTINCT states like every async surface (IT6).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { StatementTable, type StatementSection } from './StatementTable';

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

const PROFIT_AND_LOSS: StatementSection[] = [
  {
    label: 'Revenue · 収益',
    rows: [{ label: '4101 · Sales revenue', amount: '42000.00', type: 'credit' }],
    total: { label: 'Total revenue', amount: '42000.00' },
  },
  {
    label: 'Expenses · 費用',
    rows: [
      { label: '6210 · Software', amount: '3840.00', type: 'debit' },
      { label: '6010 · Contractors', amount: '17634.00', type: 'debit' },
    ],
    total: { label: 'Total expenses', amount: '21474.00' },
  },
];

const meta: Meta<typeof StatementTable> = {
  title: 'Components/StatementTable',
  component: StatementTable,
};
export default meta;

type Story = StoryObj<typeof StatementTable>;

export const BalanceSheetReady: Story = {
  args: {
    state: 'ready',
    title: 'Balance sheet · 貸借対照表',
    sections: BALANCE_SHEET,
    check: { debits: ['96850.18'], credits: ['96850.18'] },
  },
};
export const ProfitAndLossReady: Story = {
  args: {
    state: 'ready',
    title: 'P&L · 損益計算書',
    sections: PROFIT_AND_LOSS,
  },
};
export const Loading: Story = { args: { state: 'loading', title: 'Balance sheet' } };
export const Empty: Story = { args: { state: 'empty', title: 'Balance sheet' } };
export const ErrorState: Story = {
  args: { state: 'error', title: 'Balance sheet', errorCode: 'server_error', onRetry: () => {} },
};
