import { withRsbuildConfig } from '@rstest/adapter-rsbuild';
import { defineConfig } from '@rstest/core';

export default defineConfig({
  extends: withRsbuildConfig(),
  testEnvironment: 'happy-dom',
  setupFiles: ['./test/setup.ts'],
  include: ['src/**/*.{test,spec}.{ts,tsx}'],
  coverage: {
    provider: 'istanbul',
    reporters: ['text', 'html'],
    include: ['src/**/*.{ts,tsx}'],
    exclude: ['src/**/*.{test,spec}.{ts,tsx}', 'src/**/*.module.css'],
  },
});
