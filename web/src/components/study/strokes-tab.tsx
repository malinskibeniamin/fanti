import { useQuery } from '@connectrpc/connect-query';
import { useState } from 'react';

import { getCapabilityStatus } from '@/components/character/capability-status';
import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { ErrorState } from '@/components/fanti/error-state';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { Volume2 } from '@/components/icons';
import { StrokeLearningSurface } from '@/components/study/stroke-learning-surface';
import { EIGHT_STROKES, pickTriple } from '@/components/study/study-content';
import { Button as UiButton } from '@/components/ui/button';
import { CapabilityStatus } from '@/gen/fanti/v1/dictionary_pb';
import { CardMode, StudyService } from '@/gen/fanti/v1/study_pb';
import { useLocale } from '@/i18n/locale';
import { speak } from '@/lib/speak';

const DECK_PAGE_SIZE = 50;
const PAD_SIZE = 300;
const YONG = '永';

/**
 * Stroke practice: deck chips, the 300px 田-grid tracing pad with a ghost
 * glyph, and the 永字八法 stroke-name primer whose rows load single strokes
 * onto the pad.
 */
function StrokesTab() {
  const { t, tGloss, locale } = useLocale();
  const deckQuery = useQuery(StudyService.method.listDueCards, {
    mode: CardMode.CHARACTER,
    pageSize: DECK_PAGE_SIZE,
  });

  const [picked, setPicked] = useState('');

  if (deckQuery.isError) {
    return (
      <ErrorState
        title={t('stStrokesL')}
        description={deckQuery.error.rawMessage}
        onRetry={() => deckQuery.refetch()}
      />
    );
  }
  if (!deckQuery.data) {
    return <Skeleton className="h-80 rounded-xl" />;
  }

  const dueCharacters = deckQuery.data.dueCards
    .map((card) => card.character)
    .filter((character) => character !== undefined);
  const deck = dueCharacters.filter((character) => {
    const glyph = character.glyphs.find(
      (candidate) => candidate.glyph === character.traditional,
    );
    return (
      getCapabilityStatus(glyph?.capabilities, 'strokes') ===
      CapabilityStatus.AVAILABLE
    );
  });
  const glyph = picked || deck[0]?.traditional || YONG;
  const current = deck.find((character) => character.traditional === glyph);

  return (
    <div className="flex flex-col gap-3.5">
      {dueCharacters.length > 0 && deck.length === 0 ? (
        <p role="status" className="text-muted-foreground text-sm">
          {t('noStrokePracticeCards')}
        </p>
      ) : null}
      {deck.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {deck.map((character) => (
            <Chip
              key={character.traditional}
              selected={glyph === character.traditional}
              onClick={() => setPicked(character.traditional)}
              className="min-h-11 min-w-11 px-3 font-display text-lg"
            >
              {character.traditional}
            </Chip>
          ))}
        </div>
      ) : null}

      <Card className="flex flex-col items-center gap-3.5 p-5">
        <div className="flex items-center gap-2.5">
          <span className="font-display text-[26px]">{glyph}</span>
          {current?.pinyin ? (
            <span className="text-md text-muted-foreground">
              {current.pinyin}
            </span>
          ) : null}
          <Button
            variant="ghost"
            aria-label="Pronounce"
            onClick={() => speak(glyph)}
            className="size-9 min-h-9 rounded-full bg-muted text-foreground hover:bg-secondary hover:text-secondary-foreground"
          >
            <Volume2 aria-hidden="true" className="size-[17px]" />
          </Button>
        </div>

        <StrokeLearningSurface
          sizePx={PAD_SIZE}
          practiceAriaLabel={`${t('practiceStrokes')} ${glyph}`}
          glyph={glyph}
          expectedStrokeCount={current?.strokeCount}
        />

        <div className="text-center text-[11px] text-muted-foreground">
          {t('strokeNote')}
        </div>
      </Card>

      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <SectionLabel gloss={tGloss('strokes8T')}>
            {t('strokes8T')}
          </SectionLabel>
          <Button variant="secondary" size="sm" onClick={() => setPicked(YONG)}>
            {t('practiceYong')}
          </Button>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-4">
          <div className="relative flex size-[92px] flex-none items-center justify-center rounded-lg bg-reading-background shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--primary)_28%,transparent)]">
            <div
              aria-hidden="true"
              className="absolute inset-y-1.5 left-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-l border-dashed"
            />
            <div
              aria-hidden="true"
              className="absolute inset-x-1.5 top-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-t border-dashed"
            />
            <span className="relative font-display text-[60px] text-reading-foreground">
              {YONG}
            </span>
          </div>
          <div className="min-w-[200px] flex-1 text-muted-foreground text-sm leading-normal">
            {t('strokes8Intro')}
          </div>
        </div>

        <div className="mt-2 grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-x-5">
          {EIGHT_STROKES.map((stroke) => (
            <UiButton
              variant="unstyled"
              size="unstyled"
              key={stroke.glyph}
              type="button"
              onClick={() => setPicked(stroke.glyph)}
              className="flex w-full min-w-0 cursor-pointer items-center gap-2.5 whitespace-normal rounded-sm border-foreground/7 border-t px-1 pt-2.5 pb-1 text-left transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              <span className="flex size-[30px] flex-none items-center justify-center rounded-md bg-muted font-display text-lg">
                {stroke.glyph}
              </span>
              <span className="whitespace-nowrap font-display font-semibold text-md">
                {stroke.name}
              </span>
              <span className="min-w-0 text-muted-foreground text-sm leading-snug">
                {pickTriple(locale, stroke.description)}
              </span>
            </UiButton>
          ))}
        </div>
      </Card>
    </div>
  );
}

export { StrokesTab };
