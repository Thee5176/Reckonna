// Alert — the error branch every async surface must ship (design §04), keyed
// by the plan-03 RFC 7807 `code` (never localized text, IT5). server_error is
// the only retryable code.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Alert } from './Alert';

const meta: Meta<typeof Alert> = {
  title: 'Components/Alert',
  component: Alert,
};
export default meta;

type Story = StoryObj<typeof Alert>;

export const UnbalancedEntry: Story = { args: { code: 'unbalanced_entry' } };
export const ValidationFailed: Story = { args: { code: 'validation_failed' } };
export const Unauthorized: Story = { args: { code: 'unauthorized' } };
export const ServerErrorWithRetry: Story = {
  args: { code: 'server_error', onRetry: () => {} },
};
