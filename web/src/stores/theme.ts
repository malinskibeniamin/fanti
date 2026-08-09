import { create } from 'zustand';
import {
  createJSONStorage,
  persist,
  type StateStorage,
} from 'zustand/middleware';

export type Theme = 'light' | 'dark';

const LIGHT_BODY_BACKGROUND = '#f7f0df';
const DARK_BODY_BACKGROUND = '#1d1510';

function systemTheme(): Theme {
  if (typeof window === 'undefined' || !window.matchMedia) {
    return 'light';
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light';
}

/** Tolerates the pre-store format where the raw string "dark"/"light" was persisted. */
const themeStorage: StateStorage = {
  getItem: (name) => {
    const raw = window.localStorage.getItem(name);
    if (raw === 'dark' || raw === 'light') {
      return JSON.stringify({ state: { theme: raw }, version: 0 });
    }
    return raw;
  },
  setItem: (name, value) => window.localStorage.setItem(name, value),
  removeItem: (name) => window.localStorage.removeItem(name),
};

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      theme: systemTheme(),
      setTheme: (theme) => set({ theme }),
      toggleTheme: () =>
        set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),
    }),
    { name: 'fanti-theme', storage: createJSONStorage(() => themeStorage) },
  ),
);

export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark');
  document.body.style.background =
    theme === 'dark' ? DARK_BODY_BACKGROUND : LIGHT_BODY_BACKGROUND;
}

/** Applies the current theme immediately and keeps the DOM in sync with the store. */
export function initTheme() {
  applyTheme(useThemeStore.getState().theme);
  return useThemeStore.subscribe((state) => applyTheme(state.theme));
}
