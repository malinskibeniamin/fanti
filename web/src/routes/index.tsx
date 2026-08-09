import { useQuery } from '@connectrpc/connect-query';
import { createFileRoute, Link } from '@tanstack/react-router';
import { HanziTile } from '@/components/character/hanzi-tile';
import { Card } from '@/components/fanti/card';
import { EmptyState } from '@/components/fanti/empty-state';
import { ErrorState } from '@/components/fanti/error-state';
import { PageHeading } from '@/components/fanti/page-heading';
import { ProgressBar } from '@/components/fanti/progress-bar';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { Plus } from '@/components/icons';
import { buttonVariants } from '@/components/ui/button';
import { type Book, LibraryService } from '@/gen/fanti/v1/book_pb';
import { FileFormat } from '@/gen/fanti/v1/common_pb';
import { StudyService } from '@/gen/fanti/v1/study_pb';
import { useLocale } from '@/i18n/locale';
import { coverGradient, resourceId, scriptChar } from '@/lib/book-format';
import { cn } from '@/lib/utils';

export const Route = createFileRoute('/')({
  component: LibraryPage,
});

const BOOKS_PAGE_SIZE = 100;

// Cover-art ink is fixed parchment/ink regardless of theme, verbatim from
// the design's book covers.
// allow: design-token cover art uses the design's fixed cover ink colors
const COVER_INK = 'text-[#fff8e8]';
const COVER_INK_FADED = 'text-[#fff8e8]/75';

const FILE_FORMAT_SHORT: Record<number, string> = {
  [FileFormat.EPUB]: 'EPUB',
  [FileFormat.TXT]: 'TXT',
  [FileFormat.SRT]: 'SRT',
  [FileFormat.MOBI]: 'MOBI',
};

function LibraryPage() {
  const { t, tGloss } = useLocale();
  const booksQuery = useQuery(LibraryService.method.listBooks, {
    pageSize: BOOKS_PAGE_SIZE,
  });

  let content: React.ReactNode;

  if (booksQuery.isError) {
    content = (
      <ErrorState
        title={t('navLib')}
        description={booksQuery.error.message}
        onRetry={() => booksQuery.refetch()}
      />
    );
  } else if (booksQuery.isPending) {
    content = (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(148px,1fr))] gap-4">
        {['a', 'b', 'c', 'd'].map((key) => (
          <Skeleton key={key} className="aspect-3/4 rounded-lg" />
        ))}
      </div>
    );
  } else {
    const books = booksQuery.data.books;
    const shelf = books.filter((b) => !b.gradedStory);
    const stories = books.filter((b) => b.gradedStory);
    const reading = shelf.find((b) => b.readingProgress > 0);

    content = (
      <>
        <ContinueStudyingCard />
        {reading ? <ContinueReadingCard book={reading} /> : null}

        <div className="grid grid-cols-[repeat(auto-fill,minmax(148px,1fr))] gap-4">
          {shelf.map((book) => (
            <BookTile key={book.name} book={book} />
          ))}
          <Link
            to="/convert"
            className="flex aspect-3/4 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-1.5 border-border border-dashed text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <Plus size={22} aria-hidden />
            <span className="text-sm">{t('addBook')}</span>
            <span className="text-[9px] uppercase tracking-[0.14em] opacity-80">
              {tGloss('addBook')}
            </span>
          </Link>
        </div>

        {shelf.length === 0 ? (
          <EmptyState
            glyph={<span className="font-display text-5xl">書</span>}
            title={t('addBook')}
            description={t('fileDropSub')}
          />
        ) : null}

        {stories.length > 0 ? (
          // allow: nested-card graded-story tiles inside the shelf card match the design
          <Card className="p-4">
            <SectionLabel gloss={tGloss('gradedT')}>
              {t('gradedT')}
            </SectionLabel>
            <p className="mt-1.5 text-muted-foreground text-sm leading-normal">
              {t('gradedSub')}
            </p>
            <div className="mt-3 grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
              {stories.map((story) => (
                <StoryCard key={story.name} story={story} />
              ))}
            </div>
          </Card>
        ) : null}
      </>
    );
  }

  return (
    <section className="flex animate-fanti-fade flex-col gap-5">
      <PageHeading gloss={tGloss('navLib')} title={t('navLib')} />
      {content}
    </section>
  );
}

/** The design's continue-studying hero: today's lesson character. */
function ContinueStudyingCard() {
  const { t, locale } = useLocale();
  const lessonQuery = useQuery(StudyService.method.getLesson, {});
  const profileQuery = useQuery(StudyService.method.getStudyProfile, {
    name: 'studyProfile',
  });

  const next = lessonQuery.data?.nextCharacter;
  if (!next) {
    return null;
  }

  const practiced = profileQuery.data?.practiceDays.length ?? 0;
  const streakNote =
    locale === 'en' ? `${practiced} / 28 days` : `${practiced} / 28 天`;

  return (
    <Card className="flex flex-col items-stretch gap-3.5 p-3.5 min-[380px]:flex-row min-[380px]:items-center">
      <div className="flex min-w-0 items-center gap-3.5">
        <HanziTile glyph={next.traditional} size={54} fontSize={32} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <SectionLabel>{t('todayT')}</SectionLabel>
            <span className="whitespace-nowrap text-[10px] text-muted-foreground tabular-nums">
              {streakNote}
            </span>
          </div>
          <div className="mt-0.5 font-display text-lg">
            {t('meetChar')} · {next.traditional}
          </div>
          <div className="mt-0.5 text-muted-foreground text-xs leading-snug">
            {next.pinyin} · {next.meaning}
          </div>
        </div>
      </div>
      <Link
        to="/study"
        search={{ tab: 'lessons' }}
        className={cn(
          buttonVariants({ variant: 'default' }),
          'w-full bg-accent text-accent-foreground hover:bg-accent/90 min-[380px]:w-auto',
        )}
      >
        {t('startLesson')}
      </Link>
    </Card>
  );
}

function ContinueReadingCard({ book }: { book: Book }) {
  const { t } = useLocale();
  return (
    <Card className="flex items-center gap-3.5 p-3.5 shadow-gold">
      <BookCoverSpine book={book} />
      <div className="min-w-0 flex-1">
        <SectionLabel>{t('contRead')}</SectionLabel>
        <div className="mt-0.5 font-display text-lg">{book.title}</div>
        <ProgressBar
          className="mt-2"
          value={book.readingProgress}
          label={t('contRead')}
        />
        <div className="mt-1 text-muted-foreground text-xs tabular-nums">
          {Math.round(book.readingProgress * 100)}% ·{' '}
          {Math.max(1, Math.round(book.readingProgress * book.chapterCount))} /{' '}
          {book.chapterCount}
        </div>
      </div>
      <Link
        to="/read/$bookId"
        params={{ bookId: resourceId(book.name) }}
        className={cn(buttonVariants({ variant: 'default' }))}
      >
        {t('resume')}
      </Link>
    </Card>
  );
}

function BookCoverSpine({ book }: { book: Book }) {
  return (
    <div
      className="flex h-18 w-13.5 flex-none items-start justify-center rounded-md pt-2"
      // Dynamic per-book cover color from the API.
      style={{ background: coverGradient(book.coverColor) }}
    >
      <span
        className={cn(
          'max-h-15 overflow-hidden font-display text-xs tracking-[0.08em] [writing-mode:vertical-rl]',
          COVER_INK,
        )}
      >
        {book.title}
      </span>
    </div>
  );
}

function BookTile({ book }: { book: Book }) {
  const { locale } = useLocale();
  return (
    <Link
      to="/books/$bookId"
      params={{ bookId: resourceId(book.name) }}
      className="flex cursor-pointer flex-col gap-2 rounded-lg focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <div
        className="relative aspect-3/4 overflow-hidden rounded-lg shadow-sm"
        // Dynamic per-book cover color from the API.
        style={{ background: coverGradient(book.coverColor) }}
      >
        <span
          className={cn(
            'absolute top-2.5 left-2.5 max-h-[78%] overflow-hidden font-display text-xl tracking-[0.14em] [writing-mode:vertical-rl]',
            COVER_INK,
          )}
        >
          {book.title}
        </span>
        <span
          className={cn(
            'absolute top-2 right-2.5 font-semibold text-[9px] uppercase tracking-[0.12em]',
            COVER_INK_FADED,
          )}
        >
          {FILE_FORMAT_SHORT[book.sourceFormat] ?? ''}
        </span>
        <span className="absolute right-2.5 bottom-2.5 flex size-6 items-center justify-center rounded-sm bg-popover/92 font-display text-foreground text-sm">
          {scriptChar(book.script)}
        </span>
      </div>
      <ProgressBar value={book.readingProgress} label={book.title} />
      <div className="flex flex-col gap-0.5">
        <span className="truncate font-medium text-sm">
          {locale === 'en' && book.titleEnglish
            ? book.titleEnglish
            : book.title}
        </span>
        <span className="truncate text-muted-foreground text-xs tabular-nums">
          {book.author}
          {book.author ? ' · ' : ''}
          {book.chapterCount}
        </span>
      </div>
    </Link>
  );
}

function StoryCard({ story }: { story: Book }) {
  const { t, locale } = useLocale();
  return (
    <Link
      to="/read/$bookId"
      params={{ bookId: resourceId(story.name) }}
      className="flex cursor-pointer flex-col gap-2 rounded-lg bg-reading-background p-3.5 shadow-hairline transition-shadow hover:shadow-sm focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <div className="flex items-center justify-between gap-2.5">
        <span className="font-display text-reading-foreground text-xl">
          {story.title}
        </span>
        <span className="whitespace-nowrap rounded-full bg-accent/16 px-2 py-0.5 font-semibold text-[10px] text-status-exact">
          {story.levelLabel}
        </span>
      </div>
      <p className="text-muted-foreground text-sm leading-snug">
        {locale === 'en' && story.titleEnglish
          ? story.titleEnglish
          : story.description}
      </p>
      <div className="mt-0.5 flex items-center justify-between gap-2.5">
        <span className="text-muted-foreground text-xs tabular-nums">
          {story.charCount.toString()} 字
        </span>
        <span className="font-semibold text-accent text-sm">
          {t('readStory')} →
        </span>
      </div>
    </Link>
  );
}
