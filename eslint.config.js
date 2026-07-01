// Flat ESLint config for the RN/Expo app (ESLint 9). Extends eslint-config-expo.
const expoConfig = require('eslint-config-expo/flat');

module.exports = [
  ...expoConfig,
  {
    ignores: ['node_modules/**', 'dist/**', '.expo/**'],
  },
];
