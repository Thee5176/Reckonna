// Spinner — indeterminate progress (design §04). Pairs with Skeleton in the
// loading state.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Spinner } from './Spinner';

const meta: Meta<typeof Spinner> = {
  title: 'Components/Spinner',
  component: Spinner,
};
export default meta;

type Story = StoryObj<typeof Spinner>;

export const Bare: Story = {};
export const WithLabel: Story = { args: { label: 'Fetching ledgers…' } };
