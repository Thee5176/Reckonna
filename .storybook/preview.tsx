// Global decorator — every story renders inside PaperProvider + paperTheme so
// Paper-backed surfaces (AppNav's tabbar, etc.) match the app's real chrome
// instead of Material defaults. Background matches the design's --bg token
// (design/02-frontend-ledger.design-system.html) so component washes read
// correctly against the ledger's warm surface.
import React from 'react';
import { View } from 'react-native';
import { PaperProvider } from 'react-native-paper';
import type { Preview } from '@storybook/react-native-web-vite';
import { paperTheme } from '../theme/paperTheme';
import { color } from '../theme/tokens';

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
      },
    },
  },
  decorators: [
    (Story) => (
      <PaperProvider theme={paperTheme}>
        <View style={{ minHeight: 40, padding: 24, backgroundColor: color.bg }}>
          <Story />
        </View>
      </PaperProvider>
    ),
  ],
};

export default preview;
