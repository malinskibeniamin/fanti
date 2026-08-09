import type { StringKey } from '@/i18n/strings';
import type { PinyinMode } from '@/stores/reader-prefs';

/** Chip label for each pinyin mode — shared by the toolbar and settings sheet. */
export const PINYIN_LABEL_KEY: Record<PinyinMode, StringKey> = {
  off: 'pyOff',
  hints: 'pyHint',
  all: 'pyAll',
};

/** Toolbar 拼 chip cycle: off → hints → all → off. */
export const PINYIN_CYCLE: Record<PinyinMode, PinyinMode> = {
  off: 'hints',
  hints: 'all',
  all: 'off',
};
