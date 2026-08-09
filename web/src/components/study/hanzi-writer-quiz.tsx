import { Code } from '@connectrpc/connect';
import { useQuery } from '@connectrpc/connect-query';
import HanziWriter from 'hanzi-writer';
import type { ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/fanti/button';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import { strokeCopy } from '@/components/study/stroke-copy';
import {
  parseDrawnPath,
  parseStrokeData,
} from '@/components/study/stroke-data';
import { StrokeFrame } from '@/components/study/stroke-frame';
import type { StrokePoint } from '@/components/study/stroke-pad';
import { DictionaryService } from '@/gen/fanti/v1/dictionary_pb';
import { type Locale, useLocale } from '@/i18n/locale';

interface HanziWriterQuizProps {
  glyph: string;
  sizePx: number;
  onComplete: (strokes: StrokePoint[][]) => void;
  fallbackAction?: ReactNode;
  showOutline?: boolean;
  onMistake?: () => void;
}

function practiceStatus(
  locale: Locale,
  completed: number,
  total: number,
  mistakeStroke?: number,
) {
  if (locale === 'en') {
    if (mistakeStroke !== undefined) {
      return `Try stroke ${mistakeStroke + 1} again`;
    }
    return completed > 0
      ? `Stroke ${completed} of ${total} complete`
      : `Draw stroke 1 of ${total}`;
  }
  const traditional = locale === 'tc';
  const unit = traditional ? '筆' : '笔';
  if (mistakeStroke !== undefined) {
    return `${traditional ? '請重試' : '请重试'}第 ${mistakeStroke + 1} ${unit}`;
  }
  return completed > 0
    ? `完成第 ${completed} ${unit}，共 ${total} ${unit}`
    : `${traditional ? '請寫' : '请写'}第 1 ${unit}，共 ${total} ${unit}`;
}

function HanziWriterQuiz({
  glyph,
  sizePx,
  onComplete,
  fallbackAction,
  showOutline = true,
  onMistake,
}: HanziWriterQuizProps) {
  const { locale, t } = useLocale();
  const writerRef = useRef<HanziWriter>(null);
  const drawnStrokesRef = useRef<StrokePoint[][]>([]);
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;
  const onMistakeRef = useRef(onMistake);
  onMistakeRef.current = onMistake;
  const [target, setTarget] = useState<HTMLDivElement | null>(null);
  const [writerReady, setWriterReady] = useState(false);
  const [writerVersion, setWriterVersion] = useState(0);
  const [currentStroke, setCurrentStroke] = useState(0);
  const [mistakeStroke, setMistakeStroke] = useState<number>();
  const [renderFailed, setRenderFailed] = useState(false);
  const [hintPending, setHintPending] = useState(false);
  const strokeQuery = useQuery(DictionaryService.method.getStrokeData, {
    name: `characters/${glyph}`,
  });
  const strokeDataJson = strokeQuery.data?.data;
  const strokeData = strokeDataJson
    ? parseStrokeData(strokeDataJson)
    : undefined;

  useEffect(
    function mountWriter() {
      const writerStrokeData = strokeDataJson
        ? parseStrokeData(strokeDataJson)
        : undefined;
      if (!target || !writerStrokeData?.valid) {
        return;
      }

      drawnStrokesRef.current = [];
      setCurrentStroke(0);
      setMistakeStroke(undefined);
      let active = true;
      const styles = getComputedStyle(target);
      target.replaceChildren();
      const writer = HanziWriter.create(target, glyph, {
        width: sizePx,
        height: sizePx,
        padding: 20,
        showCharacter: false,
        showOutline,
        strokeColor:
          styles.getPropertyValue('--reading-foreground').trim() || '#18120c',
        outlineColor: styles.getPropertyValue('--border').trim() || '#c9b88c',
        drawingColor:
          styles.getPropertyValue('--reading-foreground').trim() || '#18120c',
        highlightColor: styles.getPropertyValue('--accent').trim() || '#237a64',
        charDataLoader: () => writerStrokeData.data,
        onLoadCharDataSuccess: () => {
          if (active) {
            setWriterReady(true);
            setWriterVersion((version) => version + 1);
          }
        },
        onLoadCharDataError: () => {
          if (active) {
            setWriterReady(false);
            setRenderFailed(true);
          }
        },
      });
      writerRef.current = writer;

      return () => {
        active = false;
        writer.cancelQuiz();
        if (writerRef.current === writer) {
          writerRef.current = null;
        }
      };
    },
    [glyph, showOutline, sizePx, strokeDataJson, target],
  );

  useEffect(
    function startQuiz() {
      const quizStrokeData = strokeDataJson
        ? parseStrokeData(strokeDataJson)
        : undefined;
      if (
        writerVersion === 0 ||
        !writerReady ||
        !writerRef.current ||
        !quizStrokeData?.valid
      ) {
        return;
      }
      void writerRef.current
        .quiz({
          highlightOnComplete: false,
          leniency: 0.5,
          markStrokeCorrectAfterMisses: false,
          showHintAfterMisses: false,
          onCorrectStroke: ({ drawnPath, strokeNum }) => {
            const points = parseDrawnPath(drawnPath.pathString);
            if (!points) {
              setRenderFailed(true);
              return;
            }
            drawnStrokesRef.current[strokeNum] = points;
            setCurrentStroke(strokeNum + 1);
            setMistakeStroke(undefined);
          },
          onMistake: ({ strokeNum }) => {
            setMistakeStroke(strokeNum);
            onMistakeRef.current?.();
          },
          onComplete: () => {
            const hasEveryStroke = quizStrokeData.data.strokes.every(
              (_stroke, index) => drawnStrokesRef.current[index] !== undefined,
            );
            if (hasEveryStroke) {
              onCompleteRef.current(drawnStrokesRef.current);
            } else {
              setRenderFailed(true);
            }
          },
        })
        .catch(() => setRenderFailed(true));
    },
    [strokeDataJson, writerReady, writerVersion],
  );

  function showHint() {
    const writer = writerRef.current;
    if (!writer || hintPending) {
      return;
    }
    setHintPending(true);

    async function highlightNextStroke(activeWriter: HanziWriter) {
      try {
        await activeWriter.highlightStroke(currentStroke);
      } catch {
        setRenderFailed(true);
      } finally {
        setHintPending(false);
      }
    }

    void highlightNextStroke(writer);
  }

  if (strokeQuery.isPending) {
    return (
      <div role="status" aria-label={strokeCopy(locale, 'loading')}>
        <StrokeFrame sizePx={sizePx}>
          <Skeleton className="absolute inset-4 rounded-md" />
        </StrokeFrame>
      </div>
    );
  }

  if (strokeQuery.isError && strokeQuery.error.code === Code.NotFound) {
    return (
      <div className="flex w-full flex-col items-center gap-3">
        <StrokeFrame sizePx={sizePx}>
          <span
            role="img"
            aria-label={`${t('qWrite')} ${glyph}`}
            className="pointer-events-none absolute inset-0 flex items-center justify-center font-display text-reading-foreground leading-none"
            style={{ fontSize: sizePx * 0.73 }}
          >
            {glyph}
          </span>
        </StrokeFrame>
        <p className="text-center text-muted-foreground text-sm">
          {strokeCopy(locale, 'practiceUnavailable')}
        </p>
        {fallbackAction}
      </div>
    );
  }

  if (strokeQuery.isError) {
    return (
      <ErrorState
        title={strokeCopy(locale, 'practiceLoadError')}
        description={strokeQuery.error.rawMessage}
        onRetry={() => {
          void strokeQuery.refetch();
        }}
        className="w-full"
      />
    );
  }

  if (strokeQuery.data && !strokeData?.valid) {
    return (
      <ErrorState
        title={strokeCopy(locale, 'practiceLoadError')}
        description={strokeCopy(locale, 'invalid')}
        onRetry={() => {
          void strokeQuery.refetch();
        }}
        className="w-full"
      />
    );
  }

  if (renderFailed) {
    return (
      <ErrorState
        title={strokeCopy(locale, 'practiceStopped')}
        onRetry={() => {
          setWriterReady(false);
          setRenderFailed(false);
        }}
        className="w-full"
      />
    );
  }

  return (
    <div className="flex w-full flex-col items-center gap-3">
      <StrokeFrame sizePx={sizePx}>
        <div
          ref={setTarget}
          role="img"
          aria-label={`${t('qWrite')} ${glyph}`}
          className="absolute inset-0 h-full w-full [&>svg]:h-full [&>svg]:w-full"
        />
      </StrokeFrame>
      <p
        role="status"
        aria-live="polite"
        className="text-muted-foreground text-sm tabular-nums"
      >
        {practiceStatus(
          locale,
          currentStroke,
          strokeData?.valid ? strokeData.data.strokes.length : 0,
          mistakeStroke,
        )}
      </p>
      {showOutline ? (
        <Button
          variant="outline"
          onClick={showHint}
          disabled={
            !writerReady ||
            !strokeData?.valid ||
            hintPending ||
            currentStroke >= strokeData.data.strokes.length
          }
        >
          {strokeCopy(locale, 'hintNext')}
        </Button>
      ) : null}
    </div>
  );
}

export { HanziWriterQuiz };
export default HanziWriterQuiz;
