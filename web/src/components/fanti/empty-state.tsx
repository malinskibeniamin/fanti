import { Card } from '@/components/fanti/card';
import { cn } from '@/lib/utils';

interface EmptyStateProps {
  title: string;
  description?: string;
  /** Optional action, usually a fanti Button. */
  action?: React.ReactNode;
  /** Decorative glyph or icon shown above the title. */
  glyph?: React.ReactNode;
  className?: string;
}

function EmptyState({
  title,
  description,
  action,
  glyph,
  className,
}: EmptyStateProps) {
  return (
    <Card
      className={cn(
        'flex flex-col items-center gap-2 px-6 py-10 text-center',
        className,
      )}
    >
      {glyph ? (
        <div
          aria-hidden="true"
          className="font-display text-4xl text-muted-foreground"
        >
          {glyph}
        </div>
      ) : null}
      <p className="font-medium text-base">{title}</p>
      {description ? (
        <p className="max-w-sm text-muted-foreground text-sm leading-normal">
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </Card>
  );
}

export { EmptyState };
