// AccountSelect — the chart-of-accounts picker (design §03). RN has no native
// <select>; the field row shows the current selection and toggles a list.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { AccountSelect, type Account } from './AccountSelect';

const COA: Account[] = [
  { code: '4101', name: 'Sales revenue', element: 'Revenue' },
  { code: '6210', name: 'Software', element: 'Expenses' },
  { code: '1100', name: 'Cash', element: 'Assets' },
];

const meta: Meta<typeof AccountSelect> = {
  title: 'Components/AccountSelect',
  component: AccountSelect,
  args: {
    label: 'Account · CoA',
    accounts: COA,
  },
};
export default meta;

type Story = StoryObj<typeof AccountSelect>;

export const Placeholder: Story = {};
export const Selected: Story = { args: { value: '1100' } };
export const Invalid: Story = {
  args: { invalid: true, error: 'Choose a chart-of-account code.' },
};
