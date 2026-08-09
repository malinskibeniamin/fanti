import { useQuery } from '@connectrpc/connect-query';

import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import {
  type Character,
  type CharacterForm,
  CharacterFormStage,
  DictionaryService,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocale } from '@/i18n/locale';

const HISTORICAL_STAGES = [
  {
    stage: CharacterFormStage.ORACLE,
    label: 'historyOracle',
    note: 'historyOracleNote',
    alt: 'historyOracleAlt',
  },
  {
    stage: CharacterFormStage.BRONZE,
    label: 'historyBronze',
    note: 'historyBronzeNote',
    alt: 'historyBronzeAlt',
  },
  {
    stage: CharacterFormStage.SEAL,
    label: 'historySeal',
    note: 'historySealNote',
    alt: 'historySealAlt',
  },
  {
    stage: CharacterFormStage.CLERICAL,
    label: 'historyClerical',
    note: 'historyClericalNote',
    alt: 'historyClericalAlt',
  },
] as const;

interface CharacterHistoryProps {
  character: Character;
}

function CharacterHistory({ character }: CharacterHistoryProps) {
  const { t } = useLocale();
  const historyQuery = useQuery(DictionaryService.method.getCharacterHistory, {
    name: `${character.name}/history`,
  });

  if (historyQuery.isError) {
    return (
      <ErrorState
        title={t('historyError')}
        description={historyQuery.error.rawMessage}
        onRetry={() => historyQuery.refetch()}
      />
    );
  }

  if (historyQuery.isPending || !historyQuery.data) {
    return <Skeleton className="h-64 rounded-xl" />;
  }

  return (
    <Card
      role="region"
      aria-label={t('historyRegion')}
      className="overflow-hidden p-4"
    >
      <div className="overflow-x-auto pb-2">
        <ol className="grid min-w-[640px] grid-cols-5 gap-3">
          {HISTORICAL_STAGES.map((metadata) => {
            const form = historyQuery.data.forms.find(
              (candidate) => candidate.stage === metadata.stage,
            );

            return (
              <li
                key={metadata.stage}
                className="relative flex min-w-0 flex-col gap-2 after:absolute after:top-15 after:-right-2.5 after:text-gold-500 after:content-['→']"
              >
                <HistoryImage
                  form={form}
                  alt={`${character.traditional} ${t(metadata.alt)}`}
                  missingLabel={t('historyMissing')}
                />
                <div>
                  <p className="font-semibold text-sm">{t(metadata.label)}</p>
                  <p className="mt-0.5 text-muted-foreground text-xs leading-normal">
                    {t(metadata.note)}
                  </p>
                </div>
                {form?.sourceUrl ? (
                  <a
                    href={form.sourceUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="w-fit text-accent text-xs underline-offset-3 hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    {t('historySource')}
                  </a>
                ) : null}
              </li>
            );
          })}
          <li className="flex min-w-0 flex-col gap-2">
            <ModernForms character={character} />
            <div>
              <p className="font-semibold text-sm">{t('historyToday')}</p>
              <p className="mt-0.5 text-muted-foreground text-xs leading-normal">
                {t('historyTodayNote')}
              </p>
            </div>
          </li>
        </ol>
      </div>
      <p className="mt-2 text-muted-foreground text-xs">
        {t('historySourceNote')}
      </p>
    </Card>
  );
}

function HistoryImage({
  form,
  alt,
  missingLabel,
}: {
  form?: CharacterForm;
  alt: string;
  missingLabel: string;
}) {
  if (!form?.available || form.svg.length === 0) {
    return (
      <div className="flex h-30 items-center justify-center rounded-lg bg-muted px-2 text-center text-muted-foreground text-xs leading-normal">
        {missingLabel}
      </div>
    );
  }

  const svg = new TextDecoder().decode(form.svg);

  return (
    <div className="flex h-30 items-center justify-center rounded-lg bg-reading-background p-2 shadow-hairline">
      <img
        src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`}
        alt={alt}
        className="size-25 object-contain opacity-85 dark:invert"
      />
    </div>
  );
}

function ModernForms({ character }: { character: Character }) {
  const { t } = useLocale();
  const sameForm = character.traditional === character.simplified;

  return (
    <div className="flex h-30 items-center justify-center gap-2 rounded-lg bg-gold-300/28 px-2 shadow-hairline">
      <ModernGlyph label={sameForm ? t('historyModern') : t('historyTrad')}>
        {character.traditional}
      </ModernGlyph>
      {sameForm ? null : (
        <ModernGlyph label={t('historySimp')}>
          {character.simplified}
        </ModernGlyph>
      )}
    </div>
  );
}

function ModernGlyph({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col items-center gap-1">
      <span className="font-display text-[42px] leading-none">{children}</span>
      <span className="text-[10px] text-muted-foreground">{label}</span>
    </div>
  );
}

export { CharacterHistory };
