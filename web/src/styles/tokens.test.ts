import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

test('uses Source Sans for English before CJK fallbacks', () => {
  const tokens = readFileSync(resolve('src/styles/tokens.css'), 'utf8');
  const uiFamily = tokens.match(/--font-ui:\s*([^;]+);/)?.[1];
  const displayFamily = tokens.match(/--font-display:\s*([^;]+);/)?.[1];

  expect(uiFamily).toMatch(/^"Source Sans 3 Variable",/);
  expect(displayFamily).toMatch(/^"Source Sans 3 Variable",/);
  expect(displayFamily).toContain('"Kaiti TC"');
});

test('does not apply CJK tracking to all English UI text', () => {
  const indexCss = readFileSync(resolve('src/index.css'), 'utf8');
  const bodyRule = indexCss.match(/body\s*{([^}]+)}/)?.[1];

  expect(bodyRule).not.toContain('var(--tracking-han)');
});

test('uses Source Sans for pinyin tone marks', () => {
  const indexCss = readFileSync(resolve('src/index.css'), 'utf8');

  expect(indexCss).toContain('source-sans-3-latin-ext-wght-normal.woff2');
});
