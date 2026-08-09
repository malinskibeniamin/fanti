import { create } from '@bufbuild/protobuf';
import { FieldMaskSchema } from '@bufbuild/protobuf/wkt';
import { useMutation, useQuery } from '@connectrpc/connect-query';
import { lazy, Suspense, useRef, useState } from 'react';

import { Button } from '@/components/fanti/button';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import { strokeCopy } from '@/components/study/stroke-copy';
import { StrokePad, type StrokePoint } from '@/components/study/stroke-pad';
import { pickLocalized } from '@/components/study/study-content';
import {
  VisualMnemonic,
  visualMnemonicFor,
} from '@/components/study/visual-mnemonic';
import { PracticeDifficulty } from '@/gen/fanti/v1/common_pb';
import {
  ReviewSchema,
  StudyService,
  UpdateReviewRequestSchema,
} from '@/gen/fanti/v1/study_pb';
import type { GradeHandwritingResponse } from '@/gen/fanti/v1/tutor_pb';
import { TutorService } from '@/gen/fanti/v1/tutor_pb';
import { useLocale } from '@/i18n/locale';

const StrokeAnimator = lazy(() => import('@/components/study/stroke-animator'));
const HanziWriterQuiz = lazy(
  () => import('@/components/study/hanzi-writer-quiz'),
);

interface StrokePracticeQuizProps {
  glyph: string;
  sizePx: number;
  onResult: (correct: boolean) => void;
}

function StrokePracticeQuiz({
  glyph,
  sizePx,
  onResult,
}: StrokePracticeQuizProps) {
  const { locale, t } = useLocale();
  const [practicing, setPracticing] = useState(false);
  const [watched, setWatched] = useState(false);
  const [fallback, setFallback] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const [handwriting, setHandwriting] = useState<GradeHandwritingResponse>();
  const [completedStrokes, setCompletedStrokes] = useState<StrokePoint[][]>();
  const [gradeError, setGradeError] = useState<string>();
  const [selectedDifficulty, setSelectedDifficulty] =
    useState<PracticeDifficulty>();
  const [preferenceError, setPreferenceError] = useState<string>();
  const [hintUsed, setHintUsed] = useState(false);
  const [visualHintVisible, setVisualHintVisible] = useState(false);
  const [attemptDifficulty, setAttemptDifficulty] =
    useState<PracticeDifficulty>();
  const mistakeCount = useRef(0);
  const reviewQuery = useQuery(StudyService.method.getReview, {
    name: `reviews/${glyph}`,
  });
  const savedDifficulty =
    reviewQuery.data?.practiceDifficulty ?? PracticeDifficulty.GUIDED;
  const difficulty = selectedDifficulty ?? savedDifficulty;
  const updateReviewMutation = useMutation(StudyService.method.updateReview, {
    onSuccess: (review) => {
      setPreferenceError(undefined);
      setSelectedDifficulty(review.practiceDifficulty);
      void reviewQuery.refetch();
      setPracticing(review.practiceDifficulty !== PracticeDifficulty.GUIDED);
      setWatched(false);
      setFallback(false);
      setRevealed(false);
      setHandwriting(undefined);
      setCompletedStrokes(undefined);
      setGradeError(undefined);
      setAttemptDifficulty(undefined);
      setHintUsed(false);
      setVisualHintVisible(false);
      mistakeCount.current = 0;
    },
    onError: (error) => setPreferenceError(error.rawMessage),
  });
  const gradeHandwritingMutation = useMutation(
    TutorService.method.gradeHandwriting,
    {
      onSuccess: (response) => {
        setGradeError(undefined);
        setHandwriting(response);
      },
      onError: (error) => setGradeError(error.rawMessage),
    },
  );

  if (reviewQuery.isPending) {
    return (
      <div
        role="status"
        aria-label={strokeCopy(locale, 'difficultyLoading')}
        className="flex w-full flex-col items-center gap-3"
      >
        <Skeleton className="h-10 w-64 rounded-lg" />
        <Skeleton
          className="aspect-square w-full rounded-lg"
          style={{ maxWidth: sizePx }}
        />
      </div>
    );
  }

  if (reviewQuery.isError && !reviewQuery.data) {
    return (
      <ErrorState
        title={strokeCopy(locale, 'difficulty')}
        description={reviewQuery.error.rawMessage}
        onRetry={() => {
          void reviewQuery.refetch();
        }}
        className="w-full"
      />
    );
  }

  function submitGrade(strokes: StrokePoint[][]) {
    if (gradeHandwritingMutation.isPending) {
      return;
    }
    gradeHandwritingMutation.mutate({
      character: glyph,
      strokes: strokes.map((points) => ({ points })),
      canvasWidth: sizePx,
      canvasHeight: sizePx,
      practiceDifficulty: difficulty,
      hintUsed,
    });
  }

  function grade(strokes: StrokePoint[][]) {
    setCompletedStrokes(strokes);
    setGradeError(undefined);
    setHandwriting(undefined);
    submitGrade(strokes);
  }

  function selectDifficulty(nextDifficulty: PracticeDifficulty) {
    if (nextDifficulty === difficulty || updateReviewMutation.isPending) {
      return;
    }
    updateReviewMutation.mutate(
      create(UpdateReviewRequestSchema, {
        review: create(ReviewSchema, {
          name: `reviews/${glyph}`,
          practiceDifficulty: nextDifficulty,
        }),
        updateMask: create(FieldMaskSchema, {
          paths: ['practice_difficulty'],
        }),
      }),
    );
  }

  const practiceActive = practicing || difficulty !== PracticeDifficulty.GUIDED;
  const effectiveDifficulty = attemptDifficulty ?? difficulty;
  const hasVisualMnemonic = visualMnemonicFor(glyph) !== undefined;
  const difficultyOptions = [
    [PracticeDifficulty.GUIDED, strokeCopy(locale, 'difficultyGuided')],
    [PracticeDifficulty.RECALL, strokeCopy(locale, 'difficultyRecall')],
    [PracticeDifficulty.MASTERY, strokeCopy(locale, 'difficultyMastery')],
  ] as const;

  function revealVisualHint() {
    setHintUsed(true);
    setVisualHintVisible(true);
    if (difficulty === PracticeDifficulty.MASTERY) {
      setAttemptDifficulty(PracticeDifficulty.RECALL);
    }
  }

  function recordMistake() {
    mistakeCount.current += 1;
    if (
      difficulty === PracticeDifficulty.RECALL &&
      mistakeCount.current >= 2 &&
      hasVisualMnemonic
    ) {
      setHintUsed(true);
      setVisualHintVisible(true);
    }
  }

  return (
    <div className="flex w-full flex-col items-center gap-3">
      <div
        role="radiogroup"
        aria-label={strokeCopy(locale, 'difficulty')}
        className="flex rounded-lg bg-muted p-1"
      >
        {difficultyOptions.map(([value, label]) => (
          <Button
            key={value}
            role="radio"
            aria-checked={difficulty === value}
            variant={difficulty === value ? 'accent' : 'ghost'}
            disabled={
              reviewQuery.isPending ||
              reviewQuery.isError ||
              updateReviewMutation.isPending ||
              gradeHandwritingMutation.isPending
            }
            onClick={() => selectDifficulty(value)}
            className="min-h-8 px-3 py-1 text-xs"
          >
            {label}
          </Button>
        ))}
      </div>
      {preferenceError ? (
        <ErrorState
          title={strokeCopy(locale, 'difficulty')}
          description={preferenceError}
          onRetry={() => {
            setPreferenceError(undefined);
            void reviewQuery.refetch();
          }}
          className="w-full"
        />
      ) : null}
      <p className="max-w-[420px] text-center text-muted-foreground text-sm">
        {strokeCopy(
          locale,
          practiceActive || fallback ? 'drawFromMemory' : 'watchOnce',
        )}
      </p>
      {hasVisualMnemonic &&
      ((difficulty === PracticeDifficulty.GUIDED && !practiceActive) ||
        visualHintVisible) ? (
        <VisualMnemonic glyph={glyph} />
      ) : null}
      <Suspense
        fallback={
          <Skeleton
            className="aspect-square w-full rounded-lg"
            style={{ maxWidth: sizePx }}
          />
        }
      >
        {fallback ? (
          <StrokePad
            sizePx={sizePx}
            ariaLabel={`${t('qWrite')} ${glyph}`}
            ghostGlyph={glyph}
            ghostVisible={revealed}
          />
        ) : practiceActive ? (
          <HanziWriterQuiz
            glyph={glyph}
            sizePx={sizePx}
            onComplete={grade}
            onMistake={recordMistake}
            showOutline={effectiveDifficulty !== PracticeDifficulty.MASTERY}
            fallbackAction={
              <Button variant="outline" onClick={() => setFallback(true)}>
                {strokeCopy(locale, 'practiceNoHints')}
              </Button>
            }
          />
        ) : (
          <StrokeAnimator
            glyph={glyph}
            sizePx={sizePx}
            onComplete={() => setWatched(true)}
            fallbackAction={
              <Button variant="outline" onClick={() => setFallback(true)}>
                {strokeCopy(locale, 'practiceNoAnimation')}
              </Button>
            }
          />
        )}
      </Suspense>
      {practiceActive && hasVisualMnemonic && !visualHintVisible ? (
        <Button variant="ghost" onClick={revealVisualHint}>
          {strokeCopy(
            locale,
            difficulty === PracticeDifficulty.MASTERY
              ? 'hintRecall'
              : 'hintVisual',
          )}
        </Button>
      ) : null}
      {fallback ? (
        <div className="flex flex-wrap justify-center gap-2.5">
          {revealed ? (
            <>
              <Button variant="accent" onClick={() => onResult(true)}>
                {t('selfRightL')}
              </Button>
              <Button variant="outline" onClick={() => onResult(false)}>
                {t('selfWrongL')}
              </Button>
            </>
          ) : (
            <Button onClick={() => setRevealed(true)}>{t('revealL')}</Button>
          )}
        </div>
      ) : null}
      {gradeHandwritingMutation.isPending ? (
        <p role="status" className="text-muted-foreground text-sm">
          {strokeCopy(locale, 'checkingStrokes')}
        </p>
      ) : null}
      {gradeError && completedStrokes ? (
        <ErrorState
          title={strokeCopy(locale, 'gradeError')}
          description={gradeError}
          onRetry={() => submitGrade(completedStrokes)}
          className="w-full"
        />
      ) : null}
      {handwriting ? (
        <div
          role="status"
          className="flex max-w-[440px] flex-col items-center gap-2 rounded-lg bg-[color-mix(in_srgb,var(--gold-300)_18%,var(--card))] px-4 py-3 text-center text-sm leading-normal shadow-hairline"
        >
          <span className="font-semibold">
            {strokeCopy(
              locale,
              handwriting.correct ? 'gradePass' : 'gradeFail',
            )}
          </span>
          <span>{pickLocalized(locale, handwriting.feedback)}</span>
          <span className="tabular-nums">
            {handwriting.gotStrokes} / {handwriting.expectedStrokes}
          </span>
          <Button
            variant="accent"
            onClick={() => onResult(handwriting.correct)}
          >
            {strokeCopy(locale, 'continue')}
          </Button>
        </div>
      ) : null}
      {watched && !practicing && !fallback ? (
        <Button variant="accent" onClick={() => setPracticing(true)}>
          {strokeCopy(locale, 'startMemory')}
        </Button>
      ) : null}
    </div>
  );
}

export { StrokePracticeQuiz };
