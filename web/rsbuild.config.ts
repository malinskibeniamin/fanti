import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from '@rsbuild/core';
import { pluginBabel } from '@rsbuild/plugin-babel';
import { pluginReact } from '@rsbuild/plugin-react';
import { pluginTailwindcss } from '@rsbuild/plugin-tailwindcss';
import { tanstackRouter } from '@tanstack/router-plugin/rspack';

const rootDir = dirname(fileURLToPath(import.meta.url));

// Docs: https://rsbuild.rs/config/
export default defineConfig({
  plugins: [
    pluginReact(),
    pluginBabel({
      include: /\.[jt]sx?$/,
      exclude: [/[\\/]node_modules[\\/]/],
      babelLoaderOptions(opts) {
        opts.plugins?.unshift('babel-plugin-react-compiler');
      },
    }),
    pluginTailwindcss(),
  ],
  source: {
    alias: {
      '@': resolve(rootDir, 'src'),
    },
  },
  html: {
    title: 'Fanti 繁体 · 玉簡閣',
    favicon: './public/favicon.png',
    meta: {
      'theme-color': '#8f1d18',
      description:
        'Convert books between Simplified and Traditional Chinese, read with tap-to-define, and study characters.',
    },
    tags: [
      {
        tag: 'link',
        attrs: { rel: 'manifest', href: '/manifest.webmanifest' },
      },
      {
        tag: 'link',
        attrs: { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
      },
    ],
  },
  tools: {
    rspack: {
      plugins: [
        tanstackRouter({
          target: 'react',
          autoCodeSplitting: true,
        }),
      ],
    },
  },
});
