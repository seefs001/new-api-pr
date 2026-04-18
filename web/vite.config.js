/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import react from '@vitejs/plugin-react';
import { defineConfig, transformWithEsbuild } from 'vite';
import path from 'path';
import { codeInspectorPlugin } from 'code-inspector-plugin';
import semiViteCompatPlugin from './vite-plugin-semi-compat.js';

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: [
      {
        find: /^@lobehub\/ui\/icons$/,
        replacement: path.resolve(
          __dirname,
          './src/shims/lobehub-ui-icons.jsx',
        ),
      },
      {
        find: /^@lobehub\/ui$/,
        replacement: path.resolve(__dirname, './src/shims/lobehub-ui.jsx'),
      },
      {
        find: '@',
        replacement: path.resolve(__dirname, './src'),
      },
    ],
  },
  plugins: [
    codeInspectorPlugin({
      bundler: 'vite',
    }),
    {
      name: 'treat-js-files-as-jsx',
      async transform(code, id) {
        if (!/src\/.*\.js$/.test(id)) {
          return null;
        }

        // Use the exposed transform from vite, instead of directly
        // transforming with esbuild
        return transformWithEsbuild(code, id, {
          loader: 'jsx',
          jsx: 'automatic',
        });
      },
    },
    react(),
    semiViteCompatPlugin({
      cssLayer: false,
    }),
  ],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('/src/helpers/render.jsx')) {
            return 'helper-render';
          }
          if (id.includes('/src/helpers/dashboard/')) {
            return 'helper-dashboard';
          }
          if (id.includes('/src/helpers/layoutIcons.jsx')) {
            return 'layout-icons';
          }
          if (id.includes('/src/components/common/markdown/')) {
            return 'markdown-renderer';
          }
          if (!id.includes('node_modules')) {
            return;
          }
          if (
            id.includes('/react/') ||
            id.includes('/react-dom/') ||
            id.includes('/react-router-dom/')
          ) {
            return 'react-core';
          }
          if (
            id.includes('/@douyinfe/semi-icons/') ||
            id.includes('/@douyinfe/semi-ui/') ||
            id.includes('/@douyinfe/semi-foundation/')
          ) {
            return 'semi-ui';
          }
          if (
            id.includes('/@lobehub/icons/') ||
            id.includes('/@lobehub/ui/') ||
            id.includes('/antd-style/') ||
            id.includes('/react-layout-kit/')
          ) {
            return 'lobe-icons';
          }
          if (
            id.includes('/@visactor/') ||
            id.includes('/@visactor/vchart') ||
            id.includes('/@visactor/vrender') ||
            id.includes('/@visactor/react-vchart')
          ) {
            return 'visactor';
          }
          if (
            id.includes('/react-markdown/') ||
            id.includes('/remark-') ||
            id.includes('/rehype-') ||
            id.includes('/highlight.js/') ||
            id.includes('/mermaid/')
          ) {
            return 'markdown-vendor';
          }
          if (
            id.includes('/cytoscape/') ||
            id.includes('/cytoscape-') ||
            id.includes('/dagre') ||
            id.includes('/cose-bilkent/')
          ) {
            return 'graph-vendor';
          }
          if (id.includes('/katex/')) {
            return 'katex';
          }
          if (
            id.includes('/axios/') ||
            id.includes('/history/') ||
            id.includes('/marked/')
          ) {
            return 'tools';
          }
          if (
            id.includes('/react-dropzone/') ||
            id.includes('/react-fireworks/') ||
            id.includes('/react-telegram-login/') ||
            id.includes('/react-toastify/') ||
            id.includes('/react-turnstile/')
          ) {
            return 'react-components';
          }
          if (
            id.includes('/i18next/') ||
            id.includes('/react-i18next/') ||
            id.includes('/i18next-browser-languagedetector/')
          ) {
            return 'i18n';
          }
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/mj': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/pg': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
    },
  },
});
