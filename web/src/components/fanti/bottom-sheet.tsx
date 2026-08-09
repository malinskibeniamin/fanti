import { useEffect, useRef } from 'react';

import { cn } from '@/lib/utils';

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

interface BottomSheetProps {
  open: boolean;
  onClose: () => void;
  /** Accessible dialog name. */
  ariaLabel: string;
  children: React.ReactNode;
  className?: string;
}

/**
 * The design's bottom sheet: dim overlay plus a parchment sheet rising from
 * the bottom edge. Full dialog semantics — focus is trapped inside, Escape
 * and backdrop click close, and focus returns to the opener. The entrance
 * animations are disabled under prefers-reduced-motion.
 */
function BottomSheet({
  open,
  onClose,
  ariaLabel,
  children,
  className,
}: BottomSheetProps) {
  const sheetRef = useRef<HTMLDivElement>(null);

  useEffect(
    function trapFocusWhileOpen() {
      if (!open) {
        return;
      }
      const sheet = sheetRef.current;
      if (!sheet) {
        return;
      }

      const opener =
        document.activeElement instanceof HTMLElement
          ? document.activeElement
          : null;
      const firstFocusable =
        sheet.querySelector<HTMLElement>(FOCUSABLE_SELECTOR);
      (firstFocusable ?? sheet).focus();

      function handleKeyDown(event: KeyboardEvent) {
        if (event.key === 'Escape') {
          event.stopPropagation();
          onClose();
          return;
        }
        if (event.key !== 'Tab' || !sheet) {
          return;
        }
        const focusable = Array.from(
          sheet.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
        );
        if (focusable.length === 0) {
          event.preventDefault();
          sheet.focus();
          return;
        }
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        const active = document.activeElement;
        if (event.shiftKey && (active === first || active === sheet)) {
          event.preventDefault();
          last.focus();
        } else if (!event.shiftKey && active === last) {
          event.preventDefault();
          first.focus();
        }
      }

      document.addEventListener('keydown', handleKeyDown);
      return () => {
        document.removeEventListener('keydown', handleKeyDown);
        opener?.focus();
      };
    },
    [open, onClose],
  );

  if (!open) {
    return null;
  }

  return (
    <>
      {/* Backdrop click is a convenience dismissal; Escape and the sheet's own controls cover keyboard users. */}
      {/* biome-ignore lint/a11y/noStaticElementInteractions: backdrop dismissal pattern */}
      {/* biome-ignore lint/a11y/useKeyWithClickEvents: Escape already closes the dialog */}
      <div
        data-testid="bottom-sheet-backdrop"
        onClick={onClose}
        className="fixed inset-0 z-60 animate-fanti-fade bg-[rgba(31,23,16,0.4)]"
      />
      <div className="fixed inset-x-0 bottom-0 z-61 animate-fanti-sheet-up">
        <div
          ref={sheetRef}
          role="dialog"
          aria-modal="true"
          aria-label={ariaLabel}
          tabIndex={-1}
          className={cn(
            'mx-auto max-h-[calc(100dvh-env(safe-area-inset-top))] max-w-[560px] overflow-y-auto overscroll-contain rounded-t-[18px] bg-popover px-5 pt-2.5 pb-[calc(24px+env(safe-area-inset-bottom))] text-popover-foreground shadow-[var(--shadow-lg),var(--ring-hairline)] outline-none',
            className,
          )}
        >
          <div
            aria-hidden="true"
            className="mx-auto mb-4 h-1 w-9 rounded-full bg-muted"
          />
          {children}
        </div>
      </div>
    </>
  );
}

export { BottomSheet };
