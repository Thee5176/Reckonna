// Badge — the entry-status chip (design §02): draft → review → posted →
// flagged.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Badge } from './Badge';

const meta: Meta<typeof Badge> = {
  title: 'Components/Badge',
  component: Badge,
};
export default meta;

type Story = StoryObj<typeof Badge>;

export const Draft: Story = { args: { label: 'draft', status: 'draft' } };
export const Review: Story = { args: { label: 'review', status: 'review' } };
export const Posted: Story = { args: { label: 'posted', status: 'posted' } };
export const Flagged: Story = { args: { label: 'flagged', status: 'flagged' } };
