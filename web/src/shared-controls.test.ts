import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) {
      return path.endsWith('/components/ui') ? [] : sourceFiles(path);
    }
    return /\.(ts|tsx)$/.test(entry.name) && !entry.name.includes('.test.')
      ? [path]
      : [];
  });
}

test('production controls use shared UI primitives', () => {
  const violations = sourceFiles(resolve('src')).filter((path) =>
    /<(button|input|textarea)(\s|>)/.test(readFileSync(path, 'utf8')),
  );

  expect(violations).toEqual([]);
});
