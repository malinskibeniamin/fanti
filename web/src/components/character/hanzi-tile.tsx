import { cn } from '@/lib/utils';

interface HanziTileProps {
  glyph: string;
  /** Tile edge in pixels. */
  size: number;
  /** Glyph font size in pixels. */
  fontSize: number;
  className?: string;
}

/**
 * 田-grid glyph tile: parchment reading surface, hairline primary inset ring,
 * dashed centre guides, and a display-font glyph on top.
 */
function HanziTile({ glyph, size, fontSize, className }: HanziTileProps) {
  return (
    <div
      className={cn(
        'relative flex flex-none items-center justify-center rounded-lg bg-reading-background shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--primary)_28%,transparent)]',
        className,
      )}
      style={{ width: size, height: size }}
    >
      <div
        aria-hidden="true"
        className="absolute inset-y-2 left-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-l border-dashed"
      />
      <div
        aria-hidden="true"
        className="absolute inset-x-2 top-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-t border-dashed"
      />
      <span
        className="relative font-display text-reading-foreground"
        style={{ fontSize, lineHeight: 1 }}
      >
        {glyph}
      </span>
    </div>
  );
}

export { HanziTile };
