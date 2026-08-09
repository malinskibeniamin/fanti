import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useState } from 'react';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { Volume2 } from '@/components/icons';
import { StrokePracticeQuiz } from '@/components/study/stroke-practice-quiz';
import { pickLocalized } from '@/components/study/study-content';
import { Button as UiButton } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  CardMode,
  QuestionType,
  type Quiz,
  QuizPool,
  StudyService,
  type SubmitQuizAnswerResponse,
} from '@/gen/fanti/v1/study_pb';
import { type Locale, useLocale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';
import { toastRpcError } from '@/lib/book-format';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

const WRITE_PAD_SIZE = 260;
const MISTAKE_SCAN_PAGE_SIZE = 50;
const CJK_RE = /[㐀-鿿]/;

const QUESTION_LABEL_KEY: Partial<Record<QuestionType, StringKey>> = {
  [QuestionType.READING]: 'qReading',
  [QuestionType.MEANING]: 'qMeaning',
  [QuestionType.SCRIPT]: 'qScript',
  [QuestionType.MATCH]: 'qMatch',
  [QuestionType.WRITE]: 'qWrite',
  [QuestionType.TYPE]: 'qType',
  [QuestionType.TONE]: 'qTone',
  [QuestionType.LISTEN]: 'qListen',
  [QuestionType.CLOZE]: 'qCloze',
};

function isAudioQuestion(type: QuestionType): boolean {
  return type === QuestionType.TONE || type === QuestionType.LISTEN;
}

function scoreLabel(locale: Locale, score: number): string {
  return locale === 'en' ? `Score ${score}` : `得分 ${score}`;
}

type ReadingSystem = 'pinyin' | 'zhuyin';

interface QuizTabProps {
  quiz: Quiz | undefined;
  onQuizChange: (quiz: Quiz) => void;
  onStart: (pool: QuizPool, lessonCharacter?: string) => void;
  onRetry: () => void;
  startPending: boolean;
}

/**
 * The mixed quiz: intro launcher, one question card per type (option grids,
 * IME typing, and the handwriting canvas), tutor feedback after every answer,
 * and the score / mistakes summary.
 */
function QuizTab({
  quiz,
  onQuizChange,
  onStart,
  onRetry,
  startPending,
}: QuizTabProps) {
  if (!quiz) {
    return <QuizIntro onStart={onStart} startPending={startPending} />;
  }
  if (quiz.finished) {
    return <QuizFinished quiz={quiz} onRetry={onRetry} />;
  }
  return (
    <QuizQuestionCard
      key={`${quiz.name}:${quiz.currentIndex}`}
      quiz={quiz}
      onQuizChange={onQuizChange}
    />
  );
}

function QuizIntro({
  onStart,
  startPending,
}: {
  onStart: (pool: QuizPool) => void;
  startPending: boolean;
}) {
  const { t } = useLocale();
  // Display-only preference for how readings show in this session.
  const [readingSystem, setReadingSystem] = useState<ReadingSystem>('pinyin');

  const dueQuery = useQuery(StudyService.method.listDueCards, {
    mode: CardMode.CHARACTER,
    pageSize: MISTAKE_SCAN_PAGE_SIZE,
  });
  const hasMistakes = (dueQuery.data?.dueCards ?? []).some(
    (card) => (card.review?.mistakeCount ?? 0) > 0,
  );

  function start(pool: QuizPool) {
    if (startPending) {
      return;
    }
    onStart(pool);
  }

  return (
    <Card className="flex flex-col items-center gap-3.5 px-5 py-6">
      <div className="font-display text-xl">{t('quizL')}</div>
      <div className="max-w-[420px] text-center text-muted-foreground text-sm leading-normal">
        {t('quizIntroTxt')}
      </div>
      <div className="font-semibold text-[11px] text-accent">
        {t('tutorFree')}
      </div>
      <div className="flex items-center gap-2.5">
        <span className="font-medium text-sm">{t('readingSys')}</span>
        {(
          [
            ['pinyin', t('pinyinL')],
            ['zhuyin', '注音'],
          ] as const
        ).map(([value, label]) => (
          <Chip
            key={value}
            selected={readingSystem === value}
            onClick={() => setReadingSystem(value)}
            className="min-h-8 px-3 text-xs"
          >
            {label}
          </Chip>
        ))}
      </div>
      <Button size="lg" onClick={() => start(QuizPool.ALL)}>
        {t('startQuizL')}
      </Button>
      {hasMistakes ? (
        <Button variant="outline" onClick={() => start(QuizPool.WEAK)}>
          {t('mistakesL')}
        </Button>
      ) : null}
    </Card>
  );
}

function QuizQuestionCard({
  quiz,
  onQuizChange,
}: {
  quiz: Quiz;
  onQuizChange: (quiz: Quiz) => void;
}) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const transport = useTransport();

  const question = quiz.questions[quiz.currentIndex];
  const [answer, setAnswer] = useState<SubmitQuizAnswerResponse>();
  const [typed, setTyped] = useState('');

  const submitMutation = useMutation(StudyService.method.submitQuizAnswer, {
    onSuccess: async (response) => {
      setAnswer(response);
      if (response.quiz?.finished) {
        // Mistake counts feed the profile and the due deck.
        await Promise.all([
          queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({
              schema: StudyService.method.getStudyProfile,
              transport,
              input: { name: 'studyProfile' },
              cardinality: 'finite',
            }),
          }),
          queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({
              schema: StudyService.method.listDueCards,
              transport,
              cardinality: 'finite',
            }),
          }),
          // A finished lesson quiz marks its character learned, changing
          // what the speakable summary unlocks.
          queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({
              schema: StudyService.method.getSpeakableSummary,
              transport,
              input: {},
              cardinality: 'finite',
            }),
          }),
        ]);
      }
    },
    onError: toastRpcError,
  });

  if (!question) {
    return null;
  }

  const answered = answer !== undefined;
  const labelKey = QUESTION_LABEL_KEY[question.type];

  function submitOption(optionIndex: number) {
    if (answered || submitMutation.isPending) {
      return;
    }
    submitMutation.mutate({
      name: quiz.name,
      questionIndex: quiz.currentIndex,
      answer: { case: 'optionIndex', value: optionIndex },
    });
  }

  function submitTyped() {
    if (answered || submitMutation.isPending || typed.trim() === '') {
      return;
    }
    submitMutation.mutate({
      name: quiz.name,
      questionIndex: quiz.currentIndex,
      answer: { case: 'typedText', value: typed.trim() },
    });
  }

  function submitSelf(selfCorrect: boolean) {
    if (answered || submitMutation.isPending) {
      return;
    }
    submitMutation.mutate({
      name: quiz.name,
      questionIndex: quiz.currentIndex,
      answer: { case: 'selfCorrect', value: selfCorrect },
    });
  }

  function next() {
    if (answer?.quiz) {
      onQuizChange(answer.quiz);
    }
  }

  const feedbackText = pickLocalized(locale, answer?.feedback);

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex justify-between text-muted-foreground text-sm tabular-nums">
        <span>
          {quiz.currentIndex + 1} / {quiz.questions.length}
        </span>
        <span>{scoreLabel(locale, quiz.score)}</span>
      </div>

      <Card className="flex flex-col items-center gap-4 px-5 py-6 shadow-md">
        <div className="text-muted-foreground text-sm">
          {labelKey ? t(labelKey) : ''}
        </div>

        {question.type === QuestionType.CLOZE ? (
          <div className="max-w-[460px] text-center font-reading text-[22px] leading-[2]">
            {question.prompt}
          </div>
        ) : isAudioQuestion(question.type) ? (
          <div className="flex flex-col items-center gap-2">
            <UiButton
              variant="unstyled"
              size="unstyled"
              type="button"
              aria-label={t('tapToHear')}
              onClick={() => speak(question.ttsText)}
              className="flex size-[88px] cursor-pointer items-center justify-center rounded-full bg-gold-300/30 text-foreground shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--secondary)_60%,transparent)] transition-colors hover:bg-gold-300/48 focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <Volume2 aria-hidden="true" className="size-[34px]" />
            </UiButton>
            <span className="text-[11px] text-muted-foreground">
              {t('tapToHear')}
            </span>
          </div>
        ) : question.type === QuestionType.WRITE ? null : question.prompt ? (
          <span className="font-display text-[90px] leading-[1.1]">
            {question.prompt}
          </span>
        ) : null}

        {question.type === QuestionType.WRITE && !answered ? (
          <StrokePracticeQuiz
            glyph={question.character}
            sizePx={WRITE_PAD_SIZE}
            onResult={submitSelf}
          />
        ) : null}

        {question.type === QuestionType.TYPE ? (
          <>
            {!answered ? (
              <div className="flex w-full max-w-[380px] items-center gap-2.5">
                <Input
                  aria-label={t('qType')}
                  value={typed}
                  onChange={(event) => setTyped(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      submitTyped();
                    }
                  }}
                  className="h-11 flex-1 font-reading text-lg"
                />
                <Button size="lg" onClick={submitTyped}>
                  {t('submitL')}
                </Button>
              </div>
            ) : null}
            <div className="text-[11px] text-muted-foreground">
              {t('typeHintKb')}
            </div>
            {answered ? (
              <div className="flex w-full max-w-[440px] flex-col gap-2.5 rounded-lg bg-[color-mix(in_srgb,var(--gold-300)_18%,var(--card))] px-4 py-3.5 shadow-hairline">
                <div className="flex justify-center gap-7">
                  <div className="flex flex-col items-center gap-1">
                    <span className="text-[10px] text-muted-foreground uppercase tracking-[0.1em]">
                      {t('youTyped')}
                    </span>
                    <span className="font-display text-[44px] text-status-manual leading-tight">
                      {typed.trim()}
                    </span>
                  </div>
                  <div className="flex flex-col items-center gap-1">
                    <span className="text-[10px] text-muted-foreground uppercase tracking-[0.1em]">
                      {t('correctIs')}
                    </span>
                    <span className="font-display text-[44px] text-status-exact leading-tight">
                      {question.character}
                    </span>
                  </div>
                </div>
              </div>
            ) : null}
          </>
        ) : null}

        {question.options.length > 0 ? (
          <div className="grid w-full max-w-[440px] grid-cols-2 gap-2.5">
            {question.options.map((option, optionIndex) => (
              <UiButton
                variant="unstyled"
                size="unstyled"
                key={option}
                type="button"
                disabled={answered}
                onClick={() => submitOption(optionIndex)}
                className={cn(
                  'min-h-11 cursor-pointer rounded-lg bg-muted px-3 py-2 text-foreground transition-colors hover:bg-gold-300/30 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-60',
                  CJK_RE.test(option) ? 'font-display text-2xl' : 'text-base',
                  answered &&
                    answer?.correctAnswer === option &&
                    'bg-accent/16 text-status-exact opacity-100',
                )}
              >
                {option}
              </UiButton>
            ))}
          </div>
        ) : null}

        {/* Tutor verdict — announced to screen readers as it appears. */}
        <div aria-live="polite" className="flex flex-col items-center gap-3">
          {answered && answer ? (
            <>
              <div
                className={cn(
                  'max-w-[440px] text-center text-sm leading-normal',
                  answer.correct ? 'text-status-exact' : 'text-status-manual',
                )}
              >
                {answer.correct
                  ? t('rightFb')
                  : `${t('wrongFb')}${answer.correctAnswer}`}
                {feedbackText ? (
                  <span className="mt-1 block text-foreground">
                    {feedbackText}
                  </span>
                ) : null}
              </div>
              <Button variant="secondary" onClick={next}>
                {t('nextQ')}
              </Button>
            </>
          ) : null}
        </div>
      </Card>
    </div>
  );
}

function QuizFinished({ quiz, onRetry }: { quiz: Quiz; onRetry: () => void }) {
  const { t } = useLocale();

  return (
    <Card className="flex flex-col items-center gap-3.5 px-5 py-7 shadow-md">
      <div className="font-display text-xl">{t('quizDoneL')}</div>
      <div className="font-semibold text-[34px] tabular-nums">
        {quiz.score} / {quiz.questions.length}
      </div>
      {quiz.mistakes.length > 0 ? (
        <>
          <div className="text-center text-[11px] text-muted-foreground">
            <a
              href="https://www.reddit.com/r/ChineseLanguage/"
              target="_blank"
              rel="noreferrer"
              className="text-accent no-underline"
            >
              {t('communityNudge')}
            </a>
          </div>
          <div className="text-muted-foreground text-sm">{t('mistakesL')}</div>
          <div className="flex flex-wrap justify-center gap-2">
            {quiz.mistakes.map((glyph) => (
              <Link
                key={glyph}
                to="/characters/$char"
                params={{ char: glyph }}
                className="flex min-h-10 min-w-10 items-center justify-center rounded-lg bg-muted px-2.5 font-display text-xl transition-colors hover:bg-gold-300/30 focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                {glyph}
              </Link>
            ))}
          </div>
        </>
      ) : (
        <div className="text-sm text-status-exact">{t('noMistakesL')}</div>
      )}
      <Button variant="secondary" size="lg" onClick={onRetry}>
        {t('retryQuizL')}
      </Button>
    </Card>
  );
}

export { QuizTab };
