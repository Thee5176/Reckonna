import React, { useEffect } from 'react';
import { Stack } from 'expo-router';
import { PaperProvider } from 'react-native-paper';
import { useFonts as useJetBrainsMono, JetBrainsMono_400Regular } from '@expo-google-fonts/jetbrains-mono';
import {
  useFonts as useSourceSerif,
  SourceSerif4_400Regular,
  SourceSerif4_600SemiBold,
} from '@expo-google-fonts/source-serif-4';
import { paperTheme } from '../theme/paperTheme';

export default function RootLayout() {
  const [monoLoaded] = useJetBrainsMono({ 'JetBrains Mono': JetBrainsMono_400Regular });
  const [serifLoaded] = useSourceSerif({
    'Source Serif 4': SourceSerif4_400Regular,
    'Source Serif 4_600': SourceSerif4_600SemiBold,
  });

  useEffect(() => {
    // fonts registered under exact family names tokens.font.mono / .serif reference
  }, []);

  if (!monoLoaded || !serifLoaded) return null;

  return (
    <PaperProvider theme={paperTheme}>
      <Stack screenOptions={{ headerShown: false }} />
    </PaperProvider>
  );
}
