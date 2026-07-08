// EmptyState — the empty branch every async surface must ship (design §04).
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { EmptyState } from './EmptyState';

const meta: Meta<typeof EmptyState> = {
  title: 'Components/EmptyState',
  component: EmptyState,
};
export default meta;

type Story = StoryObj<typeof EmptyState>;

export const Default: Story = {};
export const WithAction: Story = {
  args: { actionLabel: '+ New entry', onAction: () => {} },
};
