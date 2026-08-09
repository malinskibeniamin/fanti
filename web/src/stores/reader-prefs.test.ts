import { beforeEach, expect, test } from 'vitest';

import {
  READER_SIZE_MAX,
  READER_SIZE_MIN,
  useReaderPrefs,
} from '@/stores/reader-prefs';

beforeEach(() => {
  window.localStorage.clear();
  useReaderPrefs.setState({
    size: 19,
    font: 'serif',
    pinyin: 'off',
    lineHeight: 2,
    traditional: true,
  });
});

test('size steppers clamp to the 14-26 range', () => {
  const { increaseSize, decreaseSize } = useReaderPrefs.getState();

  useReaderPrefs.setState({ size: READER_SIZE_MAX });
  increaseSize();
  expect(useReaderPrefs.getState().size).toBe(READER_SIZE_MAX);

  useReaderPrefs.setState({ size: READER_SIZE_MIN });
  decreaseSize();
  expect(useReaderPrefs.getState().size).toBe(READER_SIZE_MIN);

  increaseSize();
  expect(useReaderPrefs.getState().size).toBe(READER_SIZE_MIN + 1);
});

test('persists preference changes to local storage', () => {
  useReaderPrefs.getState().setPinyin('hints');
  useReaderPrefs.getState().setTraditional(false);

  const raw = window.localStorage.getItem('fanti-reader-prefs');
  expect(raw).not.toBeNull();
  const persisted = JSON.parse(raw ?? '{}') as {
    state: { pinyin: string; traditional: boolean };
  };
  expect(persisted.state.pinyin).toBe('hints');
  expect(persisted.state.traditional).toBe(false);
});
