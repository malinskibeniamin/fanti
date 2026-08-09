import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { useLocale } from '@/i18n/locale';
import { cn } from '@/lib/utils';

interface ErrorStateProps {
  title: string;
  description?: string;
  /** Retry handler — renders the retry button when provided. */
  onRetry?: () => void;
  retryLabel?: string;
  className?: string;
}

function ErrorState({
  title,
  description,
  onRetry,
  retryLabel,
  className,
}: ErrorStateProps) {
  const { t } = useLocale();
  return (
    <Card
      role="alert"
      className={cn(
        'flex flex-col items-center gap-2 px-6 py-10 text-center',
        className,
      )}
    >
      <p className="font-medium text-base text-status-manual">{title}</p>
      {description ? (
        <p className="max-w-sm text-muted-foreground text-sm leading-normal">
          {description}
        </p>
      ) : null}
      {onRetry ? (
        <Button variant="outline" className="mt-2" onClick={onRetry}>
          {retryLabel ?? t('retryQuizL')}
        </Button>
      ) : null}
    </Card>
  );
}

export { ErrorState };
