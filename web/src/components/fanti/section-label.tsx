import { cn } from '@/lib/utils';

interface SectionLabelProps {
  children: React.ReactNode;
  /** Secondary-script gloss rendered after the label. */
  gloss?: string;
  className?: string;
}

/** 10px uppercase tracked label — the archival section marker. */
function SectionLabel({ children, gloss, className }: SectionLabelProps) {
  return (
    <div
      className={cn(
        'font-semibold text-[10px] text-muted-foreground uppercase tracking-[0.18em]',
        className,
      )}
    >
      {children}
      {gloss ? <span className="ml-1.5 opacity-80">· {gloss}</span> : null}
    </div>
  );
}

export { SectionLabel };
