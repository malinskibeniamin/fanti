import { useMutation, useQuery } from '@connectrpc/connect-query';
import { createFileRoute, Link } from '@tanstack/react-router';
import { useState } from 'react';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { SectionLabel } from '@/components/fanti/section-label';
import { Skeleton } from '@/components/fanti/skeleton';
import { buttonVariants } from '@/components/ui/button';
import { type Book, LibraryService } from '@/gen/fanti/v1/book_pb';
import { useLocale } from '@/i18n/locale';
import { coverGradient, scriptLabel, toastRpcError } from '@/lib/book-format';
import { saveFile } from '@/lib/save-file';
import { cn } from '@/lib/utils';

export const Route = createFileRoute('/books/$bookId')({
  component: BookDetailPage,
});

const DESCRIPTION_CLAMP = 160;

// Cover-art ink is fixed parchment regardless of theme, verbatim from the
// design's book covers.
// allow: design-token cover art uses the design's fixed cover ink colors
const COVER_INK = 'text-[#fff8e8]';

function BookDetailPage() {
  const { bookId } = Route.useParams();
  const { t } = useLocale();
  const bookQuery = useQuery(LibraryService.method.getBook, {
    name: `books/${bookId}`,
  });

  if (bookQuery.isError) {
    return (
      <ErrorState
        title={t('aboutBook')}
        description={bookQuery.error.message}
        onRetry={() => bookQuery.refetch()}
      />
    );
  }

  if (bookQuery.isPending) {
    return (
      <section className="mx-auto flex max-w-[680px] flex-col gap-4">
        <Skeleton className="h-40 rounded-xl" />
        <Skeleton className="h-11 rounded-xl" />
        <Skeleton className="h-40 rounded-xl" />
      </section>
    );
  }

  return <BookDetail book={bookQuery.data} bookId={bookId} />;
}

function BookDetail({ book, bookId }: { book: Book; bookId: string }) {
  const { t, tGloss } = useLocale();
  const [expanded, setExpanded] = useState(false);

  const downloadMutation = useMutation(LibraryService.method.downloadBook, {
    onSuccess: (resp) => saveFile(resp.filename, resp.data),
    onError: toastRpcError,
  });

  const longDescription = book.description.length > DESCRIPTION_CLAMP;
  const description =
    expanded || !longDescription
      ? book.description
      : `${book.description.slice(0, DESCRIPTION_CLAMP)}…`;

  return (
    <section className="mx-auto flex max-w-[680px] animate-fanti-fade flex-col gap-4">
      <Card className="flex items-start gap-4.5 p-5 shadow-md">
        <div
          className="relative h-37.5 w-28 flex-none rounded-md"
          // Dynamic per-book cover color from the API.
          style={{ background: coverGradient(book.coverColor) }}
        >
          <span
            className={cn(
              '-translate-x-1/2 absolute top-2.5 left-1/2 max-h-[75%] overflow-hidden font-display text-lg tracking-[0.14em] [writing-mode:vertical-rl]',
              COVER_INK,
            )}
          >
            {book.title}
          </span>
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <h1 className="font-display text-2xl leading-tight">{book.title}</h1>
          <div className="text-muted-foreground text-sm">
            {book.titleEnglish}
            {book.titleEnglish && book.author ? ' · ' : ''}
            {book.author}
          </div>
          <div className="mt-1 flex flex-wrap gap-2">
            <DetailPill>{book.fileSizeLabel || 'EPUB3'}</DetailPill>
            <DetailPill className="font-display">
              {scriptLabel(book.script)}
            </DetailPill>
            <DetailPill>
              {book.chapterCount} · {book.charCount.toString()} 字
            </DetailPill>
          </div>
          <div className="mt-1.5">
            <p className="text-sm leading-normal">{description}</p>
            {longDescription ? (
              <Button
                variant="ghost"
                size="sm"
                className="mt-1 px-0 text-accent"
                onClick={() => setExpanded((v) => !v)}
              >
                {expanded ? t('readLessL') : t('readMoreL')}
              </Button>
            ) : null}
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-2 gap-2.5">
        <Link
          to="/read/$bookId"
          params={{ bookId }}
          className={cn(buttonVariants({ size: 'lg' }), 'w-full')}
        >
          {t('readNow')}
        </Link>
        <Button
          variant="secondary"
          size="lg"
          disabled={downloadMutation.isPending}
          onClick={() => downloadMutation.mutate({ name: book.name })}
        >
          {t('dlFree')} · EPUB3
        </Button>
      </div>

      <Card className="px-4 py-3.5">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <div className="flex items-baseline gap-2">
            <span className="font-semibold">EPUB3</span>
            <span className="text-muted-foreground text-xs tabular-nums">
              {book.fileSizeLabel}
            </span>
          </div>
          <span className="whitespace-nowrap text-status-exact text-xs">
            ★ {t('recommendedL')}
          </span>
        </div>
        <SectionLabel className="mt-3">{t('deviceGuide')}</SectionLabel>
        <DeviceRow left="Kindle" right="Send-to-Kindle" />
        <DeviceRow left="Kobo · Nook" right={t('devUsb')} />
        <DeviceRow left={t('devPhone')} right="Apple Books · Calibre …" />
      </Card>

      {book.metadataFields.length > 0 ? (
        <Card className="px-4 py-3.5">
          <SectionLabel gloss={tGloss('aboutBook')}>
            {t('aboutBook')}
          </SectionLabel>
          {book.metadataFields.map((field) => (
            <div
              key={field.label}
              className="grid grid-cols-[120px_1fr] gap-3 py-2.5 shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_7%,transparent)]"
            >
              <span className="text-muted-foreground text-sm">
                {field.label}
              </span>
              <span className="text-sm leading-snug">{field.value}</span>
            </div>
          ))}
        </Card>
      ) : null}
    </section>
  );
}

function DetailPill({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        'whitespace-nowrap rounded-full bg-muted px-2.5 py-0.5 text-[10px] text-muted-foreground tracking-[0.08em]',
        className,
      )}
    >
      {children}
    </span>
  );
}

function DeviceRow({ left, right }: { left: string; right: string }) {
  return (
    <div className="flex justify-between gap-3 py-2.5 text-sm shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_7%,transparent)]">
      <span>{left}</span>
      <span className="text-muted-foreground">{right}</span>
    </div>
  );
}
