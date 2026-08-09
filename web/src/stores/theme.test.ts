import { beforeEach, expect, test } from 'vitest';

import { applyTheme, initTheme, useThemeStore } from '@/stores/theme';

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.classList.remove('dark');
  useThemeStore.setState({ theme: 'light' });
});

test('defaults to light when the system gives no preference', () => {
  // jsdom has no matchMedia, which is the "no preference" fallback path.
  expect(useThemeStore.getState().theme).toBe('light');
});

test('toggleTheme flips between light and dark', () => {
  useThemeStore.getState().toggleTheme();
  expect(useThemeStore.getState().theme).toBe('dark');

  useThemeStore.getState().toggleTheme();
  expect(useThemeStore.getState().theme).toBe('light');
});

test('setTheme persists to the fanti-theme key', () => {
  useThemeStore.getState().setTheme('dark');

  const raw = window.localStorage.getItem('fanti-theme');
  expect(raw).not.toBeNull();
  expect(JSON.parse(raw ?? '{}').state.theme).toBe('dark');
});

test('applyTheme toggles the dark class and syncs the body background', () => {
  applyTheme('dark');
  expect(document.documentElement.classList.contains('dark')).toBe(true);
  expect(document.body.style.background).toContain('rgb(29, 21, 16)');

  applyTheme('light');
  expect(document.documentElement.classList.contains('dark')).toBe(false);
  expect(document.body.style.background).toContain('rgb(247, 240, 223)');
});

test('initTheme applies the current theme and tracks store changes', () => {
  const unsubscribe = initTheme();
  expect(document.documentElement.classList.contains('dark')).toBe(false);

  useThemeStore.getState().setTheme('dark');
  expect(document.documentElement.classList.contains('dark')).toBe(true);

  unsubscribe();
  useThemeStore.getState().setTheme('light');
  expect(document.documentElement.classList.contains('dark')).toBe(true);
});

test('rehydrates a persisted override', async () => {
  window.localStorage.setItem(
    'fanti-theme',
    JSON.stringify({ state: { theme: 'dark' }, version: 0 }),
  );

  await useThemeStore.persist.rehydrate();

  expect(useThemeStore.getState().theme).toBe('dark');
});

test('tolerates the legacy raw string storage format', async () => {
  window.localStorage.setItem('fanti-theme', 'dark');

  await useThemeStore.persist.rehydrate();

  expect(useThemeStore.getState().theme).toBe('dark');
});
