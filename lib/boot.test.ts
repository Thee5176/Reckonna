// S1 boot smoke: jest-expo boots and the bundled fonts + svg are importable.
import { useFonts as useSerif } from '@expo-google-fonts/source-serif-4';
import { useFonts as useMono } from '@expo-google-fonts/jetbrains-mono';
import Svg from 'react-native-svg';

describe('S1 boot', () => {
  it('imports the bundled Google fonts (offline, not CDN)', () => {
    expect(useSerif).toBeDefined();
    expect(useMono).toBeDefined();
  });

  it('imports react-native-svg', () => {
    expect(Svg).toBeDefined();
  });
});
