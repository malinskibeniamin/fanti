import { cn } from '@/lib/utils';

interface ProgressBarProps {
  /** Progress in the 0–1 range. */
  value: number;
  label: string;
  className?: string;
}

/** The design's hairline progress: 3px track on muted, antique-gold fill. */
function ProgressBar({ value, label, className }: ProgressBarProps) {
  const percent = Math.round(Math.min(Math.max(value, 0), 1) * 100);
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={percent}
      className={cn(
        'h-[3px] w-full overflow-hidden rounded-full bg-muted',
        className,
      )}
    >
      {/* Dynamic width must be an inline custom property; the fill animates via transform to stay off the layout path. */}
      <div
        className="h-full w-full origin-left rounded-full bg-secondary transition-transform duration-(--duration-base) ease-(--ease-standard)"
        style={{ transform: `scaleX(${percent / 100})` }}
      />
    </div>
  );
}

export { ProgressBar };
