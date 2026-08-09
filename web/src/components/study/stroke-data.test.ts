import { expect, test } from 'vitest';

import { parseDrawnPath } from '@/components/study/stroke-data';

test('parses Hanzi Writer external drawing coordinates', () => {
  expect(parseDrawnPath('M 12 34 L 56.5 78 L -1 2')).toEqual([
    { x: 12, y: 34 },
    { x: 56.5, y: 78 },
    { x: -1, y: 2 },
  ]);
});

test('rejects malformed Hanzi Writer drawing paths', () => {
  expect(parseDrawnPath('M 12 nope')).toBeUndefined();
  expect(parseDrawnPath('L 12 34')).toBeUndefined();
  expect(parseDrawnPath('M 12 34 C 4 5')).toBeUndefined();
});
