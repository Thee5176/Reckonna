// AppNav — adaptive navigation chrome (design §08, AT7): a top navrow on web,
// a bottom tabbar on native. `platform` overrides Platform.OS so both chromes
// are visible here regardless of the browser Storybook runs in.
import type { Meta, StoryObj } from '@storybook/react-native-web-vite';
import { AppNav } from './AppNav';

const meta: Meta<typeof AppNav> = {
  title: 'Components/AppNav',
  component: AppNav,
  args: {
    activeKey: 'ledger',
    onNavigate: () => {},
  },
};
export default meta;

type Story = StoryObj<typeof AppNav>;

export const Web: Story = { args: { platform: 'web' } };
export const Native: Story = { args: { platform: 'native' } };
