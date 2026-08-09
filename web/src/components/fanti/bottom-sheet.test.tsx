import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { expect, test, vi } from 'vitest';

import { BottomSheet } from '@/components/fanti/bottom-sheet';

function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button type="button" onClick={() => setOpen(true)}>
        Open sheet
      </button>
      <BottomSheet
        open={open}
        onClose={() => setOpen(false)}
        ariaLabel="Dictionary"
      >
        <p>Sheet body</p>
        <button type="button" onClick={() => setOpen(false)}>
          First action
        </button>
        <button type="button">Last action</button>
      </BottomSheet>
    </div>
  );
}

test('renders dialog semantics when open', async () => {
  const user = userEvent.setup();
  render(<Harness />);

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Open sheet' }));

  const dialog = screen.getByRole('dialog', { name: 'Dictionary' });
  expect(dialog).toBeVisible();
  expect(dialog).toHaveAttribute('aria-modal', 'true');
});

test('moves focus into the sheet when opened', async () => {
  const user = userEvent.setup();
  render(<Harness />);

  await user.click(screen.getByRole('button', { name: 'Open sheet' }));

  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'First action' })).toHaveFocus(),
  );
});

test('closes on Escape and returns focus to the opener', async () => {
  const user = userEvent.setup();
  render(<Harness />);

  const opener = screen.getByRole('button', { name: 'Open sheet' });
  await user.click(opener);
  expect(screen.getByRole('dialog')).toBeVisible();

  await user.keyboard('{Escape}');

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  await waitFor(() => expect(opener).toHaveFocus());
});

test('closes when the backdrop is clicked', async () => {
  const user = userEvent.setup();
  render(<Harness />);

  await user.click(screen.getByRole('button', { name: 'Open sheet' }));
  await user.click(screen.getByTestId('bottom-sheet-backdrop'));

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

test('traps Tab focus inside the sheet', async () => {
  const user = userEvent.setup();
  render(<Harness />);

  await user.click(screen.getByRole('button', { name: 'Open sheet' }));
  const first = screen.getByRole('button', { name: 'First action' });
  const last = screen.getByRole('button', { name: 'Last action' });

  await waitFor(() => expect(first).toHaveFocus());

  await user.tab();
  expect(last).toHaveFocus();

  await user.tab();
  expect(first).toHaveFocus();

  await user.tab({ shift: true });
  expect(last).toHaveFocus();
});

test('does not invoke onClose while closed', async () => {
  const onClose = vi.fn();
  render(
    <BottomSheet open={false} onClose={onClose} ariaLabel="Dictionary">
      <p>Sheet body</p>
    </BottomSheet>,
  );

  const user = userEvent.setup();
  await user.keyboard('{Escape}');
  expect(onClose).not.toHaveBeenCalled();
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});
