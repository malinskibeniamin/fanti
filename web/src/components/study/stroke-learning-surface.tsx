import { lazy, Suspense, useState } from 'react';

import { Chip } from '@/components/fanti/chip';
import { Skeleton } from '@/components/fanti/skeleton';
import { strokeCopy } from '@/components/study/stroke-copy';
import { StrokePad } from '@/components/study/stroke-pad';
import { useLocale } from '@/i18n/locale';
import { useThemeStore } from '@/stores/theme';

const StrokeAnimator = lazy(() => import('@/components/study/stroke-animator'));

interface StrokeLearningSurfaceProps {
  glyph: string;
  sizePx: number;
  practiceAriaLabel: string;
  expectedStrokeCount?: number;
}

type LearningMode = 'practice' | 'watch';

function StrokeLearningSurface({
  glyph,
  sizePx,
  practiceAriaLabel,
  expectedStrokeCount,
}: StrokeLearningSurfaceProps) {
  const { locale } = useLocale();
  const theme = useThemeStore((state) => state.theme);
  const [mode, setMode] = useState<LearningMode>('watch');

  return (
    <div className="flex w-full flex-col items-center gap-3">
      <fieldset className="flex gap-1 rounded-full bg-muted p-0.75">
        <legend className="sr-only">{strokeCopy(locale, 'mode')}</legend>
        <Chip selected={mode === 'watch'} onClick={() => setMode('watch')}>
          {strokeCopy(locale, 'watch')}
        </Chip>
        <Chip
          selected={mode === 'practice'}
          onClick={() => setMode('practice')}
        >
          {strokeCopy(locale, 'practice')}
        </Chip>
      </fieldset>

      {mode === 'watch' ? (
        <Suspense
          fallback={
            <div role="status" aria-label={strokeCopy(locale, 'loading')}>
              <Skeleton
                className="aspect-square w-full rounded-lg"
                style={{ maxWidth: sizePx }}
              />
            </div>
          }
        >
          <StrokeAnimator
            key={`${glyph}-${theme}`}
            glyph={glyph}
            sizePx={sizePx}
          />
        </Suspense>
      ) : (
        <StrokePad
          key={glyph}
          sizePx={sizePx}
          ariaLabel={practiceAriaLabel}
          ghostGlyph={glyph}
          expectedStrokeCount={expectedStrokeCount}
        />
      )}
    </div>
  );
}

export { StrokeLearningSurface };
