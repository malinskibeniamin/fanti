interface StrokeFrameProps {
  children: React.ReactNode;
  sizePx: number;
}

/** Responsive square shared by animated and freehand stroke practice. */
function StrokeFrame({ children, sizePx }: StrokeFrameProps) {
  return (
    <div
      className="relative aspect-square w-full rounded-lg bg-reading-background shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--primary)_35%,transparent)]"
      style={{ maxWidth: sizePx }}
    >
      <div
        aria-hidden="true"
        className="absolute inset-y-2.5 left-1/2 border-[color-mix(in_srgb,var(--primary)_22%,transparent)] border-l border-dashed"
      />
      <div
        aria-hidden="true"
        className="absolute inset-x-2.5 top-1/2 border-[color-mix(in_srgb,var(--primary)_22%,transparent)] border-t border-dashed"
      />
      {children}
    </div>
  );
}

export { StrokeFrame };
