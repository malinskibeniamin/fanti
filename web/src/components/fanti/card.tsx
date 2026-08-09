import { cn } from '@/lib/utils';

interface CardProps extends React.ComponentProps<'div'> {}

/** Archive panel: warm card surface, 14px radius, hairline ring instead of border. */
function Card({ className, ...props }: CardProps) {
  return (
    <div
      className={cn(
        'rounded-xl bg-card p-4 text-card-foreground shadow-hairline',
        className,
      )}
      {...props}
    />
  );
}

export { Card };
