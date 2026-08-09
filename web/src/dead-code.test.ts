import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

const KEEP_UI = [
  'button.tsx',
  'input.tsx',
  'skeleton.tsx',
  'switch.tsx',
  'textarea.tsx',
  'tooltip.tsx',
];

test('keeps only shipped routes, screens, and UI primitives', () => {
  for (const path of [
    'src/routes/components.tsx',
    'src/components/brand-crest.tsx',
    'src/components/library/book-detail-screen.tsx',
    'src/components/library/library-screen.tsx',
    'src/hooks/use-mobile.ts',
  ]) {
    expect(existsSync(resolve(path)), `${path} should be removed`).toBe(false);
  }

  expect(readdirSync(resolve('src/components/ui')).sort()).toEqual(KEEP_UI);
});

test('does not ship packages used only by removed registry components', () => {
  const packageJson = readFileSync(resolve('package.json'), 'utf8');
  for (const dependency of [
    'cmdk',
    'embla-carousel-react',
    'input-otp',
    'next-themes',
    'react-day-picker',
    'react-resizable-panels',
    'recharts',
    'vaul',
  ]) {
    expect(packageJson).not.toContain(`"${dependency}"`);
  }
});
