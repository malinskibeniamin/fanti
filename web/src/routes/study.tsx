import { useMutation, useQuery } from '@connectrpc/connect-query';
import { createFileRoute, Link } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';

import { ErrorState } from '@/components/fanti/error-state';
import { PageHeading } from '@/components/fanti/page-heading';
import { Skeleton } from '@/components/fanti/skeleton';
import { CardsTab } from '@/components/study/cards-tab';
import { DictTab } from '@/components/study/dict-tab';
import { FantiCharacterCard } from '@/components/study/fanti-character-card';
import { LessonsTab } from '@/components/study/lessons-tab';
import { QuizTab } from '@/components/study/quiz-tab';
import { StrokesTab } from '@/components/study/strokes-tab';
import { hskCefrLabel } from '@/components/study/study-content';
import { CardMode, type Quiz, StudyService } from '@/gen/fanti/v1/study_pb';
import { useLocale } from '@/i18n/locale';
import { toastRpcError } from '@/lib/book-format';
import { cn } from '@/lib/utils';

type StudyTab = 'lessons' | 'cards' | 'quiz' | 'strokes' | 'dict' | 'origins';

const STUDY_TABS: readonly StudyTab[] = [
  'lessons',
  'cards',
  'quiz',
  'strokes',
  'dict',
  'origins',
];

interface StudySearch {
  tab: StudyTab;
}

export const Route = createFileRoute('/study')({
  component: StudyPage,
  validateSearch: (search: unknown): StudySearch => {
    const candidate =
      typeof search === 'object' && search !== null
        ? (search as { tab?: unknown })
        : {};
    const tab = STUDY_TABS.find((known) => known === candidate.tab);
    return { tab: tab ?? 'lessons' };
  },
});

function StudyPage() {
  const { t, tGloss } = useLocale();
  const { tab } = Route.useSearch();
  const navigate = Route.useNavigate();
  const [quiz, setQuiz] = useState<Quiz>();
  const tabListRef = useRef<HTMLElement>(null);

  useEffect(
    function keepSelectedTabVisible() {
      const tabList = tabListRef.current;
      const selectedTab = tabList?.querySelector<HTMLElement>(
        `a[href="/study?tab=${tab}"]`,
      );
      if (!(tabList && selectedTab)) {
        return;
      }
      const scrollContainer = tabList;
      const tabToReveal = selectedTab;

      function revealSelectedTab() {
        if (tabToReveal.offsetLeft < scrollContainer.scrollLeft) {
          scrollContainer.scrollLeft = tabToReveal.offsetLeft;
          return;
        }

        const selectedRight = tabToReveal.offsetLeft + tabToReveal.offsetWidth;
        const visibleRight =
          scrollContainer.scrollLeft + scrollContainer.clientWidth;
        if (selectedRight > visibleRight) {
          scrollContainer.scrollLeft =
            selectedRight - scrollContainer.clientWidth;
        }
      }

      revealSelectedTab();
      const resizeObserver = new ResizeObserver(revealSelectedTab);
      resizeObserver.observe(scrollContainer);
      resizeObserver.observe(tabToReveal);

      return () => resizeObserver.disconnect();
    },
    [tab],
  );

  const createQuizMutation = useMutation(StudyService.method.createQuiz, {
    onSuccess: (created) => {
      setQuiz(created);
      void navigate({ search: { tab: 'quiz' } });
    },
    onError: toastRpcError,
  });

  const labels: Record<StudyTab, string> = {
    lessons: t('stLessonsL'),
    cards: t('stCardsL'),
    quiz: t('quizL'),
    strokes: t('stStrokesL'),
    dict: t('stDictL'),
    origins: t('stStoriesL'),
  };

  return (
    <section className="flex animate-fanti-fade flex-col gap-4.5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <PageHeading gloss={tGloss('navSt')} title={t('navSt')} />
        <nav
          ref={tabListRef}
          className="flex max-w-full flex-nowrap justify-start gap-1 overflow-x-auto rounded-full bg-muted p-0.75 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          aria-label={t('navSt')}
        >
          {STUDY_TABS.map((key) => (
            <Link
              key={key}
              from={Route.fullPath}
              search={{ tab: key }}
              className={cn(
                'inline-flex min-h-9 shrink-0 items-center justify-center rounded-full px-4 py-1.5 font-ui text-sm transition-colors focus-visible:ring-3 focus-visible:ring-ring/50',
                tab === key
                  ? 'bg-card font-semibold text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {labels[key]}
            </Link>
          ))}
        </nav>
      </div>

      {tab === 'lessons' ? (
        <LessonsTab
          onStartQuiz={(pool, lessonCharacter) =>
            createQuizMutation.mutate({ pool, lessonCharacter })
          }
        />
      ) : null}
      {tab === 'cards' ? <CardsTab /> : null}
      {tab === 'quiz' ? (
        <QuizTab
          quiz={quiz}
          onQuizChange={setQuiz}
          onStart={(pool, lessonCharacter) =>
            createQuizMutation.mutate({ pool, lessonCharacter })
          }
          onRetry={() => setQuiz(undefined)}
          startPending={createQuizMutation.isPending}
        />
      ) : null}
      {tab === 'strokes' ? <StrokesTab /> : null}
      {tab === 'dict' ? <DictTab /> : null}
      {tab === 'origins' ? <OriginsTab /> : null}
    </section>
  );
}

/** Origin stories — the starter deck's character cards with HSK pills. */
function OriginsTab() {
  const { t } = useLocale();
  const deckQuery = useQuery(StudyService.method.listDueCards, {
    mode: CardMode.CHARACTER,
    pageSize: 20,
  });

  if (deckQuery.isError) {
    return (
      <ErrorState
        title={t('stStoriesL')}
        description={deckQuery.error.message}
        onRetry={() => deckQuery.refetch()}
      />
    );
  }

  if (deckQuery.isPending) {
    return <Skeleton className="h-80 rounded-xl" />;
  }

  return (
    <div className="flex flex-col gap-3.5">
      <p className="text-muted-foreground text-sm leading-normal">
        {t('storiesIntro')}
      </p>
      {deckQuery.data.dueCards.map((card) =>
        card.character ? (
          <div key={card.character.name} className="flex flex-col gap-1.5">
            <span className="self-end whitespace-nowrap rounded-full bg-muted px-2.5 py-0.5 text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]">
              {hskCefrLabel(card.character.hskLevel)}
            </span>
            <FantiCharacterCard character={card.character} />
          </div>
        ) : null,
      )}
    </div>
  );
}
