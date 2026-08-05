import path from 'path';
import { createRequire } from 'module';
import { fileURLToPath } from 'url';
import fs from 'fs';
import { defineConfig, loadEnv } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const semiUiDir = path.resolve(
  path.dirname(require.resolve('@douyinfe/semi-ui')),
  '../..',
);

// Resolve date-fns from the semi-ui package so it works with any bun
// node_modules layout (hoisted vs nested). semi-ui requires date-fns@2.x
// which may differ from the top-level date-fns used by other packages.
const semiUiRequire = createRequire(path.resolve(semiUiDir, 'package.json'));
const dateFnsDir = (() => {
  const nested = path.resolve(semiUiDir, 'node_modules/date-fns');
  if (fs.existsSync(nested)) return nested;
  return path.dirname(semiUiRequire.resolve('date-fns/package.json'));
})();

function normalizeBasePath(value: string | undefined): string {
  if (!value || value.trim() === '/' || value.trim() === '') return '';
  return `/${value.trim().replace(/^\/+|\/+$/g, '')}`;
}

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] });
  const basePath = normalizeBasePath(
    process.env.VITE_APP_BASE_PATH || env.rawPublicVars.VITE_APP_BASE_PATH,
  );
  const clientServerUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    '';
  const proxyServerUrl = clientServerUrl || 'http://localhost:3000';
  const isProd = envMode === 'production';
  const devProxy = Object.fromEntries(
    (['/api', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: proxyServerUrl, changeOrigin: true },
    ]),
  ) as Record<string, { target: string; changeOrigin: boolean }>;

  return {
    plugins: [pluginReact()],
    source: {
      entry: {
        index: './src/index.jsx',
      },
      define: {
        'import.meta.env.VITE_REACT_APP_SERVER_URL':
          JSON.stringify(clientServerUrl),
        'import.meta.env.VITE_APP_BASE_PATH': JSON.stringify(basePath),
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        'date-fns': dateFnsDir,
        '@douyinfe/semi-ui/dist/css/semi.css': path.resolve(
          semiUiDir,
          'dist/css/semi.css',
        ),
      },
    },
    html: {
      template: './index.html',
    },
    server: {
      base: basePath || '/',
      host: '0.0.0.0',
      strictPort: true,
      proxy: devProxy,
    },
    output: {
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
      assetPrefix: basePath || '/',
    },
    performance: {
      removeConsole: isProd ? ['log'] : false,
      buildCache: {
        cacheDigest: [process.env.VITE_REACT_APP_VERSION],
      },
    },
    tools: {
      rspack: {
        module: {
          rules: [
            {
              test: /src[\\/].*\.js$/,
              type: 'javascript/auto',
              use: [
                {
                  loader: 'builtin:swc-loader',
                  options: {
                    jsc: {
                      parser: {
                        syntax: 'ecmascript',
                        jsx: true,
                      },
                      transform: {
                        react: {
                          runtime: 'automatic',
                          development: !isProd,
                          refresh: !isProd,
                        },
                      },
                    },
                  },
                },
              ],
            },
          ],
        },
      },
    },
  };
});
