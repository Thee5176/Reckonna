// DebitCreditSegment — the 借方/貸方 toggle (design §03, AT8). The ONLY place
// entry type is chosen.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { DebitCreditSegment } from './DebitCreditSegment';

const meta: Meta<typeof DebitCreditSegment> = {
  title: 'Components/DebitCreditSegment',
  component: DebitCreditSegment,
};
export default meta;

type Story = StoryObj<typeof DebitCreditSegment>;

export const Debit: Story = { args: { value: 'debit' } };
export const Credit: Story = { args: { value: 'credit' } };
