// Jest config for the RN/Expo app. jest-expo preset handles RN Web + native transforms.
module.exports = {
  preset: 'jest-expo',
  transformIgnorePatterns: [
    'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@unimodules/.*|unimodules|sentry-expo|native-base|react-native-svg|react-native-paper))',
  ],
  setupFilesAfterEnv: [],
  testMatch: ['**/*.test.ts', '**/*.test.tsx'],
  // Component-library coverage surface (plan-04 enhancement: 100% target).
  // Stories, the barrel, and type-only files carry no runtime logic to cover.
  collectCoverageFrom: [
    'components/**/*.{ts,tsx}',
    'theme/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    '!**/*.test.{ts,tsx}',
    '!**/*.stories.{ts,tsx}',
    '!components/index.ts',
  ],
};
