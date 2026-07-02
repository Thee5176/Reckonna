// Button — design §02 controls: five variants + the disabled (unbalanced →
// cannot post) state.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { Button } from './Button';

const meta: Meta<typeof Button> = {
  title: 'Components/Button',
  component: Button,
  args: {
    label: 'Post entry →',
    onPress: () => {},
  },
};
export default meta;

type Story = StoryObj<typeof Button>;

export const Primary: Story = { args: { variant: 'primary' } };
export const Secondary: Story = { args: { variant: 'secondary' } };
export const Ghost: Story = { args: { variant: 'ghost' } };
export const Accent: Story = { args: { variant: 'accent', label: 'Review balance →' } };
export const Danger: Story = { args: { variant: 'danger', label: 'Delete' } };
export const Disabled: Story = {
  args: { variant: 'primary', disabled: true, label: 'Post entry →' },
};
