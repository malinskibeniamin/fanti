import type { Button as ButtonPrimitive } from '@base-ui/react/button';
import type { VariantProps } from 'class-variance-authority';

import {
  type buttonVariants,
  Button as UiButton,
} from '@/components/ui/button';
import { cn } from '@/lib/utils';

type UiVariant = VariantProps<typeof buttonVariants>['variant'];
type UiSize = VariantProps<typeof buttonVariants>['size'];

/**
 * Design-system variant names: default = lacquer, secondary = gold,
 * accent = jade, plus outline and ghost.
 */
export type FantiButtonVariant =
  | 'default'
  | 'secondary'
  | 'accent'
  | 'outline'
  | 'ghost';

export type FantiButtonSize = 'sm' | 'default' | 'lg' | 'icon';

const variantClasses: Record<FantiButtonVariant, string> = {
  default: '',
  secondary: '',
  accent:
    'bg-accent text-accent-foreground hover:bg-[color-mix(in_srgb,var(--accent),var(--foreground)_8%)]',
  outline: '',
  ghost: '',
};

const uiVariant: Record<FantiButtonVariant, UiVariant> = {
  default: 'default',
  secondary: 'secondary',
  accent: 'default',
  outline: 'outline',
  ghost: 'ghost',
};

/* The design's touch-target floor: lg buttons hit --tap-min (44px). */
const sizeClasses: Record<FantiButtonSize, string> = {
  sm: '',
  default: 'min-h-10 rounded-lg px-4',
  lg: 'min-h-11 rounded-lg px-5 text-sm',
  icon: 'size-10 rounded-lg',
};

const uiSize: Record<FantiButtonSize, UiSize> = {
  sm: 'sm',
  default: 'default',
  lg: 'lg',
  icon: 'icon',
};

interface FantiButtonProps extends ButtonPrimitive.Props {
  variant?: FantiButtonVariant;
  size?: FantiButtonSize;
}

function Button({
  className,
  variant = 'default',
  size = 'default',
  nativeButton,
  ...props
}: FantiButtonProps) {
  return (
    <UiButton
      variant={uiVariant[variant]}
      size={uiSize[size]}
      className={cn(variantClasses[variant], sizeClasses[size], className)}
      // A render prop swaps in a non-button element (router Link), which
      // Base UI must be told about or it logs a semantics error.
      nativeButton={nativeButton ?? props.render === undefined}
      {...props}
    />
  );
}

export { Button };
