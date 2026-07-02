// Storybook config for the Forest Reserve component library. Framework is
// @storybook/react-native-web-vite — the official Storybook path for an
// Expo/RN-Web component library (renders RN + react-native-web components in
// a real browser via Vite, no on-device simulator required). See plan-04
// enhancement brief for why this framework over @storybook/react-native.
import type { StorybookConfig } from '@storybook/react-native-web-vite';

const config: StorybookConfig = {
  stories: ['../components/**/*.stories.@(ts|tsx)'],
  addons: [],
  framework: {
    name: '@storybook/react-native-web-vite',
    options: {},
  },
};

export default config;
