import { create } from '@bufbuild/protobuf';
import { useQuery } from '@connectrpc/connect-query';
import { Link } from '@tanstack/react-router';

import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { ProgressBar } from '@/components/fanti/progress-bar';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { ExampleSentences } from '@/components/study/example-sentences';
import { localized, TOPICS } from '@/content/discover';
import { ExampleSentenceSchema } from '@/gen/fanti/v1/common_pb';
import { type SpeakableSentence, StudyService } from '@/gen/fanti/v1/study_pb';
import { useLocale } from '@/i18n/locale';
import { formatCount } from '@/lib/book-format';

/**
 * "What you can say now": the sentences the learner's known characters
 * fully unlock in the real corpus, plus the nearest still-locked
 * sentences with their missing characters linked as the next step.
 */
function SpeakableCard() {
  const { t, tGloss, locale } = useLocale();
  const summaryQuery = useQuery(StudyService.method.getSpeakableSummary, {});

  // Cached data survives a failed background refetch — the card only
  // degrades to an error state when there is nothing to show.
  if (summaryQuery.isError && !summaryQuery.data) {
    return (
      <ErrorState
        title={t('speakTitle')}
        description={summaryQuery.error.rawMessage}
        onRetry={() => summaryQuery.refetch()}
      />
    );
  }
  if (!summaryQuery.data) {
    return <Skeleton className="h-64 rounded-xl" />;
  }

  const summary = summaryQuery.data;
  const topics = TOPICS.filter((topic) => summary.topics.includes(topic.id));

  return (
    <Card className="flex flex-col gap-2.5">
      <SectionLabel gloss={tGloss('speakTitle')}>
        {t('speakTitle')}
      </SectionLabel>

      <div className="flex items-baseline gap-2">
        <span className="font-semibold text-[26px] tabular-nums">
          {summary.unlockedCount}
        </span>
        <span className="text-muted-foreground text-sm">
          / {formatCount(summary.totalCount)} {t('speakUnit')}
        </span>
      </div>
      <ProgressBar
        value={
          summary.totalCount > 0
            ? summary.unlockedCount / summary.totalCount
            : 0
        }
        label={t('speakTitle')}
      />

      {topics.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {topics.map((topic) => (
            <span
              key={topic.id}
              className="whitespace-nowrap rounded-full bg-muted px-2.5 py-[3px] text-[10px] text-muted-foreground tracking-[0.08em]"
            >
              {localized(locale, topic.label)}
            </span>
          ))}
        </div>
      ) : null}

      {summary.sentences.length > 0 ? (
        <ExampleSentences
          examples={summary.sentences.map((sentence) =>
            create(ExampleSentenceSchema, {
              chinese: sentence.traditional,
              english: sentence.english,
            }),
          )}
        />
      ) : (
        <p className="m-0 text-muted-foreground text-sm">{t('speakEmpty')}</p>
      )}

      {summary.almostUnlocked.length > 0 ? (
        <>
          <SectionLabel gloss={tGloss('almostTitle')}>
            {t('almostTitle')}
          </SectionLabel>
          <ul className="m-0 list-none p-0">
            {summary.almostUnlocked.map((sentence) => (
              <li
                key={sentence.id}
                className="flex flex-col gap-1 border-foreground/7 border-t py-3 first:border-t-0"
              >
                <span className="font-reading text-[17px] leading-loose">
                  <AlmostSentence
                    sentence={sentence}
                    learnLabel={t('missingL')}
                  />
                </span>
                <span className="text-muted-foreground text-xs">
                  {sentence.english}
                </span>
              </li>
            ))}
          </ul>
        </>
      ) : null}
    </Card>
  );
}

/**
 * The sentence text with each not-yet-learned character rendered as a
 * link to its character page — the nearest useful next step.
 */
function AlmostSentence({
  sentence,
  learnLabel,
}: {
  sentence: SpeakableSentence;
  learnLabel: string;
}) {
  const missing = new Set(sentence.missingCharacters);

  // Key each glyph by its occurrence count so repeated characters stay
  // distinct without leaning on the array index.
  const seen = new Map<string, number>();
  const glyphs = [...sentence.traditional].map((ch) => {
    const occurrence = (seen.get(ch) ?? 0) + 1;
    seen.set(ch, occurrence);
    return { ch, key: `${ch}#${occurrence}` };
  });

  return (
    <>
      {glyphs.map(({ ch, key }) =>
        missing.has(ch) ? (
          <Link
            key={key}
            to="/characters/$char"
            params={{ char: ch }}
            aria-label={`${learnLabel} ${ch}`}
            className="rounded-xs bg-gold-300/26 text-foreground underline decoration-secondary decoration-dotted underline-offset-4"
          >
            {ch}
          </Link>
        ) : (
          <span key={key} className="text-muted-foreground">
            {ch}
          </span>
        ),
      )}
    </>
  );
}

export { SpeakableCard };
