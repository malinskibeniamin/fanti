import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { useShallow } from 'zustand/react/shallow';

import { STR, type StringKey } from '@/i18n/strings';

export type Locale = 'en' | 'tc' | 'sc';

export const LOCALE_IDX: Record<Locale, 0 | 1 | 2> = { en: 0, tc: 1, sc: 2 };

export function isLocale(value: unknown): value is Locale {
  return value === 'en' || value === 'tc' || value === 'sc';
}

interface LocaleState {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

export const useLocaleStore = create<LocaleState>()(
  persist(
    (set) => ({
      locale: 'tc',
      setLocale: (locale) => set({ locale }),
    }),
    { name: 'fanti-locale' },
  ),
);

/** Active-locale string for a design-system key. */
export function translate(locale: Locale, key: StringKey): string {
  return STR[key][LOCALE_IDX[locale]];
}

/**
 * Secondary-script gloss — the design pairs every label with a gloss:
 * Traditional when the UI is English, English otherwise.
 */
export function translateGloss(locale: Locale, key: StringKey): string {
  return locale === 'en' ? STR[key][1] : STR[key][0];
}

export function useLocale() {
  const { locale, setLocale } = useLocaleStore(
    useShallow((state) => ({
      locale: state.locale,
      setLocale: state.setLocale,
    })),
  );

  const t = (key: StringKey) => translate(locale, key);
  const tGloss = (key: StringKey) => translateGloss(locale, key);

  return { locale, setLocale, t, tGloss };
}
