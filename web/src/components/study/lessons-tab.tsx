import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { format, subDays } from 'date-fns';
import { useState } from 'react';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { ErrorState } from '@/components/fanti/error-state';
import { ProgressBar } from '@/components/fanti/progress-bar';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { SpeakableCard } from '@/components/study/speakable-card';
import {
  GOAL_LABEL_KEY,
  GOAL_ORDER,
  knownGoal,
  pickLocalized,
  pickTriple,
  SUCCESS_BULLETS,
} from '@/components/study/study-content';
import { Textarea } from '@/components/ui/textarea';
import type { Character } from '@/gen/fanti/v1/dictionary_pb';
import {
  type CurriculumProgress,
  type Goal,
  type LearningRecord,
  QuizPool,
  type StudyProfile,
  StudyService,
} from '@/gen/fanti/v1/study_pb';
import { useDebouncedCallback } from '@/hooks/use-debounced-callback';
import { type Locale, useLocale } from '@/i18n/locale';
import { formatCount, toastRpcError } from '@/lib/book-format';
import { cn } from '@/lib/utils';

const MISSION_SAVE_DEBOUNCE_MS = 600;
const PRACTICE_DOT_COUNT = 28;
const PROFILE_INPUT = { name: 'studyProfile' } as const;

interface LessonsTabProps {
  onStartQuiz: (pool: QuizPool, lessonCharacter?: string) => void;
}

/**
 * The teaching workspace: mission + success criteria, the practice-day dot
 * calendar, today's three-step lesson, learning records, learning progress
 * with goal picker and coverage, and immersion milestones.
 */
function LessonsTab({ onStartQuiz }: LessonsTabProps) {
  const { t } = useLocale();
  const profileQuery = useQuery(
    StudyService.method.getStudyProfile,
    PROFILE_INPUT,
  );
  const lessonQuery = useQuery(StudyService.method.getLesson, {});

  if (profileQuery.isError) {
    return (
      <ErrorState
        title={t('stLessonsL')}
        description={profileQuery.error.rawMessage}
        onRetry={() => profileQuery.refetch()}
      />
    );
  }
  if (lessonQuery.isError) {
    return (
      <ErrorState
        title={t('todayT')}
        description={lessonQuery.error.rawMessage}
        onRetry={() => lessonQuery.refetch()}
      />
    );
  }
  if (!profileQuery.data || !lessonQuery.data) {
    return <Skeleton className="h-80 rounded-xl" />;
  }

  const profile = profileQuery.data;
  return (
    <div className="flex flex-col gap-4">
      <MissionCard profile={profile} />
      <PracticeRecordCard practiceDays={profile.practiceDays} />
      <TodayLessonCard
        weakCharacters={lessonQuery.data.weakCharacters}
        nextCharacter={lessonQuery.data.nextCharacter}
        onStartQuiz={onStartQuiz}
      />
      <RecordsCard profile={profile} />
      <ProgressCard profile={profile} />
      <SpeakableCard />
      <MilestonesCard profile={profile} />
    </div>
  );
}

/** Shared invalidate-profile mutation for mission and goal edits. */
function useUpdateProfileMutation() {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(StudyService.method.updateStudyProfile, {
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: StudyService.method.getStudyProfile,
          transport,
          input: PROFILE_INPUT,
          cardinality: 'finite',
        }),
      });
    },
    onError: toastRpcError,
  });
}

function MissionCard({ profile }: { profile: StudyProfile }) {
  const { t, tGloss, locale } = useLocale();
  // Local draft wins over the server value while the learner types.
  const [missionDraft, setMissionDraft] = useState<string | null>(null);
  const updateProfileMutation = useUpdateProfileMutation();
  const saveMission = useDebouncedCallback(
    (mission: string) => {
      updateProfileMutation.mutate({
        studyProfile: { name: profile.name, mission },
        updateMask: { paths: ['mission'] },
      });
    },
    MISSION_SAVE_DEBOUNCE_MS,
    { flushOnUnmount: true },
  );

  const bullets = SUCCESS_BULLETS[knownGoal(profile.goal)];

  return (
    <Card className="flex flex-col gap-2.5">
      <SectionLabel gloss={tGloss('missionT')}>{t('missionT')}</SectionLabel>
      <Textarea
        rows={2}
        aria-label={t('missionT')}
        placeholder={t('missionWhyPh')}
        value={missionDraft ?? profile.mission}
        onChange={(event) => {
          setMissionDraft(event.target.value);
          saveMission(event.target.value);
        }}
        className="resize-none bg-transparent font-reading text-base"
      />
      <div className="mt-0.5 font-medium text-sm">{t('successT')}</div>
      <div className="flex flex-col gap-1.5">
        {bullets.map((bullet) => (
          <div
            key={bullet[0]}
            className="flex items-center gap-2 text-muted-foreground text-sm"
          >
            <span
              aria-hidden="true"
              className="size-1.5 flex-none rounded-full bg-secondary"
            />
            {pickTriple(locale, bullet)}
          </div>
        ))}
      </div>
    </Card>
  );
}

function PracticeRecordCard({ practiceDays }: { practiceDays: string[] }) {
  const { t, tGloss, locale } = useLocale();
  const practiced = new Set(practiceDays);
  const today = new Date();
  const dots = Array.from({ length: PRACTICE_DOT_COUNT }, (_, index) => {
    const day = format(
      subDays(today, PRACTICE_DOT_COUNT - 1 - index),
      'yyyy-MM-dd',
    );
    return { day, filled: practiced.has(day) };
  });
  const filledCount = dots.filter((dot) => dot.filled).length;
  const countLabel =
    locale === 'en'
      ? `${filledCount} / ${PRACTICE_DOT_COUNT} days`
      : `${filledCount} / ${PRACTICE_DOT_COUNT} 天`;

  return (
    <Card className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <SectionLabel gloss={tGloss('streakT')}>{t('streakT')}</SectionLabel>
        <span className="whitespace-nowrap text-[11px] text-muted-foreground tabular-nums">
          {countLabel}
        </span>
      </div>
      <div className="grid max-w-[440px] grid-cols-[repeat(14,1fr)] gap-1.5">
        {dots.map((dot) => (
          <span
            key={dot.day}
            title={dot.day}
            className={cn(
              'aspect-square rounded-full',
              dot.filled ? 'bg-accent' : 'bg-muted',
            )}
          />
        ))}
      </div>
      <div className="text-[11px] text-muted-foreground leading-snug">
        {t('streakSub')}
      </div>
    </Card>
  );
}

function TodayLessonCard({
  weakCharacters,
  nextCharacter,
  onStartQuiz,
}: {
  weakCharacters: Character[];
  nextCharacter: Character | undefined;
  onStartQuiz: (pool: QuizPool, lessonCharacter?: string) => void;
}) {
  const { t, tGloss } = useLocale();

  return (
    <Card className="flex flex-col gap-3.5 shadow-gold">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <SectionLabel gloss={tGloss('todayT')}>{t('todayT')}</SectionLabel>
        <span className="text-[11px] text-muted-foreground">
          {t('lessonTime')}
        </span>
      </div>

      {weakCharacters.length > 0 ? (
        <div className="flex flex-wrap items-center gap-3">
          <LessonStepNumber step={1} />
          <div className="flex min-w-40 flex-1 flex-col gap-1.5">
            <span className="font-medium text-sm">{t('lessonStep1')}</span>
            <div className="flex flex-wrap gap-1.5">
              {weakCharacters.map((weak) => (
                <Link
                  key={weak.traditional}
                  to="/characters/$char"
                  params={{ char: weak.traditional }}
                  className="flex min-h-9 min-w-9 items-center justify-center rounded-lg bg-muted px-2.5 font-display text-lg transition-colors hover:bg-gold-300/30 focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  {weak.traditional}
                </Link>
              ))}
            </div>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => onStartQuiz(QuizPool.WEAK)}
          >
            {t('startStep')}
          </Button>
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-3">
        <LessonStepNumber step={2} />
        <div className="flex min-w-40 flex-1 flex-col gap-1">
          <span className="font-medium text-sm">{t('lessonStep2')}</span>
          {nextCharacter ? (
            <div className="flex items-center gap-2">
              <span className="font-display text-[26px]">
                {nextCharacter.traditional}
              </span>
              <span className="text-muted-foreground text-sm">
                {nextCharacter.pinyin} · {nextCharacter.meaning}
              </span>
            </div>
          ) : (
            <span className="text-status-exact text-sm">
              {t('curriculumComplete')}
            </span>
          )}
        </div>
        {nextCharacter ? (
          <Button
            variant="outline"
            size="sm"
            render={
              <Link
                to="/characters/$char"
                params={{ char: nextCharacter.traditional }}
              />
            }
          >
            {t('meetChar')}
          </Button>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <LessonStepNumber step={3} />
        <span className="min-w-40 flex-1 font-medium text-sm">
          {t('lessonStep3')}
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onStartQuiz(QuizPool.ALL)}
        >
          {t('startStep')}
        </Button>
      </div>

      {nextCharacter ? (
        <Button
          size="lg"
          className="w-full"
          onClick={() =>
            onStartQuiz(QuizPool.LESSON, nextCharacter.traditional)
          }
        >
          {t('startLesson')}
        </Button>
      ) : null}
    </Card>
  );
}

function LessonStepNumber({ step }: { step: number }) {
  return (
    <span className="flex size-[30px] flex-none items-center justify-center rounded-full bg-secondary font-semibold text-secondary-foreground text-xs">
      {step}
    </span>
  );
}

function recordText(
  record: LearningRecord,
  profile: StudyProfile,
  locale: Locale,
): string {
  if (record.type === 'milestone') {
    const milestone = profile.milestones.find(
      (candidate) => candidate.threshold === record.milestoneThreshold,
    );
    const label = pickLocalized(locale, milestone?.label);
    return label || formatCount(record.milestoneThreshold);
  }
  return locale === 'en'
    ? `Learned ${record.character}`
    : locale === 'tc'
      ? `學會了 ${record.character}`
      : `学会了 ${record.character}`;
}

function RecordsCard({ profile }: { profile: StudyProfile }) {
  const { t, tGloss, locale } = useLocale();

  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('recordsT')} className="mb-1">
        {t('recordsT')}
      </SectionLabel>
      {profile.records.length === 0 ? (
        <div className="pt-3 pb-1.5 text-muted-foreground text-sm">
          {t('recordsEmpty')}
        </div>
      ) : (
        profile.records.map((record) => (
          <div
            key={`${record.type}:${record.character}:${record.milestoneThreshold}:${record.recordTime?.seconds ?? 0n}`}
            className="flex items-center gap-3 border-foreground/7 border-t pt-2.5 pb-1 first:border-t-0"
          >
            <span
              className={cn(
                'flex size-[30px] flex-none items-center justify-center rounded-lg font-display text-base',
                record.type === 'milestone'
                  ? 'bg-gold-300/30 text-foreground'
                  : 'bg-muted',
              )}
            >
              {record.type === 'milestone' ? '✓' : record.character}
            </span>
            <span className="min-w-0 flex-1 text-sm">
              {recordText(record, profile, locale)}
            </span>
            <span className="flex-none text-[11px] text-muted-foreground tabular-nums">
              {record.recordTime
                ? format(timestampDate(record.recordTime), 'MM-dd')
                : ''}
            </span>
          </div>
        ))
      )}
    </Card>
  );
}

function ProgressCard({ profile }: { profile: StudyProfile }) {
  const { t } = useLocale();
  const updateProfileMutation = useUpdateProfileMutation();
  const coveragePercent = Math.round(profile.coverage * 100);

  function pickGoal(goal: Goal) {
    if (goal === profile.goal) {
      return;
    }
    updateProfileMutation.mutate({
      studyProfile: { name: profile.name, goal },
      updateMask: { paths: ['goal'] },
    });
  }

  return (
    <Card className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2">
        <SectionLabel>{t('goalTitle')}</SectionLabel>
        {GOAL_ORDER.map((goal) => (
          <Chip
            key={goal}
            selected={knownGoal(profile.goal) === goal}
            onClick={() => pickGoal(goal)}
            className="min-h-8 px-3 text-xs"
          >
            {t(GOAL_LABEL_KEY[goal])}
          </Chip>
        ))}
      </div>

      {profile.curriculumProgress ? (
        <CurriculumProgressRows progress={profile.curriculumProgress} />
      ) : null}

      <div className="mt-1 flex items-baseline justify-between gap-3">
        <span className="whitespace-nowrap font-semibold text-[11px] text-muted-foreground uppercase tracking-[0.12em]">
          {t('coverageL')}
        </span>
        <span className="font-semibold text-md text-secondary-foreground tabular-nums">
          {coveragePercent}%
        </span>
      </div>
      <ProgressBar value={profile.coverage} label={t('coverageL')} />
      <div className="text-[11px] text-muted-foreground leading-normal">
        {t('coverageFact')}
      </div>
      <div className="text-[11px] text-muted-foreground">{t('courseLine')}</div>

      {profile.examReadiness.length > 0 ? (
        <div className="flex flex-wrap items-center gap-2">
          <SectionLabel>{t('examReady')}</SectionLabel>
          {profile.examReadiness.map((readiness) => (
            <span
              key={readiness.level}
              className={cn(
                'whitespace-nowrap rounded-full px-2.5 py-[3px] text-[11px] tabular-nums',
                readiness.progress >= 1
                  ? 'bg-accent/16 font-semibold text-status-exact'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {readiness.level} · {Math.round(readiness.progress * 100)}%
            </span>
          ))}
        </div>
      ) : null}

      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
        <span
          aria-hidden="true"
          className="inline-block size-[7px] rounded-full bg-status-exact"
        />
        {t('syncNote')}
      </div>
    </Card>
  );
}

function CurriculumProgressRows({
  progress,
}: {
  progress: CurriculumProgress;
}) {
  const { t } = useLocale();
  const rows = [
    {
      key: 'core',
      label: t('coreCurriculum'),
      learned: progress.coreLearned,
      size: progress.coreSize,
    },
    {
      key: 'complete',
      label: t('completeCurriculum'),
      learned: progress.completeLearned,
      size: progress.completeSize,
    },
    {
      key: 'reference',
      label: t('referenceLearned'),
      learned: progress.referenceLearned,
      size: progress.referenceSize,
    },
  ];

  return (
    <div className="grid gap-3 sm:grid-cols-3">
      {rows.map((row) => (
        <div key={row.key} className="flex min-w-0 flex-col gap-1.5">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <span className="font-medium text-sm">{row.label}</span>
            <span className="text-[11px] text-muted-foreground tabular-nums">
              {formatCount(row.learned)} / {formatCount(row.size)}
            </span>
          </div>
          <ProgressBar
            value={row.size === 0 ? 0 : row.learned / row.size}
            label={row.label}
          />
        </div>
      ))}
    </div>
  );
}

function MilestonesCard({ profile }: { profile: StudyProfile }) {
  const { t, tGloss, locale } = useLocale();
  if (profile.milestones.length === 0) {
    return null;
  }

  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('milestonesT')} className="mb-1">
        {t('milestonesT')}
      </SectionLabel>
      {profile.milestones.map((milestone) => (
        <div
          key={milestone.threshold}
          className="flex items-center gap-3 border-foreground/7 border-t pt-2.5 pb-1 first:border-t-0"
        >
          <span
            className={cn(
              'w-12 flex-none font-semibold text-sm tabular-nums',
              milestone.reached ? 'text-status-exact' : 'text-muted-foreground',
            )}
          >
            {formatCount(milestone.threshold)}
          </span>
          <span
            className={cn(
              'min-w-0 flex-1 text-sm',
              milestone.reached ? '' : 'text-muted-foreground',
            )}
          >
            {pickLocalized(locale, milestone.label)}
          </span>
          {milestone.reached ? (
            <span className="flex-none whitespace-nowrap rounded-full bg-accent/16 px-2.5 py-[3px] font-semibold text-[10px] text-status-exact">
              ✓ {t('msDone')}
            </span>
          ) : null}
        </div>
      ))}
    </Card>
  );
}

export { LessonsTab };
