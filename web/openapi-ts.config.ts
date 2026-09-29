import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../openapi/v1/openapi.yaml',
  output: { path: 'src/infrastructure/api' },
  plugins: [
    '@hey-api/client-fetch',
    '@hey-api/typescript',
    'zod',
    { name: '@hey-api/sdk', validator: true },
    '@tanstack/vue-query',
  ],
})
