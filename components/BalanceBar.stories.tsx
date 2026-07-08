// BalanceBar — the 借方=貸方 gate (design §05, AT1/AT2). ok|bad is COMPUTED
// from money.isBalanced; there is deliberately no "balanced" prop.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { BalanceBar } from './BalanceBar';

const meta: Meta<typeof BalanceBar> = {
  title: 'Components/BalanceBar',
  component: BalanceBar,
};
export default meta;

type Story = StoryObj<typeof BalanceBar>;

export const Balanced: Story = {
  args: { debits: ['1000.00'], credits: ['1000.00'] },
};
export const Unbalanced: Story = {
  args: { debits: ['1000.00'], credits: ['500.00'] },
};
export const WithCta: Story = {
  args: {
    debits: ['1000.00'],
    credits: ['1000.00'],
    ctaLabel: 'Review balance →',
    onCta: () => {},
  },
};
export const CtaDisabledWhileUnbalanced: Story = {
  args: {
    debits: ['1000.00'],
    credits: ['500.00'],
    ctaLabel: 'Review balance →',
    onCta: () => {},
  },
};
export const CheckVariant: Story = {
  args: {
    variant: 'check',
    debits: ['96850.18'],
    credits: ['96850.18'],
  },
};
