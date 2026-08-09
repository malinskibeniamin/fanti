import { beforeEach, expect, test } from 'vitest';

import { translate, translateGloss, useLocaleStore } from '@/i18n/locale';
import { STR } from '@/i18n/strings';

beforeEach(() => {
  window.localStorage.clear();
  useLocaleStore.setState({ locale: 'tc' });
});

test('defaults to traditional chinese', () => {
  expect(useLocaleStore.getState().locale).toBe('tc');
  expect(translate('tc', 'navLib')).toBe(STR.navLib[1]);
});

test('translate returns the active locale string', () => {
  expect(translate('en', 'navLib')).toBe('Library');
  expect(translate('tc', 'navLib')).toBe('書庫');
  expect(translate('sc', 'navLib')).toBe('书库');
});

test('gloss is traditional for english and english otherwise', () => {
  expect(translateGloss('en', 'navLib')).toBe(STR.navLib[1]);
  expect(translateGloss('tc', 'navLib')).toBe(STR.navLib[0]);
  expect(translateGloss('sc', 'navLib')).toBe(STR.navLib[0]);
});

test('setLocale persists to the fanti-locale key', () => {
  useLocaleStore.getState().setLocale('en');

  expect(useLocaleStore.getState().locale).toBe('en');
  const raw = window.localStorage.getItem('fanti-locale');
  expect(raw).not.toBeNull();
  expect(JSON.parse(raw ?? '{}').state.locale).toBe('en');
});

test('rehydrates a persisted locale', async () => {
  window.localStorage.setItem(
    'fanti-locale',
    JSON.stringify({ state: { locale: 'sc' }, version: 0 }),
  );

  await useLocaleStore.persist.rehydrate();

  expect(useLocaleStore.getState().locale).toBe('sc');
});

test('every string key carries all three locales', () => {
  for (const values of Object.values(STR)) {
    expect(values).toHaveLength(3);
    for (const value of values) {
      expect(typeof value).toBe('string');
      expect(value.length).toBeGreaterThan(0);
    }
  }
});
