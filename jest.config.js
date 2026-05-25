// Jest config for the RN/Expo app. jest-expo preset handles RN Web + native transforms.
module.exports = {
  preset: 'jest-expo',
  transformIgnorePatterns: [
    'node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@unimodules/.*|unimodules|sentry-expo|native-base|react-native-svg|react-native-paper))',
  ],
  setupFilesAfterEnv: [],
  testMatch: ['**/*.test.ts', '**/*.test.tsx'],
};
