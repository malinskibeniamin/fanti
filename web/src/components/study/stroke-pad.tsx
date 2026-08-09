import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/fanti/button';
import { StrokeFrame } from '@/components/study/stroke-frame';
import { useLocale } from '@/i18n/locale';
import { cn } from '@/lib/utils';

export interface StrokePoint {
  x: number;
  y: number;
}

const MIN_LINE_WIDTH = 4;
const LINE_WIDTH_RATIO = 26;

interface StrokePadProps {
  /** Canvas resolution and rendered square edge, in pixels. */
  sizePx: number;
  ariaLabel: string;
  /** Model glyph traced under the strokes. */
  ghostGlyph?: string;
  /** Hide the ghost for write-from-memory drills until revealed. */
  ghostVisible?: boolean;
  /** Known target count, shown beside the learner's current stroke count. */
  expectedStrokeCount?: number;
  /** Fires with the full stroke list after every draw, undo, or clear. */
  onStrokesChange?: (strokes: StrokePoint[][]) => void;
  className?: string;
}

/**
 * The design's 田-grid practice pad: a canvas the learner draws on with
 * pointer strokes, a faint model glyph underneath, and undo / clear controls
 * with a live stroke counter. Remount (change `key`) to reset for a new glyph.
 */
function StrokePad({
  sizePx,
  ariaLabel,
  ghostGlyph,
  ghostVisible = true,
  expectedStrokeCount,
  onStrokesChange,
  className,
}: StrokePadProps) {
  const { t, locale } = useLocale();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const drawingRef = useRef(false);
  const [strokes, setStrokes] = useState<StrokePoint[][]>([]);

  const onStrokesChangeRef = useRef(onStrokesChange);
  onStrokesChangeRef.current = onStrokesChange;
  useEffect(
    function notifyStrokesChanged() {
      onStrokesChangeRef.current?.(strokes);
    },
    [strokes],
  );

  useEffect(
    function redrawStrokes() {
      const canvas = canvasRef.current;
      // jsdom has no 2d context; drawing is a no-op there.
      const context = canvas?.getContext('2d');
      if (!canvas || !context) {
        return;
      }
      context.clearRect(0, 0, sizePx, sizePx);
      context.lineWidth = Math.max(MIN_LINE_WIDTH, sizePx / LINE_WIDTH_RATIO);
      context.lineCap = 'round';
      context.lineJoin = 'round';
      // Ink color comes from the element's CSS color (reading foreground).
      const inkColor = getComputedStyle(canvas).color;
      context.strokeStyle = inkColor;
      context.fillStyle = inkColor;
      for (const stroke of strokes) {
        const [first] = stroke;
        if (!first) {
          continue;
        }
        if (stroke.length === 1) {
          context.beginPath();
          context.arc(first.x, first.y, context.lineWidth / 2, 0, Math.PI * 2);
          context.fill();
          continue;
        }
        context.beginPath();
        context.moveTo(first.x, first.y);
        for (const point of stroke.slice(1)) {
          context.lineTo(point.x, point.y);
        }
        context.stroke();
      }
    },
    [strokes, sizePx],
  );

  function canvasPoint(
    event: React.PointerEvent<HTMLCanvasElement>,
  ): StrokePoint {
    const rect = event.currentTarget.getBoundingClientRect();
    const scaleX = rect.width > 0 ? sizePx / rect.width : 1;
    const scaleY = rect.height > 0 ? sizePx / rect.height : 1;
    return {
      x: (event.clientX - rect.left) * scaleX,
      y: (event.clientY - rect.top) * scaleY,
    };
  }

  function handlePointerDown(event: React.PointerEvent<HTMLCanvasElement>) {
    // jsdom lacks pointer capture; guard so tests can drive the pad.
    if (typeof event.currentTarget.setPointerCapture === 'function') {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
    drawingRef.current = true;
    const point = canvasPoint(event);
    setStrokes((previous) => [...previous, [point]]);
  }

  function handlePointerMove(event: React.PointerEvent<HTMLCanvasElement>) {
    if (!drawingRef.current) {
      return;
    }
    const point = canvasPoint(event);
    setStrokes((previous) => {
      const last = previous[previous.length - 1];
      if (!last) {
        return previous;
      }
      const next = [...previous];
      next[next.length - 1] = [...last, point];
      return next;
    });
  }

  function endStroke() {
    drawingRef.current = false;
  }

  const strokeCountBase =
    locale === 'en'
      ? `Strokes ${strokes.length}`
      : locale === 'tc'
        ? `筆畫 ${strokes.length}`
        : `笔画 ${strokes.length}`;
  const strokeCountLabel =
    expectedStrokeCount && expectedStrokeCount > 0
      ? `${strokeCountBase} / ${expectedStrokeCount}`
      : strokeCountBase;

  return (
    <div className={cn('flex w-full flex-col items-center gap-3', className)}>
      {/* The pad edge is a runtime prop (260 quiz / 300 practice), so it must be an inline style. */}
      <StrokeFrame sizePx={sizePx}>
        {ghostGlyph && ghostVisible ? (
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 flex select-none items-center justify-center font-display text-[color-mix(in_srgb,var(--reading-foreground)_13%,transparent)] leading-none"
            // The ghost scales with the runtime pad size, so it must be an inline style.
            style={{ fontSize: sizePx * 0.73 }}
          >
            {ghostGlyph}
          </div>
        ) : null}
        <canvas
          ref={canvasRef}
          role="img"
          aria-label={ariaLabel}
          data-testid="stroke-pad-canvas"
          width={sizePx}
          height={sizePx}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={endStroke}
          onPointerLeave={endStroke}
          className="absolute inset-0 h-full w-full cursor-crosshair touch-none text-reading-foreground"
        />
      </StrokeFrame>

      <div className="flex items-center gap-3">
        <span className="text-muted-foreground text-sm tabular-nums">
          {strokeCountLabel}
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setStrokes((previous) => previous.slice(0, -1))}
        >
          {t('undo')}
        </Button>
        <Button variant="outline" size="sm" onClick={() => setStrokes([])}>
          {t('clear')}
        </Button>
      </div>
    </div>
  );
}

export { StrokePad };
