// AmountInput — right-aligned, tabular money entry (design §03). 4dp under
// the hood; display rounds to 2dp when blurred. Non-positive is invalid,
// mirroring the backend's field validation (AT3).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { AmountInput } from './AmountInput';

const meta: Meta<typeof AmountInput> = {
  title: 'Components/AmountInput',
  component: AmountInput,
};
export default meta;

type Story = StoryObj<typeof AmountInput>;

export const Empty: Story = {};
export const Valid: Story = { args: { value: '4200.00' } };
export const Invalid: Story = { args: { value: '-50.00' } };
