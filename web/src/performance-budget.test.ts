import { readFileSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

test('keeps global font and header-logo payloads bounded', () => {
  const css = readFileSync(resolve('src/index.css'), 'utf8');
  const rootRoute = readFileSync(resolve('src/routes/__root.tsx'), 'utf8');

  expect(css.match(/@font-face/g)).toHaveLength(2);
  expect(rootRoute).toContain('src="/fanti-mark.svg"');
  expect(statSync(resolve('public/icon-192.png')).size).toBeLessThan(100_000);
});
