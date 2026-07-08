// Skeleton — loading placeholder bar (design §04). One of the three states
// every async surface must ship.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Skeleton } from './Skeleton';

const meta: Meta<typeof Skeleton> = {
  title: 'Components/Skeleton',
  component: Skeleton,
};
export default meta;

type Story = StoryObj<typeof Skeleton>;

export const Default: Story = {};
export const Narrow: Story = { args: { width: '40%', height: 10 } };
