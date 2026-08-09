import { useQuery } from '@connectrpc/connect-query';
import { Link } from '@tanstack/react-router';

import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { ChevronRight } from '@/components/icons';
import { hskCefrLabel, knownGoal } from '@/components/study/study-content';
import {
  type Character,
  type Compound,
  DictionaryService,
} from '@/gen/fanti/v1/dictionary_pb';
import { Goal, StudyService } from '@/gen/fanti/v1/study_pb';
import { type Locale, useLocale } from '@/i18n/locale';
import { resourceId } from '@/lib/book-format';
import { cn } from '@/lib/utils';

const CHARACTER_PAGE_SIZE = 200;
const COMPOUND_PAGE_SIZE = 50;
const UNRANKED = Number.MAX_SAFE_INTEGER;

function weakLabel(locale: Locale, count: number): string {
  return locale === 'en'
    ? `weak · ${count}`
    : locale === 'tc'
      ? `弱點 · ${count}`
      : `弱点 · ${count}`;
}

function frequencyLabel(locale: Locale, rank: number): string {
  const word = locale === 'en' ? 'freq' : '常用';
  return `${word} #${rank > 0 ? rank : '—'}`;
}

function sortForGoal(characters: Character[], goal: Goal): Character[] {
  const sorted = [...characters];
  if (goal === Goal.EXAM) {
    sorted.sort(
      (a, b) =>
        (a.hskLevel > 0 ? a.hskLevel : UNRANKED) -
        (b.hskLevel > 0 ? b.hskLevel : UNRANKED),
    );
  } else if (knownGoal(goal) === Goal.PRACTICAL) {
    sorted.sort(
      (a, b) =>
        (a.frequencyRank > 0 ? a.frequencyRank : UNRANKED) -
        (b.frequencyRank > 0 ? b.frequencyRank : UNRANKED),
    );
  }
  // READING keeps curriculum order.
  return sorted;
}

/**
 * The learner's dictionary: every deck character with learned/new state and
 * weak badges, the components met so far, and compound words that unlock as
 * their parts are learned.
 */
function DictTab() {
  const { t, tGloss, locale } = useLocale();
  const profileQuery = useQuery(StudyService.method.getStudyProfile, {
    name: 'studyProfile',
  });
  const charactersQuery = useQuery(DictionaryService.method.listCharacters, {
    pageSize: CHARACTER_PAGE_SIZE,
  });
  const compoundsQuery = useQuery(DictionaryService.method.listCompounds, {
    pageSize: COMPOUND_PAGE_SIZE,
  });

  if (charactersQuery.isError) {
    return (
      <ErrorState
        title={t('stDictL')}
        description={charactersQuery.error.rawMessage}
        onRetry={() => charactersQuery.refetch()}
      />
    );
  }
  if (!charactersQuery.data) {
    return <Skeleton className="h-80 rounded-xl" />;
  }

  const goal = profileQuery.data?.goal ?? Goal.PRACTICAL;
  const characters = sortForGoal(charactersQuery.data.characters, goal);
  const learned = characters.filter((character) => character.learned);
  const learnedGlyphs = new Set(
    learned.map((character) => character.traditional),
  );

  const parts = new Map<string, string>();
  for (const character of learned) {
    for (const radical of character.radicalParts) {
      if (!parts.has(radical.part)) {
        parts.set(radical.part, radical.note);
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="flex flex-col px-4 pt-1.5 pb-2.5">
        {characters.map((character) => (
          <Link
            key={character.traditional}
            to="/characters/$char"
            params={{ char: character.traditional }}
            className="flex items-center gap-3 border-foreground/7 border-t py-3 first:border-t-0 focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <span className="w-11 flex-none text-center font-display text-[28px] leading-tight">
              {character.traditional}
            </span>
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm">
                {character.pinyin} · {character.meaning}
              </span>
              <span className="whitespace-nowrap text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]">
                {hskCefrLabel(character.hskLevel) ||
                  (character.frequencyRank > 0
                    ? `${locale === 'en' ? 'freq' : '常用'} #${character.frequencyRank}`
                    : t('properTag'))}
                {knownGoal(goal) === Goal.PRACTICAL
                  ? ` · ${frequencyLabel(locale, character.frequencyRank)}`
                  : ''}
              </span>
            </span>
            {character.mistakeCount > 0 ? (
              <span className="flex-none whitespace-nowrap rounded-full bg-primary/12 px-2 py-[3px] font-semibold text-[10px] text-status-manual tabular-nums">
                {weakLabel(locale, character.mistakeCount)}
              </span>
            ) : null}
            <span
              className={cn(
                'flex-none whitespace-nowrap rounded-full px-2 py-[3px] font-semibold text-[10px]',
                character.learned
                  ? 'bg-accent/16 text-status-exact'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {character.learned ? t('learnedTag') : t('newTag')}
            </span>
            <span aria-hidden="true" className="flex text-muted-foreground">
              <ChevronRight className="size-4" />
            </span>
          </Link>
        ))}
      </Card>

      {parts.size > 0 ? (
        <Card className="flex flex-col gap-2.5 px-4 py-3.5">
          <SectionLabel gloss={tGloss('partsTitle')}>
            {t('partsTitle')}
          </SectionLabel>
          <div className="flex flex-wrap gap-2">
            {Array.from(parts, ([part, note]) => (
              <span
                key={part}
                title={note}
                className="flex size-10 items-center justify-center rounded-md bg-muted font-display text-xl"
              >
                {part}
              </span>
            ))}
          </div>
        </Card>
      ) : null}

      {compoundsQuery.data && compoundsQuery.data.compounds.length > 0 ? (
        <CompoundsCard
          compounds={compoundsQuery.data.compounds}
          unlockedCount={compoundsQuery.data.unlockedCount}
          totalSize={compoundsQuery.data.totalSize}
          learnedGlyphs={learnedGlyphs}
        />
      ) : null}
    </div>
  );
}

function CompoundsCard({
  compounds,
  unlockedCount,
  totalSize,
  learnedGlyphs,
}: {
  compounds: Compound[];
  unlockedCount: number;
  totalSize: number;
  learnedGlyphs: Set<string>;
}) {
  const { t, tGloss } = useLocale();

  return (
    <Card className="px-4 py-3.5">
      <div className="flex items-center justify-between gap-3">
        <SectionLabel gloss={tGloss('comboTitle')}>
          {t('comboTitle')}
        </SectionLabel>
        <span className="whitespace-nowrap text-[11px] text-muted-foreground tabular-nums">
          {unlockedCount} / {totalSize}
        </span>
      </div>
      {compounds.map((compound) => {
        const componentGlyphs = compound.characters.map(resourceId);
        const missing = componentGlyphs.filter(
          (glyph) => !learnedGlyphs.has(glyph),
        );
        return (
          <div
            key={compound.word}
            className="flex items-center gap-3 border-foreground/7 border-t pt-3 pb-1.5 first:border-t-0"
          >
            <span className="min-w-[60px] flex-none font-display text-2xl">
              {compound.word}
            </span>
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <div className="flex items-center gap-1">
                {componentGlyphs.map((glyph) => (
                  <span
                    key={glyph}
                    className={cn(
                      'flex size-6 items-center justify-center rounded-md font-display text-sm',
                      learnedGlyphs.has(glyph)
                        ? 'bg-accent/16 text-status-exact'
                        : 'bg-muted text-muted-foreground',
                    )}
                  >
                    {glyph}
                  </span>
                ))}
                <span className="text-[11px] text-muted-foreground">
                  {compound.pinyin}
                </span>
              </div>
              <span className="text-muted-foreground text-sm leading-snug">
                {compound.gloss}
              </span>
            </div>
            <span
              className={cn(
                'flex-none whitespace-nowrap rounded-full px-2 py-[3px] font-semibold text-[10px]',
                compound.unlocked
                  ? 'bg-accent/16 text-status-exact'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {compound.unlocked
                ? t('comboUnlockedL')
                : `${t('comboLearnL')} ${missing.join(' ')}`}
            </span>
          </div>
        );
      })}
    </Card>
  );
}

export { DictTab };
