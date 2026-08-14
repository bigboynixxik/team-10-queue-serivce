import { defineConfig } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

export default defineConfig({
  plugins: [pluginReact()],
  html: {
    title: 'Авито Очередь',
    meta: {
      description: 'Сервис очереди для покупки дефицитных товаров',
    },
  },
  source: {
    entry: {
      index: './src/app/index.tsx',
    },
  },
  resolve: {
    alias: {
      '@': './src',
      '@app': './src/app',
      '@pages': './src/pages',
      '@widgets': './src/widgets',
      '@features': './src/features',
      '@entities': './src/entities',
      '@shared': './src/shared',
      '@ui': './src/shared/ui',
      '@test': './test',
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
  },
});
