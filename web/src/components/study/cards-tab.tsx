import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { EmptyState } from '@/components/fanti/empty-state';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import { ExampleSentences } from '@/components/study/example-sentences';
import { FantiCharacterCard } from '@/components/study/fanti-character-card';
import { hskCefrLabel } from '@/components/study/study-content';
import { Button as UiButton } from '@/components/ui/button';
import type { Character } from '@/gen/fanti/v1/dictionary_pb';
import { CardMode, Grade, StudyService } from '@/gen/fanti/v1/study_pb';
import { type Locale, useLocale } from '@/i18n/locale';
import { toastRpcError } from '@/lib/book-format';
import { cn } from '@/lib/utils';

const DUE_PAGE_SIZE = 30;

function dueLabel(locale: Locale, count: number): string {
  return locale === 'en'
    ? `${count} due`
    : locale === 'tc'
      ? `待複習 ${count}`
      : `待复习 ${count}`;
}

/** Glyph size that still fits multi-character words on the 180px tile. */
function glyphSizeClass(glyph: string): string {
  if (glyph.length >= 3) {
    return 'text-[44px]';
  }
  return glyph.length === 2 ? 'text-[64px]' : 'text-[96px]';
}

/**
 * SRS flashcards: due header with character/word mode chips, the unflipped
 * 田-grid prompt, and the flipped answer (CharacterCard + examples) with the
 * again / good / easy grading row.
 */
function CardsTab() {
  const { t, tGloss, locale } = useLocale();
  const queryClient = useQueryClient();
  const transport = useTransport();

  const [mode, setMode] = useState<CardMode>(CardMode.CHARACTER);
  const [index, setIndex] = useState(0);
  const [flipped, setFlipped] = useState(false);

  const listInput = { mode, pageSize: DUE_PAGE_SIZE };
  const dueQuery = useQuery(StudyService.method.listDueCards, listInput);

  const gradeCardMutation = useMutation(StudyService.method.gradeCard, {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: createConnectQueryKey({
            schema: StudyService.method.listDueCards,
            transport,
            input: listInput,
            cardinality: 'finite',
          }),
        }),
        queryClient.invalidateQueries({
          queryKey: createConnectQueryKey({
            schema: StudyService.method.getStudyProfile,
            transport,
            input: { name: 'studyProfile' },
            cardinality: 'finite',
          }),
        }),
        // A grade can tip a character into learned, changing what the
        // speakable summary unlocks.
        queryClient.invalidateQueries({
          queryKey: createConnectQueryKey({
            schema: StudyService.method.getSpeakableSummary,
            transport,
            input: {},
            cardinality: 'finite',
          }),
        }),
      ]);
      setFlipped(false);
      setIndex((previous) => previous + 1);
    },
    onError: toastRpcError,
  });

  if (dueQuery.isError) {
    return (
      <ErrorState
        title={t('stCardsL')}
        description={dueQuery.error.rawMessage}
        onRetry={() => dueQuery.refetch()}
      />
    );
  }
  if (!dueQuery.data) {
    return <Skeleton className="h-80 rounded-xl" />;
  }

  const cards = dueQuery.data.dueCards;
  const card = cards.length > 0 ? cards[index % cards.length] : undefined;
  const character = card?.character;

  function grade(value: Grade) {
    if (!character || gradeCardMutation.isPending) {
      return;
    }
    gradeCardMutation.mutate({
      name: `reviews/${character.traditional}`,
      grade: value,
    });
  }

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center justify-between gap-2.5 text-muted-foreground text-sm tabular-nums">
        <span>{dueLabel(locale, dueQuery.data.dueCount)}</span>
        <div
          role="toolbar"
          aria-label={`${t('cardModeChar')} / ${t('cardModeWord')}`}
          className="flex gap-0.5 rounded-full bg-muted p-[3px]"
        >
          {(
            [
              [CardMode.CHARACTER, tGloss('cardModeChar'), t('cardModeChar')],
              [CardMode.WORD, tGloss('cardModeWord'), t('cardModeWord')],
            ] as const
          ).map(([value, , label]) => (
            <UiButton
              variant="unstyled"
              size="unstyled"
              key={value}
              type="button"
              aria-pressed={mode === value}
              onClick={() => {
                setMode(value);
                setIndex(0);
                setFlipped(false);
              }}
              className={cn(
                'min-h-8 cursor-pointer rounded-full border-none px-3 font-ui text-xs outline-none transition-colors duration-(--duration-fast) focus-visible:ring-3 focus-visible:ring-ring/50',
                mode === value
                  ? 'bg-card font-semibold text-foreground shadow-xs'
                  : 'bg-transparent font-normal text-muted-foreground',
              )}
            >
              {label}
            </UiButton>
          ))}
        </div>
        <span>
          {cards.length > 0
            ? `${(index % cards.length) + 1} / ${cards.length}`
            : '0 / 0'}
        </span>
      </div>

      {!character ? (
        <EmptyState
          glyph="卡"
          title={dueLabel(locale, 0)}
          description={t('syncNote')}
        />
      ) : !flipped ? (
        <UnflippedCard character={character} onFlip={() => setFlipped(true)} />
      ) : (
        <div className="flex flex-col gap-3">
          <FantiCharacterCard character={character} />
          {character.examples.length > 0 ? (
            <ExamplesCard character={character} />
          ) : null}
          <div className="grid grid-cols-3 gap-2.5">
            <Button
              variant="outline"
              size="lg"
              onClick={() => grade(Grade.AGAIN)}
            >
              {t('again')}
            </Button>
            <Button
              variant="secondary"
              size="lg"
              onClick={() => grade(Grade.GOOD)}
            >
              {t('good')}
            </Button>
            <Button
              variant="accent"
              size="lg"
              onClick={() => grade(Grade.EASY)}
            >
              {t('easy')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

function UnflippedCard({
  character,
  onFlip,
}: {
  character: Character;
  onFlip: () => void;
}) {
  const { t } = useLocale();
  const levelLabel = hskCefrLabel(character.hskLevel) || t('properTag');

  return (
    <Card className="flex flex-col items-center gap-[18px] px-5 py-9 shadow-md">
      <span className="whitespace-nowrap rounded-full bg-muted px-2.5 py-[3px] text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]">
        {levelLabel}
      </span>
      <div className="relative flex size-[180px] items-center justify-center rounded-lg shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--primary)_30%,transparent)]">
        <div
          aria-hidden="true"
          className="absolute inset-y-2 left-1/2 border-[color-mix(in_srgb,var(--primary)_22%,transparent)] border-l border-dashed"
        />
        <div
          aria-hidden="true"
          className="absolute inset-x-2 top-1/2 border-[color-mix(in_srgb,var(--primary)_22%,transparent)] border-t border-dashed"
        />
        <span
          className={cn(
            'relative font-display leading-none',
            glyphSizeClass(character.traditional),
          )}
        >
          {character.traditional}
        </span>
      </div>
      <div className="text-muted-foreground text-sm">{t('flipQ')}</div>
      <Button size="lg" onClick={onFlip}>
        {t('showAns')}
      </Button>
    </Card>
  );
}

function ExamplesCard({ character }: { character: Character }) {
  const { t } = useLocale();
  const levelLabel = hskCefrLabel(character.hskLevel) || t('properTag');

  return (
    <Card className="px-4 py-3.5">
      <div className="mb-1 flex items-center justify-between">
        <span className="font-semibold text-[10px] text-muted-foreground uppercase tracking-[0.16em]">
          {t('examplesTitle')}
        </span>
        <span className="whitespace-nowrap rounded-full bg-muted px-2.5 py-[3px] text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]">
          {levelLabel}
        </span>
      </div>
      <ExampleSentences examples={character.examples} />
    </Card>
  );
}

export { CardsTab };
