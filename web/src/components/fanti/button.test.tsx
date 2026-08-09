import { render, screen } from '@testing-library/react';
import { expect, test, vi } from 'vitest';

import { Button } from '@/components/fanti/button';

test('renders a native button by default', () => {
  render(<Button onClick={() => {}}>Go</Button>);

  const button = screen.getByRole('button', { name: 'Go' });
  expect(button).toBeVisible();
  expect(button.tagName).toBe('BUTTON');
});

test('render prop swaps the element without Base UI complaints', () => {
  const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});

  render(
    <Button onClick={() => {}} render={<a href="/somewhere">Somewhere</a>} />,
  );

  // Base UI keeps button semantics on the swapped element, so the anchor
  // exposes role=button while still navigating.
  const link = screen.getByRole('button', { name: 'Somewhere' });
  expect(link).toBeVisible();
  expect(link.tagName).toBe('A');
  expect(consoleError).not.toHaveBeenCalled();

  consoleError.mockRestore();
});
