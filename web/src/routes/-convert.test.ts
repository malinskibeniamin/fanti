import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

test('clears the conversion only after delete succeeds', () => {
  const source = readFileSync(resolve('src/routes/convert.tsx'), 'utf8');
  const mutation = source.slice(
    source.indexOf('const deleteMutation'),
    source.indexOf('function setConversion'),
  );

  expect(mutation).toContain('onSuccess: () => setConversion(undefined)');
  expect(mutation).not.toContain('onSettled');
});
