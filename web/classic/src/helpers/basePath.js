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

function normalizeBasePath(value) {
  if (typeof value !== 'string') return '';
  const trimmed = value.trim();
  if (!trimmed || trimmed === '/') return '';
  return `/${trimmed.replace(/^\/+|\/+$/g, '')}`;
}

function detectRuntimeBasePath() {
  if (typeof document === 'undefined') return '';
  const assetElements = document.querySelectorAll(
    'script[src*="/static/"],link[href*="/static/"]',
  );
  for (const element of assetElements) {
    const assetUrl = element.src || element.href;
    try {
      const url = new URL(assetUrl, window.location.origin);
      const markerIndex = url.pathname.indexOf('/static/');
      if (markerIndex > 0) {
        return normalizeBasePath(url.pathname.slice(0, markerIndex));
      }
    } catch {
      // Ignore malformed asset URLs and continue probing other assets.
    }
  }
  return '';
}

export const APP_BASE_PATH = normalizeBasePath(
  import.meta.env.VITE_APP_BASE_PATH,
) || detectRuntimeBasePath();

export function withBasePath(path) {
  if (!APP_BASE_PATH) return path;
  if (!path || path.startsWith('http://') || path.startsWith('https://')) {
    return path;
  }
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  if (
    normalizedPath === APP_BASE_PATH ||
    normalizedPath.startsWith(`${APP_BASE_PATH}/`)
  ) {
    return normalizedPath;
  }
  return `${APP_BASE_PATH}${normalizedPath}`;
}
