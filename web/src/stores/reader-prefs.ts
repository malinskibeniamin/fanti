import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type ReaderFont = 'serif' | 'kai';
export type PinyinMode = 'off' | 'hints' | 'all';
export type ReaderLineHeight = 1.6 | 2 | 2.4;

export const READER_SIZE_MIN = 14;
export const READER_SIZE_MAX = 26;
const READER_SIZE_STEP = 1;
const READER_SIZE_DEFAULT = 19;

export const READER_FONT_VAR: Record<ReaderFont, string> = {
  serif: 'var(--font-reading)',
  kai: 'var(--font-display)',
};

interface ReaderPrefsState {
  size: number;
  font: ReaderFont;
  pinyin: PinyinMode;
  lineHeight: ReaderLineHeight;
  traditional: boolean;
  increaseSize: () => void;
  decreaseSize: () => void;
  setFont: (font: ReaderFont) => void;
  setPinyin: (pinyin: PinyinMode) => void;
  setLineHeight: (lineHeight: ReaderLineHeight) => void;
  setTraditional: (traditional: boolean) => void;
}

function clampSize(size: number): number {
  return Math.min(READER_SIZE_MAX, Math.max(READER_SIZE_MIN, size));
}

/** Reader typography preferences, persisted to local storage. */
export const useReaderPrefs = create<ReaderPrefsState>()(
  persist(
    (set) => ({
      size: READER_SIZE_DEFAULT,
      font: 'serif',
      pinyin: 'off',
      lineHeight: 2,
      traditional: true,
      increaseSize: () =>
        set((state) => ({ size: clampSize(state.size + READER_SIZE_STEP) })),
      decreaseSize: () =>
        set((state) => ({ size: clampSize(state.size - READER_SIZE_STEP) })),
      setFont: (font) => set({ font }),
      setPinyin: (pinyin) => set({ pinyin }),
      setLineHeight: (lineHeight) => set({ lineHeight }),
      setTraditional: (traditional) => set({ traditional }),
    }),
    { name: 'fanti-reader-prefs' },
  ),
);
