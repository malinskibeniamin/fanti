import { Code } from '@connectrpc/connect';
import { useQuery } from '@connectrpc/connect-query';
import HanziWriter from 'hanzi-writer';
import type { ReactNode } from 'react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/fanti/button';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import { Pause, Play, RotateCcw, StepForward } from '@/components/icons';
import { strokeCopy } from '@/components/study/stroke-copy';
import { parseStrokeData } from '@/components/study/stroke-data';
import { StrokeFrame } from '@/components/study/stroke-frame';
import { DictionaryService } from '@/gen/fanti/v1/dictionary_pb';
import { type Locale, useLocale } from '@/i18n/locale';

interface StrokeAnimatorProps {
  glyph: string;
  sizePx: number;
  onComplete?: () => void;
  fallbackAction?: ReactNode;
}

type PlaybackState = 'complete' | 'idle' | 'paused' | 'playing' | 'stepping';

function strokeStatusLabel(
  locale: Locale,
  playback: PlaybackState,
  stepIndex: number,
  total: number,
) {
  if (locale === 'en') {
    if (playback === 'playing') return 'Playing stroke order';
    if (playback === 'paused') return 'Stroke order paused';
    if (playback === 'complete') return `${total} strokes complete`;
    if (playback === 'stepping') return `Stroke ${stepIndex} of ${total}`;
    return `${total} strokes ready`;
  }
  if (locale === 'tc') {
    if (playback === 'playing') return '正在播放筆順';
    if (playback === 'paused') return '已暫停筆順';
    if (playback === 'complete') return `${total} 筆完成`;
    if (playback === 'stepping') return `第 ${stepIndex} 筆，共 ${total} 筆`;
    return `共 ${total} 筆，準備完成`;
  }
  if (playback === 'playing') return '正在播放笔顺';
  if (playback === 'paused') return '已暂停笔顺';
  if (playback === 'complete') return `${total} 笔完成`;
  if (playback === 'stepping') return `第 ${stepIndex} 笔，共 ${total} 笔`;
  return `共 ${total} 笔，准备完成`;
}

function StrokeAnimator({
  glyph,
  sizePx,
  onComplete,
  fallbackAction,
}: StrokeAnimatorProps) {
  const { locale, t } = useLocale();
  const writerRef = useRef<HanziWriter>(null);
  const [target, setTarget] = useState<HTMLDivElement | null>(null);
  const [playback, setPlayback] = useState<PlaybackState>('idle');
  const [stepIndex, setStepIndex] = useState(0);
  const [stepPending, setStepPending] = useState(false);
  const [renderFailed, setRenderFailed] = useState(false);
  const [writerReady, setWriterReady] = useState(false);
  const strokeQuery = useQuery(
    DictionaryService.method.getStrokeData,
    {
      name: `characters/${glyph}`,
    },
    {
      retry: (failureCount, error) =>
        error.code !== Code.NotFound && failureCount < 2,
    },
  );
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

      let active = true;
      let loaded = false;
      const styles = getComputedStyle(target);
      const writer = HanziWriter.create(target, glyph, {
        width: sizePx,
        height: sizePx,
        padding: 20,
        showCharacter: true,
        showOutline: true,
        strokeColor:
          styles.getPropertyValue('--reading-foreground').trim() || '#18120c',
        outlineColor: styles.getPropertyValue('--border').trim() || '#c9b88c',
        charDataLoader: () => writerStrokeData.data,
        onLoadCharDataSuccess: () => {
          loaded = true;
          if (active) {
            setWriterReady(true);
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
        if (writerRef.current === writer) {
          writerRef.current = null;
        }
        if (loaded) {
          void writer.showCharacter({ duration: 0 });
        }
      };
    },
    [glyph, sizePx, strokeDataJson, target],
  );

  function showPlaybackError() {
    setWriterReady(false);
    setRenderFailed(true);
    setPlayback('idle');
  }

  function retryRenderer() {
    setPlayback('idle');
    setStepIndex(0);
    setWriterReady(false);
    setRenderFailed(false);
  }

  function play() {
    const writer = writerRef.current;
    if (!writer) {
      return;
    }
    if (playback === 'playing') {
      setPlayback('paused');
      void writer.pauseAnimation().catch(showPlaybackError);
      return;
    }
    if (playback === 'paused') {
      setPlayback('playing');
      void writer.resumeAnimation().catch(showPlaybackError);
      return;
    }
    startFullAnimation(writer);
  }

  function startFullAnimation(writer: HanziWriter) {
    setStepIndex(0);
    setPlayback('playing');
    void writer
      .animateCharacter({
        onComplete: ({ canceled }) => {
          if (!canceled) {
            setPlayback('complete');
            setStepIndex(
              strokeData?.valid ? strokeData.data.strokes.length : 0,
            );
            onComplete?.();
          }
        },
      })
      .catch(showPlaybackError);
  }

  function replay() {
    const writer = writerRef.current;
    if (!writer) {
      return;
    }
    startFullAnimation(writer);
  }

  function stepForward() {
    const writer = writerRef.current;
    if (!writer || !writerReady || !strokeData?.valid || stepPending) {
      return;
    }
    const nextIndex = playback === 'stepping' ? stepIndex : 0;
    const totalStrokes = strokeData.data.strokes.length;
    setPlayback('stepping');
    setStepPending(true);

    async function animateNextStroke(activeWriter: HanziWriter) {
      try {
        if (nextIndex === 0) {
          await activeWriter.hideCharacter({ duration: 0 });
        }
        const result = await activeWriter.animateStroke(nextIndex);
        if (!result?.canceled) {
          setStepIndex(nextIndex + 1);
          if (nextIndex + 1 === totalStrokes) {
            onComplete?.();
          }
        }
      } catch {
        showPlaybackError();
      } finally {
        setStepPending(false);
      }
    }

    void animateNextStroke(writer);
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
            aria-label={`${t('stStrokesL')} ${glyph}`}
            className="pointer-events-none absolute inset-0 flex items-center justify-center font-display text-reading-foreground leading-none"
            style={{ fontSize: sizePx * 0.73 }}
          >
            {glyph}
          </span>
        </StrokeFrame>
        <p className="text-center text-muted-foreground text-sm">
          {strokeCopy(locale, 'unavailable')}
        </p>
        {fallbackAction}
      </div>
    );
  }

  if (strokeQuery.isError) {
    return (
      <ErrorState
        title={strokeCopy(locale, 'loadError')}
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
        title={strokeCopy(locale, 'loadError')}
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
        title={strokeCopy(locale, 'loadError')}
        onRetry={retryRenderer}
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
          aria-label={`${t('stStrokesL')} ${glyph}`}
          className="absolute inset-0 h-full w-full [&>svg]:h-full [&>svg]:w-full"
        />
      </StrokeFrame>

      <p
        role="status"
        aria-live="polite"
        className="text-muted-foreground text-sm tabular-nums"
      >
        {writerReady
          ? strokeStatusLabel(
              locale,
              playback,
              stepIndex,
              strokeData?.valid ? strokeData.data.strokes.length : 0,
            )
          : strokeCopy(locale, 'loading')}
      </p>

      <div className="flex flex-wrap justify-center gap-2">
        <Button
          onClick={play}
          disabled={!strokeData?.valid || !writerReady || renderFailed}
        >
          {playback === 'playing' ? (
            <Pause aria-hidden />
          ) : (
            <Play aria-hidden />
          )}
          {playback === 'playing'
            ? strokeCopy(locale, 'pause')
            : strokeCopy(locale, 'play')}
        </Button>
        <Button
          variant="outline"
          onClick={stepForward}
          disabled={
            !strokeData?.valid ||
            !writerReady ||
            renderFailed ||
            stepPending ||
            (playback === 'stepping' &&
              stepIndex >= strokeData.data.strokes.length)
          }
        >
          <StepForward aria-hidden />
          {strokeCopy(locale, 'next')}
        </Button>
        <Button
          variant="ghost"
          onClick={replay}
          disabled={!strokeData?.valid || !writerReady || renderFailed}
        >
          <RotateCcw aria-hidden />
          {strokeCopy(locale, 'replay')}
        </Button>
      </div>
    </div>
  );
}

export { StrokeAnimator };
export default StrokeAnimator;
