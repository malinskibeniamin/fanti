import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * Pill toggle used across the design — filters, font pickers, nav pills.
 * Selected chips invert to ink-on-parchment (foreground/background swap).
 */
const chipVariants = cva(
  'inline-flex min-h-9 cursor-pointer items-center justify-center gap-1 whitespace-nowrap rounded-full border-none px-3.5 py-2 font-ui text-sm tracking-[0.02em] outline-none transition-colors duration-(--duration-fast) ease-(--ease-standard) focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-45',
  {
    variants: {
      selected: {
        true: 'bg-foreground font-semibold text-background',
        false:
          'bg-muted font-normal text-muted-foreground hover:text-foreground',
      },
    },
    defaultVariants: {
      selected: false,
    },
  },
);

interface ChipProps
  extends React.ComponentProps<typeof Button>,
    VariantProps<typeof chipVariants> {}

function Chip({
  className,
  selected = false,
  role,
  type = 'button',
  ...props
}: ChipProps) {
  const isTab = role === 'tab';

  return (
    <Button
      variant="unstyled"
      size="unstyled"
      role={role}
      type={type}
      aria-pressed={isTab ? undefined : selected === true}
      aria-selected={isTab ? selected === true : undefined}
      className={cn(chipVariants({ selected }), className)}
      {...props}
    />
  );
}

export { Chip, chipVariants };
