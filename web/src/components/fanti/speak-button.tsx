import { Volume2 } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

interface SpeakButtonProps {
  /** Chinese text to pronounce. */
  text: string;
  className?: string;
  /** Overrides the icon size utility, e.g. "size-3". */
  iconClassName?: string;
}

/**
 * Round pronounce button used across Discover, guides, and the character
 * page. Rendered at least 40px square for a comfortable touch target even
 * when the visual circle is smaller in the design.
 */
function SpeakButton({ text, className, iconClassName }: SpeakButtonProps) {
  return (
    <Button
      variant="unstyled"
      size="unstyled"
      type="button"
      aria-label={`Pronounce ${text}`}
      onClick={() => speak(text)}
      // allow: visual-design — the design's pronounce control is a circle
      className={cn(
        'flex size-10 flex-none cursor-pointer items-center justify-center rounded-full border-none bg-muted text-foreground outline-none transition-colors duration-(--duration-fast) hover:bg-secondary hover:text-secondary-foreground focus-visible:ring-3 focus-visible:ring-ring/50',
        className,
      )}
    >
      <Volume2 aria-hidden="true" className={cn('size-3.5', iconClassName)} />
    </Button>
  );
}

export { SpeakButton };
