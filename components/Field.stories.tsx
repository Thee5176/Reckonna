// Field — label + input + error wrapper (design §03). The invalid variant
// mirrors backend validation (plan 03 field errors).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Field } from './Field';

const meta: Meta<typeof Field> = {
  title: 'Components/Field',
  component: Field,
  args: {
    label: 'Description',
    placeholder: 'Stripe payout · 14 invoices',
  },
};
export default meta;

type Story = StoryObj<typeof Field>;

export const Default: Story = {};
export const Focused: Story = { args: { focused: true, value: 'Stripe payout' } };
export const Invalid: Story = {
  args: { invalid: true, error: 'This field is required.', value: '' },
};
export const RightAligned: Story = {
  args: { label: 'Amount · JPY', value: '1,000.00', rightAlign: true },
};
