import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import vueA11y from 'eslint-plugin-vuejs-accessibility'

export default tseslint.config(
  { ignores: ['dist/**', 'coverage/**', 'src/infrastructure/api/**', 'playwright-report/**', 'test-results/**', 'android/**'] },
  ...tseslint.configs.strictTypeChecked,
  ...pluginVue.configs['flat/recommended'],
  ...vueA11y.configs['flat/recommended'],
  {
    files: ['**/*.{ts,vue}'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
        projectService: true,
        extraFileExtensions: ['.vue'],
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
      'vue/multi-word-component-names': 'off',
      'vuejs-accessibility/label-has-for': ['error', { required: { some: ['nesting', 'id'] } }],
      'vue/max-attributes-per-line': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/multiline-html-element-content-newline': 'off',
      'vue/html-self-closing': 'off',
    },
  },
  { files: ['**/*.spec.ts', 'tests/**/*.ts', 'src/test/**/*.ts'], rules: { '@typescript-eslint/require-await': 'off' } },
  {
    files: ['tests/e2e/**/*.ts'],
    ignores: ['tests/e2e/axe.ts'],
    rules: { 'no-restricted-imports': ['error', { paths: [{ name: '@axe-core/playwright', message: "Use violations() from './axe', which keeps a map's cells from slowing the scan." }] }] },
  },
  { files: ['eslint.config.js'], ...tseslint.configs.disableTypeChecked },
)
