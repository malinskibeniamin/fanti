import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { useShallow } from 'zustand/react/shallow';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { ErrorState } from '@/components/fanti/error-state';
import { Skeleton } from '@/components/fanti/skeleton';
import { ChevronLeft, ChevronRight, Settings } from '@/components/icons';
import { ChapterView, LibraryService } from '@/gen/fanti/v1/book_pb';
import { useLocale } from '@/i18n/locale';
import { READER_FONT_VAR, useReaderPrefs } from '@/stores/reader-prefs';

import { DictionarySheet } from './dictionary-sheet';
import { PINYIN_CYCLE, PINYIN_LABEL_KEY } from './pinyin';
import { ReaderParagraphs, type TokenTapTarget } from './reader-paragraphs';
import { ReaderSettingsSheet } from './reader-settings-sheet';

/** books/{book}/chapters/{idx} → idx. */
function chapterIndexFromName(name: string): number | null {
  const match = /\/chapters\/(\d+)$/.exec(name);
  return match ? Number.parseInt(match[1], 10) : null;
}

/**
 * Seeded chapter titles usually open with their own numeral heading
 * (第一回 宴桃園豪傑三結義). Split it off so the heading renders as the
 * design's two lines: tracked number over the display title.
 */
const CHAPTER_NUMBER_RE =
  /^(第[0-9〇零一二三四五六七八九十百千两○]{1,4}[章節回篇卷部])[\s、.·：:，]*(.*)$/;

function splitChapterHeading(
  title: string,
  index: number,
): { numberLine: string; titleLine: string } {
  const match = CHAPTER_NUMBER_RE.exec(title.trim());
  if (match?.[2]) {
    return { numberLine: match[1], titleLine: match[2] };
  }
  if (match) {
    return { numberLine: match[1], titleLine: '' };
  }
  return { numberLine: `第 ${index + 1} 章`, titleLine: title };
}

interface ReaderScreenProps {
  bookId: string;
  /** Navigate to the character page for stroke practice. */
  onPracticeStrokes: (glyph: string) => void;
}

/**
 * The reading screen: chapter toolbar (pinyin cycle, settings, chapter
 * stepper), the parchment reading card with ruby-annotated tokens, and
 * the dictionary + settings bottom sheets. Chapter position is saved to
 * the book resource as the reader navigates.
 */
function ReaderScreen({ bookId, onPracticeStrokes }: ReaderScreenProps) {
  const { t } = useLocale();
  const bookName = `books/${bookId}`;
  const queryClient = useQueryClient();
  const transport = useTransport();

  const prefs = useReaderPrefs(
    useShallow((state) => ({
      size: state.size,
      font: state.font,
      pinyin: state.pinyin,
      lineHeight: state.lineHeight,
      traditional: state.traditional,
      setPinyin: state.setPinyin,
    })),
  );

  const [chapterOverride, setChapterOverride] = useState<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [dictTarget, setDictTarget] = useState<TokenTapTarget | null>(null);

  const bookQuery = useQuery(LibraryService.method.getBook, {
    name: bookName,
  });
  const book = bookQuery.data;
  const chapterCount = book?.chapterCount ?? 0;
  const currentIdx =
    chapterOverride ??
    (book ? (chapterIndexFromName(book.currentChapter) ?? 0) : 0);

  const chapterQuery = useQuery(
    LibraryService.method.getChapter,
    {
      name: `${bookName}/chapters/${currentIdx}`,
      view: ChapterView.FULL,
    },
    { enabled: book !== undefined },
  );
  const chapter = chapterQuery.data;

  const updateBookMutation = useMutation(LibraryService.method.updateBook, {
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: LibraryService.method.getBook,
          transport,
          input: { name: bookName },
          cardinality: 'finite',
        }),
      });
    },
    onError: (error) => toast.error(error.rawMessage),
  });

  function goToChapter(index: number) {
    if (chapterCount === 0) {
      return;
    }
    const next = Math.min(chapterCount - 1, Math.max(0, index));
    if (next === currentIdx) {
      return;
    }
    setChapterOverride(next);
    window.scrollTo({ top: 0 });
    updateBookMutation.mutate({
      book: {
        name: bookName,
        currentChapter: `${bookName}/chapters/${next}`,
        readingProgress: (next + 1) / chapterCount,
      },
      updateMask: { paths: ['current_chapter', 'reading_progress'] },
    });
  }

  const chosenScript = chapter
    ? prefs.traditional
      ? chapter.traditionalParagraphs
      : chapter.simplifiedParagraphs
    : [];
  const paragraphs =
    chosenScript.length > 0
      ? chosenScript
      : (chapter?.traditionalParagraphs ?? []);

  if (bookQuery.isError) {
    return (
      <ReaderShell>
        <ErrorState
          title={bookQuery.error.rawMessage}
          onRetry={() => bookQuery.refetch()}
        />
      </ReaderShell>
    );
  }

  return (
    <ReaderShell
      toolbar={
        <>
          <Chip
            selected={prefs.pinyin !== 'off'}
            aria-label="Toggle pinyin hints"
            onClick={() => prefs.setPinyin(PINYIN_CYCLE[prefs.pinyin])}
            className="gap-1.5"
          >
            <span className="font-display">拼</span>
            <span className="text-[11px]">
              {t(PINYIN_LABEL_KEY[prefs.pinyin])}
            </span>
          </Chip>
          <Button
            variant="ghost"
            aria-label="Reader settings"
            onClick={() => setSettingsOpen(true)}
            className="size-10 text-foreground"
          >
            <Settings aria-hidden="true" className="size-5" />
          </Button>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              aria-label="Previous chapter"
              disabled={currentIdx <= 0}
              onClick={() => goToChapter(currentIdx - 1)}
              className="size-10 text-foreground"
            >
              <ChevronLeft aria-hidden="true" className="size-5" />
            </Button>
            <span className="whitespace-nowrap text-muted-foreground text-sm tabular-nums">
              第 {currentIdx + 1} / {chapterCount > 0 ? chapterCount : '—'}
            </span>
            <Button
              variant="ghost"
              aria-label="Next chapter"
              disabled={chapterCount === 0 || currentIdx >= chapterCount - 1}
              onClick={() => goToChapter(currentIdx + 1)}
              className="size-10 text-foreground"
            >
              <ChevronRight aria-hidden="true" className="size-5" />
            </Button>
          </div>
        </>
      }
    >
      <Card className="bg-reading-background px-4 pt-5 pb-7 text-reading-foreground min-[480px]:px-6 min-[480px]:pt-7 min-[480px]:pb-9">
        {chapterQuery.isError ? (
          <ErrorState
            className="bg-transparent shadow-none"
            title={chapterQuery.error.rawMessage}
            onRetry={() => chapterQuery.refetch()}
          />
        ) : chapter ? (
          <>
            <div className="mb-6 text-center">
              {(() => {
                const heading = splitChapterHeading(chapter.title, currentIdx);
                return (
                  <>
                    <div className="font-display text-sm tracking-[0.28em]">
                      {heading.numberLine}
                    </div>
                    {heading.titleLine ? (
                      <div className="mt-2 font-display text-lg leading-(--leading-snug)">
                        {heading.titleLine}
                      </div>
                    ) : null}
                  </>
                );
              })()}
              <div
                aria-hidden="true"
                className="mx-auto mt-4 h-0.5 w-11 bg-secondary"
              />
            </div>
            <ReaderParagraphs
              paragraphs={paragraphs}
              size={prefs.size}
              lineHeight={prefs.lineHeight}
              fontFamily={READER_FONT_VAR[prefs.font]}
              pinyin={prefs.pinyin}
              onTokenTap={setDictTarget}
            />
            <div className="mt-[22px] flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span
                aria-hidden="true"
                className="inline-block size-2 rounded-full bg-[color-mix(in_srgb,var(--gold-300)_70%,transparent)]"
              />
              {t('glowHint')}
            </div>
          </>
        ) : (
          <div className="flex flex-col gap-3">
            <Skeleton className="mx-auto h-5 w-40" />
            <Skeleton className="mt-3 h-5 w-full" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-4/5" />
            <Skeleton className="h-5 w-2/3" />
          </div>
        )}
      </Card>

      <DictionarySheet
        open={dictTarget !== null}
        onClose={() => setDictTarget(null)}
        characterName={dictTarget?.characterName ?? ''}
        sentence={dictTarget?.sentence ?? ''}
        onPracticeStrokes={onPracticeStrokes}
      />
      <ReaderSettingsSheet
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
      />
    </ReaderShell>
  );
}

interface ReaderShellProps {
  toolbar?: React.ReactNode;
  children: React.ReactNode;
}

function ReaderShell({ toolbar, children }: ReaderShellProps) {
  return (
    <section className="mx-auto flex max-w-[680px] animate-fanti-fade flex-col gap-3">
      {toolbar ? (
        <div className="flex items-center justify-end gap-2">{toolbar}</div>
      ) : null}
      {children}
    </section>
  );
}

export { chapterIndexFromName, ReaderScreen, splitChapterHeading };
