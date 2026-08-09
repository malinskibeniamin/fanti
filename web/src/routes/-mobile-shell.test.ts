import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

test('keeps sticky and fixed navigation clear of phone safe areas', () => {
  const rootRoute = readFileSync(resolve('src/routes/__root.tsx'), 'utf8');

  expect(rootRoute).toContain('pt-[env(safe-area-inset-top)]');
  expect(rootRoute).toContain('env(safe-area-inset-bottom)');
});
