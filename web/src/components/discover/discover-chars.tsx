import { useInfiniteQuery, useQuery } from '@connectrpc/connect-query';
import { Link } from '@tanstack/react-router';
import { useState } from 'react';

import {
  type CapabilityName,
  capabilityLabelKey,
  ENTRY_CAPABILITIES,
  GLYPH_CAPABILITIES,
  getCapabilityStatus,
} from '@/components/character/capability-status';
import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { EmptyState } from '@/components/fanti/empty-state';
import { ErrorState } from '@/components/fanti/error-state';
import { ProgressBar } from '@/components/fanti/progress-bar';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { ChevronRight } from '@/components/icons';
import { Input } from '@/components/ui/input';
import { CEFR_MAP, localized, TOPICS } from '@/content/discover';
import {
  type CapabilityCoverage,
  CapabilityStatus,
  type Character,
  CharacterCapability,
  CharacterCatalogKind,
  type CharacterCoverage,
  DictionaryService,
} from '@/gen/fanti/v1/dictionary_pb';
import { useDebouncedCallback } from '@/hooks/use-debounced-callback';
import { type Locale, useLocale } from '@/i18n/locale';
import { resourceId } from '@/lib/book-format';

const PAGE_SIZE = 50;
const SEARCH_DEBOUNCE_MS = 250;

type ScriptChoice = 'trad' | 'simp';
type CatalogChoice = '' | 'curriculum' | 'reference';
type MissingChoice = '' | CapabilityName;

const ALL_CAPABILITIES = [
  ...ENTRY_CAPABILITIES,
  ...GLYPH_CAPABILITIES,
] as const;

/** Discover · Characters: complete catalog coverage, filters, and paging. */
function DiscoverChars() {
  const { t, locale } = useLocale();
  const [searchInput, setSearchInput] = useState('');
  const [query, setQuery] = useState('');
  const [script, setScript] = useState<ScriptChoice>('trad');
  const [catalog, setCatalog] = useState<CatalogChoice>('');
  const [missing, setMissing] = useState<MissingChoice>('');
  const [topic, setTopic] = useState('');
  const debouncedSetQuery = useDebouncedCallback(setQuery, SEARCH_DEBOUNCE_MS);

  const filter = [
    catalog === '' ? '' : `catalog_kind = "${catalog}"`,
    missing === '' ? '' : `missing_capability = "${missing}"`,
    topic === '' ? '' : `topic = "${topic}"`,
  ]
    .filter(Boolean)
    .join(' AND ');

  const listQuery = useInfiniteQuery(
    DictionaryService.method.listCharacters,
    {
      pageSize: PAGE_SIZE,
      pageToken: '',
      query,
      filter,
    },
    {
      pageParamKey: 'pageToken',
      getNextPageParam: (lastPage) =>
        lastPage.nextPageToken === '' ? undefined : lastPage.nextPageToken,
    },
  );

  const rows = listQuery.data?.pages.flatMap((page) => page.characters) ?? [];
  const total = listQuery.data?.pages[0]?.totalSize ?? 0;

  return (
    <div className="flex flex-col gap-4">
      <CatalogCoverageCard />

      <Card className="flex flex-col gap-3 p-3.5">
        <label
          htmlFor="character-catalog-search"
          className="font-semibold text-sm"
        >
          {t('searchCharacters')}
        </label>
        <Input
          id="character-catalog-search"
          type="search"
          value={searchInput}
          placeholder={t('searchPh')}
          onChange={(event) => {
            setSearchInput(event.target.value);
            debouncedSetQuery(event.target.value.trim());
          }}
          className="h-11 bg-card"
        />

        <FilterGroup label={t('displayForm')}>
          <Chip
            selected={script === 'trad'}
            aria-label={`${t('displayForm')}: ${t('traditionalForm')}`}
            onClick={() => setScript('trad')}
            className="min-h-11 px-3 font-display text-base"
          >
            繁
          </Chip>
          <Chip
            selected={script === 'simp'}
            aria-label={`${t('displayForm')}: ${t('simplifiedForm')}`}
            onClick={() => setScript('simp')}
            className="min-h-11 px-3 font-display text-base"
          >
            简
          </Chip>
        </FilterGroup>

        <FilterGroup label={t('catalogFilter')}>
          <Chip
            selected={catalog === ''}
            onClick={() => setCatalog('')}
            className="min-h-11"
          >
            {t('topicAll')}
          </Chip>
          <Chip
            selected={catalog === 'curriculum'}
            onClick={() => setCatalog('curriculum')}
            className="min-h-11"
          >
            {t('curriculum')}
          </Chip>
          <Chip
            selected={catalog === 'reference'}
            onClick={() => setCatalog('reference')}
            className="min-h-11"
          >
            {t('reference')}
          </Chip>
        </FilterGroup>

        <FilterGroup label={t('missingDataFilter')}>
          <Chip
            selected={missing === ''}
            onClick={() => setMissing('')}
            className="min-h-11"
          >
            {t('anyData')}
          </Chip>
          {ALL_CAPABILITIES.map((capability) => (
            <Chip
              key={capability}
              selected={missing === capability}
              onClick={() => setMissing(capability)}
              className="min-h-11"
            >
              {t(capabilityLabelKey(capability))}
            </Chip>
          ))}
        </FilterGroup>

        <FilterGroup label={t('topicFilter')}>
          <Chip
            selected={topic === ''}
            onClick={() => setTopic('')}
            className="min-h-11"
          >
            {t('topicAll')}
          </Chip>
          {TOPICS.map((definition) => (
            <Chip
              key={definition.id}
              selected={topic === definition.id}
              onClick={() =>
                setTopic(topic === definition.id ? '' : definition.id)
              }
              className="min-h-11"
            >
              {localized(locale, definition.label)}
            </Chip>
          ))}
        </FilterGroup>
      </Card>

      {listQuery.isError ? (
        <ErrorState
          title={t('navDc')}
          description={listQuery.error.rawMessage}
          onRetry={() => listQuery.refetch()}
        />
      ) : listQuery.isPending ? (
        <Card className="flex flex-col gap-3 px-4 py-4">
          <Skeleton className="h-16 w-full rounded-lg" />
          <Skeleton className="h-16 w-full rounded-lg" />
          <Skeleton className="h-16 w-full rounded-lg" />
        </Card>
      ) : rows.length === 0 ? (
        <EmptyState title={t('noMatch')} glyph="字" />
      ) : (
        <>
          <Card className="flex flex-col px-4 pt-1.5 pb-2.5">
            {rows.map((row) => (
              <CharacterRow key={row.name} character={row} script={script} />
            ))}
          </Card>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p role="status" className="text-muted-foreground text-sm">
              {resultCountCopy(locale, rows.length, total)}
            </p>
            {listQuery.hasNextPage ? (
              <Button
                variant="outline"
                size="lg"
                disabled={listQuery.isFetchingNextPage}
                onClick={() => listQuery.fetchNextPage()}
              >
                {listQuery.isFetchingNextPage
                  ? t('loadingMore')
                  : t('loadMore')}
              </Button>
            ) : null}
          </div>
        </>
      )}
    </div>
  );
}

function FilterGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <fieldset className="flex flex-wrap items-center gap-2">
      <legend className="float-left mr-1 min-w-24 py-2 font-semibold text-muted-foreground text-xs">
        {label}
      </legend>
      {children}
    </fieldset>
  );
}

function CatalogCoverageCard() {
  const { t, locale } = useLocale();
  const coverageQuery = useQuery(
    DictionaryService.method.getCharacterCoverage,
    { name: 'characterCoverage' },
  );

  if (coverageQuery.isError) {
    return (
      <ErrorState
        title={t('catalogCoverage')}
        description={coverageQuery.error.rawMessage}
        onRetry={() => coverageQuery.refetch()}
      />
    );
  }
  if (coverageQuery.isPending) {
    return <Skeleton className="h-52 w-full rounded-xl" />;
  }

  const coverage = coverageQuery.data;

  return (
    <Card
      role="region"
      aria-label={t('catalogCoverage')}
      className="flex flex-col gap-4 shadow-gold"
    >
      <div>
        <SectionLabel>{t('catalogCoverage')}</SectionLabel>
        <p className="mt-1 text-muted-foreground text-xs">
          {t('catalogCoverageNote')}
        </p>
      </div>
      <dl className="grid grid-cols-2 gap-2 sm:grid-cols-5">
        <CoverageStat
          label={t('catalogEntries')}
          value={coverage.totalEntries}
          locale={locale}
        />
        <CoverageStat
          label={t('relatedGlyphs')}
          value={coverage.totalGlyphs}
          locale={locale}
        />
        <CoverageStat
          label={t('curriculum')}
          value={coverage.curriculumEntries}
          locale={locale}
        />
        <CoverageStat
          label={t('reference')}
          value={coverage.referenceEntries}
          locale={locale}
        />
        <CoverageStat
          label={t('coreCurriculum')}
          value={coverage.coreEntries}
          locale={locale}
          className="col-span-2 sm:col-span-1"
        />
      </dl>
      <CoverageMeters coverage={coverage} />
    </Card>
  );
}

function CoverageStat({
  label,
  value,
  locale,
  className,
}: {
  label: string;
  value: number;
  locale: Locale;
  className?: string;
}) {
  return (
    <div
      className={`flex flex-col-reverse rounded-lg bg-muted px-3 py-2.5 ${className ?? ''}`}
    >
      <dt className="text-muted-foreground text-xs">{label}</dt>
      <dd className="font-semibold text-xl tabular-nums">
        {formatCount(locale, value)}
      </dd>
    </div>
  );
}

function CoverageMeters({ coverage }: { coverage: CharacterCoverage }) {
  const { t, locale } = useLocale();
  const sourceRows = [
    ...coverage.entryCapabilities,
    ...coverage.scripts.flatMap((script) => script.capabilities),
  ];

  const aggregates = ALL_CAPABILITIES.map((capability) => ({
    capability,
    coverage: aggregateCapability(sourceRows, capability),
  })).filter((row) => row.coverage !== undefined);

  if (aggregates.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-2.5 border-foreground/7 border-t pt-3">
      <SectionLabel>{t('sourceAvailability')}</SectionLabel>
      <div className="grid gap-x-4 gap-y-3 sm:grid-cols-2">
        {aggregates.map(({ capability, coverage: row }) => {
          if (!row) return null;
          const applicable = row.available + row.unavailable;

          return (
            <div key={capability} className="flex flex-col gap-1">
              <div className="flex justify-between gap-3 text-xs">
                <span>{t(capabilityLabelKey(capability))}</span>
                <span className="text-muted-foreground tabular-nums">
                  {formatCount(locale, row.unavailable)} {t('sourceGaps')}
                  {row.notApplicable > 0
                    ? ` · ${formatCount(locale, row.notApplicable)} ${t('notApplicableShort')}`
                    : ''}
                </span>
              </div>
              <ProgressBar
                value={applicable === 0 ? 0 : row.available / applicable}
                label={t(capabilityLabelKey(capability))}
                className="h-1"
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function aggregateCapability(
  rows: CapabilityCoverage[],
  capability: CapabilityName,
): CoverageAggregate | undefined {
  const enumValue = capabilityEnum(capability);
  const matches = rows.filter((row) => row.capability === enumValue);
  if (matches.length === 0) return undefined;

  return matches.reduce<CoverageAggregate>(
    (aggregate, row) => ({
      available: aggregate.available + row.available,
      notApplicable: aggregate.notApplicable + row.notApplicable,
      unavailable: aggregate.unavailable + row.unavailable,
    }),
    {
      available: 0,
      notApplicable: 0,
      unavailable: 0,
    },
  );
}

interface CoverageAggregate {
  available: number;
  notApplicable: number;
  unavailable: number;
}

function capabilityEnum(capability: CapabilityName): CharacterCapability {
  switch (capability) {
    case 'reading':
      return CharacterCapability.READING;
    case 'meaning':
      return CharacterCapability.MEANING;
    case 'strokes':
      return CharacterCapability.STROKES;
    case 'components':
      return CharacterCapability.COMPONENTS;
    case 'history':
      return CharacterCapability.HISTORY;
    default:
      return capability satisfies never;
  }
}

function CharacterRow({
  character,
  script,
}: {
  character: Character;
  script: ScriptChoice;
}) {
  const { t, locale } = useLocale();
  const glyph =
    script === 'simp' ? character.simplified : character.traditional;
  const displayedGlyph = glyph || character.traditional;
  const glyphMetadata =
    character.glyphs.find((candidate) => candidate.glyph === displayedGlyph) ??
    character.glyphs.find((candidate) => candidate.primary);
  const missing = [
    ...ENTRY_CAPABILITIES.filter(
      (capability) =>
        getCapabilityStatus(character.entryCapabilities, capability) ===
        CapabilityStatus.UNAVAILABLE,
    ),
    ...GLYPH_CAPABILITIES.filter(
      (capability) =>
        getCapabilityStatus(glyphMetadata?.capabilities, capability) ===
        CapabilityStatus.UNAVAILABLE,
    ),
  ];
  const levelLabel =
    character.hskLevel > 0
      ? `HSK ${character.hskLevel} · ${CEFR_MAP[character.hskLevel] ?? ''}`
      : character.frequencyRank > 0
        ? `${locale === 'en' ? 'freq' : '常用'} #${character.frequencyRank}`
        : character.catalogKind === CharacterCatalogKind.CURRICULUM
          ? t('lowFrequency')
          : '';
  const catalogLabel =
    character.catalogKind === CharacterCatalogKind.REFERENCE
      ? t('reference')
      : character.catalogKind === CharacterCatalogKind.CURRICULUM
        ? t('curriculum')
        : '';
  const topicsLabel = character.topics
    .map((topicId) => {
      const definition = TOPICS.find((candidate) => candidate.id === topicId);
      return definition ? localized(locale, definition.label) : topicId;
    })
    .join(' · ');
  const metadata = [catalogLabel, levelLabel, topicsLabel]
    .filter(Boolean)
    .join(' · ');
  const summary =
    [character.pinyin, character.meaning].filter(Boolean).join(' · ') ||
    t('capUnavailable');

  return (
    <Link
      to="/characters/$char"
      params={{ char: resourceId(character.name) }}
      className="flex min-h-16 items-center gap-3 rounded-sm py-3 shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_7%,transparent)] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <span className="w-11 flex-none text-center font-display text-[28px] leading-[1.2]">
        {displayedGlyph}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm">
          {summary}
        </span>
        <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[10px] text-muted-foreground tracking-[0.06em]">
          {metadata}
        </span>
        {missing.length > 0 ? (
          <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[11px] text-muted-foreground">
            {missingCopy(
              locale,
              missing.map((capability) =>
                t(capabilityLabelKey(capability)).toLocaleLowerCase(),
              ),
            )}
          </span>
        ) : null}
      </span>
      <span
        className={
          character.learned
            ? 'flex-none rounded-full bg-[color-mix(in_srgb,var(--jade-600)_16%,transparent)] px-2.5 py-[3px] font-semibold text-[10px] text-status-exact'
            : 'flex-none rounded-full bg-muted px-2.5 py-[3px] font-semibold text-[10px] text-muted-foreground'
        }
      >
        {character.learned ? t('learnedTag') : t('newTagDc')}
      </span>
      <ChevronRight
        aria-hidden="true"
        className="size-4 flex-none text-muted-foreground"
      />
    </Link>
  );
}

function formatCount(locale: Locale, value: number): string {
  const language =
    locale === 'en' ? 'en-US' : locale === 'tc' ? 'zh-TW' : 'zh-CN';
  return value.toLocaleString(language);
}

function resultCountCopy(locale: Locale, shown: number, total: number): string {
  const shownText = formatCount(locale, shown);
  const totalText = formatCount(locale, total);
  if (locale === 'en') {
    return `Showing ${shownText} of ${totalText} characters`;
  }
  if (locale === 'tc') return `已顯示 ${shownText}／${totalText} 個字`;
  return `已显示 ${shownText}/${totalText} 个字`;
}

function missingCopy(locale: Locale, labels: string[]): string {
  if (locale === 'en') return `Missing: ${labels.join(', ')}`;
  if (locale === 'tc') return `缺漏：${labels.join('、')}`;
  return `缺漏：${labels.join('、')}`;
}

export { DiscoverChars };
