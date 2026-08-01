/* eslint-env node */
module.exports = {
  root: true,
  env: { browser: true, es2022: true },
  parser: '@typescript-eslint/parser',
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
  plugins: ['@typescript-eslint', 'react-hooks'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended'],
  ignorePatterns: ['dist', 'node_modules', '*.cjs', 'vite.config.ts'],
  rules: {
    'react-hooks/rules-of-hooks': 'error',
    'react-hooks/exhaustive-deps': 'warn',
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    // docs/12 A03: the React-safe rendering path is not optional.
    'no-restricted-properties': [
      'error',
      {
        object: 'dangerouslySetInnerHTML',
        message: 'Banned: output encoding is our XSS defence (docs/12, A03).',
      },
    ],
    'no-restricted-syntax': [
      'error',
      {
        selector: 'JSXAttribute[name.name="dangerouslySetInnerHTML"]',
        message: 'Banned: output encoding is our XSS defence (docs/12, A03).',
      },
    ],
  },
}
