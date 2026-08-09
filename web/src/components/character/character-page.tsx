import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import {
  type CapabilityName,
  CapabilityStatusBadge,
  capabilityLabelKey,
  ENTRY_CAPABILITIES,
  GLYPH_CAPABILITIES,
  getCapabilityStatus,
} from '@/components/character/capability-status';
import { CharacterHistory } from '@/components/character/character-history';
import { HanziTile } from '@/components/character/hanzi-tile';
import { RadicalAssembly } from '@/components/character/radical-assembly';
import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { ErrorState } from '@/components/fanti/error-state';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { Volume2 } from '@/components/icons';
import { ExampleSentences } from '@/components/study/example-sentences';
import { StrokeLearningSurface } from '@/components/study/stroke-learning-surface';
import { hskCefrLabel } from '@/components/study/study-content';
import { MappingStatus, Script } from '@/gen/fanti/v1/common_pb';
import {
  CapabilityStatus,
  type Character,
  CharacterCatalogKind,
  type CharacterGlyph,
  DictionaryService,
} from '@/gen/fanti/v1/dictionary_pb';
import { StudyService } from '@/gen/fanti/v1/study_pb';
import { useLocale } from '@/i18n/locale';
import { toastRpcError } from '@/lib/book-format';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

type CharacterTab = 'origin' | 'evolution' | 'calligraphy' | 'sentences';

const PRACTICE_PAD_SIZE = 300;

interface CharacterPageProps {
  char: string;
}

/** The character detail page: header card + origin/evolution/practice/examples. */
export function CharacterPage({ char }: CharacterPageProps) {
  const name = `characters/${char}`;
  const characterQuery = useQuery(DictionaryService.method.getCharacter, {
    name,
  });

  if (characterQuery.isError) {
    return (
      <ErrorState
        title={char}
        description={characterQuery.error.message}
        onRetry={() => characterQuery.refetch()}
      />
    );
  }

  if (characterQuery.isPending) {
    return (
      <section className="mx-auto flex max-w-[680px] flex-col gap-4">
        <Skeleton className="h-44 rounded-xl" />
        <Skeleton className="h-72 rounded-xl" />
      </section>
    );
  }

  return (
    <CharacterDetail
      key={characterQuery.data.name}
      character={characterQuery.data}
    />
  );
}

function CharacterDetail({ character }: { character: Character }) {
  const { t, locale } = useLocale();
  const [tab, setTab] = useState<CharacterTab>('origin');
  const queryClient = useQueryClient();
  const transport = useTransport();

  const addBankMutation = useMutation(StudyService.method.addToDeck, {
    onSuccess: async () => {
      toast.success(t('inBank'));
      await queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: DictionaryService.method.getCharacter,
          transport,
          input: { name: character.name },
          cardinality: 'finite',
        }),
      });
    },
    onError: toastRpcError,
  });

  const statusLabels: Record<number, string> = {
    [MappingStatus.EXACT]: t('legendExact'),
    [MappingStatus.AMBIGUOUS]: t('legendAmb'),
    [MappingStatus.MANUAL]: t('manual'),
  };

  const tabs: { key: CharacterTab; label: string }[] = [
    { key: 'origin', label: t('chOrigin') },
    { key: 'evolution', label: t('chEvolution') },
    { key: 'calligraphy', label: t('chCallig') },
    { key: 'sentences', label: t('chSent') },
  ];

  return (
    <section className="mx-auto flex max-w-[680px] animate-fanti-fade flex-col gap-4">
      <Card className="flex flex-wrap items-center gap-4.5 p-5 shadow-md">
        <HanziTile glyph={character.traditional} size={116} fontSize={72} />
        <div className="flex min-w-[200px] flex-1 flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-lg">
              {character.pinyin || t('capUnavailable')}
            </span>
            {character.zhuyin ? (
              <span className="whitespace-nowrap text-muted-foreground text-sm">
                {character.zhuyin}
              </span>
            ) : null}
            <Button
              variant="ghost"
              size="icon"
              className="size-10 rounded-full bg-muted"
              aria-label={`${t('chOrigin')} ${character.traditional}`}
              onClick={() => speak(character.traditional)}
            >
              <Volume2 size={15} aria-hidden />
            </Button>
            {character.hskLevel > 0 ? (
              <HeaderPill>{hskCefrLabel(character.hskLevel)}</HeaderPill>
            ) : null}
            {character.frequencyRank > 0 ? (
              <HeaderPill className="bg-gold-300/30 text-foreground">
                {locale === 'en' ? 'freq' : '常用'} #{character.frequencyRank}
              </HeaderPill>
            ) : null}
            {character.mappingStatus === MappingStatus.UNSPECIFIED ? null : (
              <HeaderPill>
                {statusLabels[character.mappingStatus] ?? ''}
              </HeaderPill>
            )}
            {character.catalogKind ===
            CharacterCatalogKind.UNSPECIFIED ? null : (
              <HeaderPill
                className={
                  character.catalogKind === CharacterCatalogKind.REFERENCE
                    ? 'bg-gold-300/30 text-foreground'
                    : undefined
                }
              >
                {character.catalogKind === CharacterCatalogKind.REFERENCE
                  ? t('referenceEntry')
                  : t('curriculumEntry')}
              </HeaderPill>
            )}
            <HeaderPill
              className={
                character.learned ? 'bg-accent/16 text-status-exact' : undefined
              }
            >
              {character.learned ? t('learnedTag') : t('newTag')}
            </HeaderPill>
          </div>
          <div className="text-base">
            {character.pos ? (
              <span className="mr-1.5 text-[11px] text-muted-foreground uppercase tracking-[0.1em]">
                {character.pos}
              </span>
            ) : null}
            {character.meaning || t('capUnavailable')}
          </div>
          <div className="flex gap-2 font-reading">
            {character.simplified ? (
              <FormChip label="简" value={character.simplified} />
            ) : null}
            {character.traditional ? (
              <FormChip label="繁" value={character.traditional} />
            ) : null}
          </div>
        </div>
        <Button
          variant="accent"
          disabled={addBankMutation.isPending || character.learned}
          onClick={() => addBankMutation.mutate({ character: character.name })}
        >
          {character.learned ? t('inBank') : t('addBank')}
        </Button>
      </Card>

      <SourceCoverageCard character={character} />
      <div className="grid gap-4 sm:grid-cols-2">
        <DictionaryRecordsCard character={character} />
        <RelatedFormsCard character={character} />
      </div>

      <div
        className="flex max-w-full gap-1 self-start overflow-x-auto rounded-full bg-muted p-0.75"
        role="tablist"
        aria-label={character.traditional}
      >
        {tabs.map(({ key, label }) => (
          <Chip
            key={key}
            id={`character-${key}-tab`}
            role="tab"
            aria-controls="character-tab-panel"
            selected={tab === key}
            className={
              tab === key
                ? 'bg-card text-foreground shadow-xs'
                : 'bg-transparent'
            }
            onClick={() => setTab(key)}
          >
            {label}
          </Chip>
        ))}
      </div>

      <div
        id="character-tab-panel"
        role="tabpanel"
        aria-labelledby={`character-${tab}-tab`}
      >
        <CharacterTabContent tab={tab} character={character} />
      </div>
    </section>
  );
}

function CharacterTabContent({
  tab,
  character,
}: {
  tab: CharacterTab;
  character: Character;
}) {
  switch (tab) {
    case 'origin':
      return <OriginTab character={character} />;
    case 'evolution':
      if (
        glyphCapabilityStatus(character, character.traditional, 'history') ===
        CapabilityStatus.UNAVAILABLE
      ) {
        return <UnavailableCapability descriptionKey="historyUnavailable" />;
      }
      return <CharacterHistory character={character} />;
    case 'calligraphy':
      return <CalligraphyTab character={character} />;
    case 'sentences':
      return <SentencesTab character={character} />;
    default:
      return tab satisfies never;
  }
}

function OriginTab({ character }: { character: Character }) {
  const { t, tGloss } = useLocale();
  return (
    <div className="flex flex-col gap-4">
      <Card className="p-4">
        <SectionLabel gloss={tGloss('chOrigin')}>{t('chOrigin')}</SectionLabel>
        {character.story || character.meaning ? (
          <p className="mt-2 font-reading text-base leading-loose">
            {character.story || character.meaning}
          </p>
        ) : (
          <p className="mt-2 text-muted-foreground text-sm">
            {t('originUnavailable')}
          </p>
        )}
        <p className="mt-2.5 text-muted-foreground text-xs">
          {t('sourcesL')}:{' '}
          <a
            href="https://www.mdbg.net"
            target="_blank"
            rel="noreferrer"
            className="text-accent"
          >
            CC-CEDICT
          </a>{' '}
          ·{' '}
          <a
            href="https://hanziwriter.org"
            target="_blank"
            rel="noreferrer"
            className="text-accent"
          >
            Hanzi Writer
          </a>{' '}
          ·{' '}
          <a
            href="https://github.com/skishore/makemeahanzi"
            target="_blank"
            rel="noreferrer"
            className="text-accent"
          >
            Make Me a Hanzi
          </a>{' '}
          ·{' '}
          <a
            href="https://tatoeba.org"
            target="_blank"
            rel="noreferrer"
            className="text-accent"
          >
            Tatoeba
          </a>{' '}
          · 說文解字
        </p>
      </Card>

      <Card className="p-4">
        <SectionLabel gloss={tGloss('radTitle')}>{t('radTitle')}</SectionLabel>
        <RadicalAssembly
          glyph={character.traditional}
          parts={character.radicalParts}
          status={glyphCapabilityStatus(
            character,
            character.traditional,
            'components',
          )}
        />
        {character.simplificationNote ? (
          <div className="mt-3.5 flex items-center gap-3 border-foreground/7 border-t pt-3">
            <span className="flex size-13 flex-none items-center justify-center rounded-md bg-muted font-display text-[26px]">
              {character.simplified}
            </span>
            <span className="text-muted-foreground text-sm leading-normal">
              {character.simplificationNote}
            </span>
          </div>
        ) : null}
      </Card>

      {character.mnemonic ? (
        <Card className="flex flex-col items-center gap-3 p-4">
          <SectionLabel className="self-start" gloss={tGloss('mnemTitle')}>
            {t('mnemTitle')}
          </SectionLabel>
          <div className="flex size-[170px] items-center justify-center rounded-[14px] bg-gold-300/38">
            <span className="font-display text-[110px] leading-none">
              {character.traditional}
            </span>
          </div>
          <p className="text-center font-reading text-sm leading-normal">
            {character.mnemonic}
          </p>
          <p className="text-center text-muted-foreground text-xs">
            {t('mnemHint')}
          </p>
        </Card>
      ) : null}
    </div>
  );
}

function CalligraphyTab({ character }: { character: Character }) {
  const { t } = useLocale();
  const [form, setForm] = useState<'trad' | 'simp'>('trad');

  const glyph = form === 'simp' ? character.simplified : character.traditional;
  const strokeStatus = glyphCapabilityStatus(character, glyph, 'strokes');

  return (
    <Card className="flex flex-col items-center gap-3.5 p-5">
      {character.simplified !== character.traditional ? (
        <div className="flex gap-2">
          <Chip selected={form === 'trad'} onClick={() => setForm('trad')}>
            繁 {character.traditional}
          </Chip>
          <Chip selected={form === 'simp'} onClick={() => setForm('simp')}>
            简 {character.simplified}
          </Chip>
        </div>
      ) : null}
      {strokeStatus === CapabilityStatus.UNAVAILABLE ||
      strokeStatus === CapabilityStatus.NOT_APPLICABLE ? (
        <div className="flex min-h-48 flex-col items-center justify-center gap-2 text-center">
          <CapabilityStatusBadge status={strokeStatus} />
          <p className="max-w-sm text-muted-foreground text-sm">
            {t('strokeUnavailable')}
          </p>
        </div>
      ) : (
        <>
          <StrokeLearningSurface
            sizePx={PRACTICE_PAD_SIZE}
            practiceAriaLabel={`${t('practiceStrokes')} ${glyph}`}
            glyph={glyph}
            expectedStrokeCount={character.strokeCount}
          />
          <p className="text-center text-muted-foreground text-xs">
            {t('strokeNote')}
          </p>
        </>
      )}
    </Card>
  );
}

function SourceCoverageCard({ character }: { character: Character }) {
  const { t } = useLocale();

  return (
    <Card
      role="region"
      aria-label={t('sourceCoverage')}
      className="flex flex-col gap-3"
    >
      <SectionLabel>{t('sourceCoverage')}</SectionLabel>
      <div className="grid gap-2 sm:grid-cols-2">
        {ENTRY_CAPABILITIES.map((capability) => (
          <CapabilityRow
            key={capability}
            capability={capability}
            status={getCapabilityStatus(
              character.entryCapabilities,
              capability,
            )}
          />
        ))}
      </div>
      {character.glyphs.map((glyph) => (
        <div
          key={glyph.glyph}
          className="flex flex-col gap-2 border-foreground/7 border-t pt-3"
        >
          <span className="font-display text-xl">{glyph.glyph}</span>
          <div className="grid gap-2 sm:grid-cols-3">
            {GLYPH_CAPABILITIES.map((capability) => (
              <CapabilityRow
                key={capability}
                capability={capability}
                status={getCapabilityStatus(glyph.capabilities, capability)}
              />
            ))}
          </div>
        </div>
      ))}
    </Card>
  );
}

function CapabilityRow({
  capability,
  status,
}: {
  capability: CapabilityName;
  status: CapabilityStatus;
}) {
  const { t } = useLocale();

  return (
    <div className="flex flex-col gap-0.5 rounded-lg bg-muted px-3 py-2">
      <span className="text-xs">{t(capabilityLabelKey(capability))}</span>
      <CapabilityStatusBadge status={status} />
    </div>
  );
}

function DictionaryRecordsCard({ character }: { character: Character }) {
  const { t } = useLocale();
  const senses = identifySenses(character.senses);

  return (
    <Card
      role="region"
      aria-label={t('dictionaryRecords')}
      className="flex flex-col gap-3"
    >
      <SectionLabel>{t('dictionaryRecords')}</SectionLabel>
      {senses.length === 0 ? (
        <p className="text-muted-foreground text-sm leading-normal">
          {t('noCedictSense')}
        </p>
      ) : (
        <ol className="flex flex-col gap-2">
          {senses.map(({ id, sense }) => (
            <li
              key={id}
              className="flex flex-col gap-0.5 border-foreground/7 border-t pt-2 first:border-t-0 first:pt-0"
            >
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="font-semibold text-sm">{sense.pinyin}</span>
                {sense.simplified ? (
                  <span className="font-display text-lg">
                    {sense.simplified}
                  </span>
                ) : null}
              </div>
              <p className="text-muted-foreground text-sm leading-normal">
                {sense.definitions.join('; ')}
              </p>
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}

function identifySenses(senses: Character['senses']) {
  const occurrences = new Map<string, number>();
  return senses.map((sense) => {
    const base = `${sense.simplified}-${sense.pinyin}-${sense.definitions.join('\u0000')}`;
    const occurrence = occurrences.get(base) ?? 0;
    occurrences.set(base, occurrence + 1);
    return { id: `${base}-${occurrence}`, sense };
  });
}

function RelatedFormsCard({ character }: { character: Character }) {
  const { t } = useLocale();
  const glyphs =
    character.glyphs.length > 0 ? character.glyphs : fallbackGlyphs(character);

  return (
    <Card
      role="region"
      aria-label={t('relatedForms')}
      className="flex flex-col gap-3"
    >
      <SectionLabel>{t('relatedForms')}</SectionLabel>
      <ul className="flex flex-col gap-2">
        {glyphs.map((glyph) => (
          <li
            key={glyph.glyph}
            className="flex items-center gap-3 rounded-lg bg-muted px-3 py-2"
          >
            <span className="w-8 flex-none text-center font-display text-2xl">
              {glyph.glyph}
            </span>
            <span className="flex min-w-0 flex-col text-xs">
              <span>{scriptLabel(character, glyph, t)}</span>
              {glyph.primary ? (
                <span className="text-muted-foreground">
                  {t('primaryEntry')}
                </span>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

type RelatedGlyph = Pick<CharacterGlyph, 'glyph' | 'primary' | 'scripts'>;

function fallbackGlyphs(character: Character): RelatedGlyph[] {
  const shared = character.traditional === character.simplified;
  return [
    {
      glyph: character.traditional,
      scripts: shared
        ? [Script.TRADITIONAL, Script.SIMPLIFIED]
        : [Script.TRADITIONAL],
      primary: true,
    },
    ...(shared || !character.simplified
      ? []
      : [
          {
            glyph: character.simplified,
            scripts: [Script.SIMPLIFIED],
            primary: false,
          },
        ]),
  ];
}

function scriptLabel(
  character: Character,
  glyph: RelatedGlyph,
  t: ReturnType<typeof useLocale>['t'],
): string {
  const scripts =
    glyph.scripts.length > 0
      ? glyph.scripts
      : glyph.glyph === character.traditional &&
          glyph.glyph === character.simplified
        ? [Script.TRADITIONAL, Script.SIMPLIFIED]
        : glyph.glyph === character.traditional
          ? [Script.TRADITIONAL]
          : glyph.glyph === character.simplified
            ? [Script.SIMPLIFIED]
            : [];
  if (
    scripts.includes(Script.TRADITIONAL) &&
    scripts.includes(Script.SIMPLIFIED)
  ) {
    return t('sharedForm');
  }
  if (scripts.includes(Script.TRADITIONAL)) return t('traditionalForm');
  if (scripts.includes(Script.SIMPLIFIED)) return t('simplifiedForm');
  return t('unclassifiedForm');
}

function glyphCapabilityStatus(
  character: Character,
  glyph: string,
  capability: Extract<CapabilityName, 'strokes' | 'components' | 'history'>,
): CapabilityStatus {
  const metadata =
    character.glyphs.find((candidate) => candidate.glyph === glyph) ??
    character.glyphs.find((candidate) => candidate.primary);
  return getCapabilityStatus(metadata?.capabilities, capability);
}

function UnavailableCapability({
  descriptionKey,
}: {
  descriptionKey: 'historyUnavailable';
}) {
  const { t } = useLocale();

  return (
    <Card className="flex min-h-48 flex-col items-center justify-center gap-2 text-center">
      <CapabilityStatusBadge status={CapabilityStatus.UNAVAILABLE} />
      <p className="max-w-sm text-muted-foreground text-sm">
        {t(descriptionKey)}
      </p>
    </Card>
  );
}

function SentencesTab({ character }: { character: Character }) {
  const { t, tGloss } = useLocale();

  if (character.examples.length === 0) {
    return (
      <Card className="p-4 text-muted-foreground text-sm">
        {t('recordsEmpty')}
      </Card>
    );
  }

  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('chSent')}>{t('chSent')}</SectionLabel>
      <ExampleSentences examples={character.examples} />
    </Card>
  );
}

function HeaderPill({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        'whitespace-nowrap rounded-full bg-muted px-2.5 py-0.5 text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]',
        className,
      )}
    >
      {children}
    </span>
  );
}

function FormChip({ label, value }: { label: string; value: string }) {
  return (
    <span className="rounded-md bg-muted px-2.5 py-1 text-base">
      <span className="mr-1.5 text-[10px] text-muted-foreground">{label}</span>
      {value}
    </span>
  );
}
