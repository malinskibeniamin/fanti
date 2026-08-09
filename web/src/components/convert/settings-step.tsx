import { useMutation } from '@connectrpc/connect-query';
import { useRef, useState } from 'react';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Chip } from '@/components/fanti/chip';
import { SectionLabel } from '@/components/fanti/section-label';
import { FileText, X } from '@/components/icons';
import { Button as UiButton } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { ConversionDirection, FileFormat } from '@/gen/fanti/v1/common_pb';
import {
  type Conversion,
  ConversionService,
} from '@/gen/fanti/v1/conversion_pb';
import { useDebouncedCallback } from '@/hooks/use-debounced-callback';
import { useLocale } from '@/i18n/locale';
import { toastRpcError } from '@/lib/book-format';
import { cn } from '@/lib/utils';

interface SettingsStepProps {
  conversion: Conversion;
  onRun: () => void;
  onReset: () => void;
}

// The design's five cover colors (lacquer, jade, bronze, ink-brown, gold).
// allow: design-token cover palette is content, not chrome, per the design
const COVER_SWATCHES = ['#8f1d18', '#2f7a62', '#b87333', '#5a4630', '#cf9f37'];
// allow: design-token cover ink is fixed parchment per the design
const COVER_INK = 'text-[#fff8e8]';
const COVER_INK_FADED = 'text-[#fff8e8]/85';

const CHAPTER_REGEX_DISPLAY =
  '^\\s*(?:第?[0-9一二三四五六七八九十百千零两]{1,4}[章節回篇卷部]|简介|序|楔子|尾声|后记|番外)[　 ]{0,10}.*$  (i)';
const LAYOUT_DEBOUNCE_MS = 500;

const FORMAT_LABEL: Record<number, string> = {
  [FileFormat.EPUB]: 'EPUB',
  [FileFormat.TXT]: 'TXT',
  [FileFormat.SRT]: 'SRT',
  [FileFormat.MOBI]: 'MOBI',
};

type LayoutEditor = 'cover' | 'toc' | 'front';

interface TocRow {
  id: string;
  title: string;
}

/** Plain local mirror of the layout message for controlled editing. */
interface LayoutDraft {
  title: string;
  author: string;
  coverColor: string;
  titleFont: string;
  bodyFont: string;
  chapters: TocRow[];
  frontMatter: string;
  indentFirstLine: boolean;
}

interface SettingsDraft {
  direction: ConversionDirection;
  localizeVocabulary: boolean;
  convertPunctuation: boolean;
}

function draftFromConversion(conversion: Conversion): LayoutDraft {
  const layout = conversion.layout;
  return {
    title: layout?.title ?? '',
    author: layout?.author ?? '',
    coverColor: layout?.coverColor ?? COVER_SWATCHES[0],
    titleFont: layout?.titleFont ?? 'kai',
    bodyFont: layout?.bodyFont ?? 'serif',
    chapters: (layout?.chapterTitles ?? []).map((title) => ({
      id: crypto.randomUUID(),
      title,
    })),
    frontMatter: layout?.frontMatter ?? '',
    indentFirstLine: layout?.indentFirstLine ?? true,
  };
}

/** Step 2 — direction, chapter detection, and book layout editors. */
export function SettingsStep({
  conversion,
  onRun,
  onReset,
}: SettingsStepProps) {
  const { t, tGloss } = useLocale();
  const [editor, setEditor] = useState<LayoutEditor>('cover');
  const [showPattern, setShowPattern] = useState(false);
  const [draft, setDraft] = useState<LayoutDraft>(() =>
    draftFromConversion(conversion),
  );
  const draftRef = useRef(draft);
  const [settingsDraft, setSettingsDraft] = useState<SettingsDraft>(() => ({
    direction: conversion.settings?.direction ?? ConversionDirection.S2T,
    localizeVocabulary: conversion.settings?.localizeVocabulary ?? true,
    convertPunctuation: conversion.settings?.convertPunctuation ?? true,
  }));
  const settingsDraftRef = useRef(settingsDraft);

  const updateMutation = useMutation(ConversionService.method.updateConversion);
  const saveChainRef = useRef(Promise.resolve(true));

  function enqueueSave(save: () => Promise<unknown>): Promise<boolean> {
    const queued = saveChainRef.current.then(async () => {
      try {
        await save();

        return true;
      } catch (error) {
        toastRpcError(error);

        return false;
      }
    });
    saveChainRef.current = queued;

    return queued;
  }

  const pushLayout = useDebouncedCallback((next: LayoutDraft) => {
    return enqueueSave(() =>
      updateMutation.mutateAsync({
        conversion: {
          name: conversion.name,
          layout: {
            title: next.title,
            author: next.author,
            coverColor: next.coverColor,
            titleFont: next.titleFont,
            bodyFont: next.bodyFont,
            chapterTitles: next.chapters.map((row) => row.title),
            frontMatter: next.frontMatter,
            indentFirstLine: next.indentFirstLine,
          },
        },
        updateMask: { paths: ['layout'] },
      }),
    );
  }, LAYOUT_DEBOUNCE_MS);

  function patchDraft(patch: Partial<LayoutDraft>) {
    const next = { ...draftRef.current, ...patch };
    draftRef.current = next;
    setDraft(next);
    pushLayout(next);
  }

  function patchSettings(patch: Partial<SettingsDraft>) {
    const next = { ...settingsDraftRef.current, ...patch };
    settingsDraftRef.current = next;
    setSettingsDraft(next);
    void enqueueSave(() =>
      updateMutation.mutateAsync({
        conversion: {
          name: conversion.name,
          settings: next,
        },
        updateMask: { paths: ['settings'] },
      }),
    );
  }

  async function runAfterSaving() {
    // Let already-started writes finish, then make one authoritative save of
    // both local drafts. This also retries state after a transient background
    // save failure and cannot race the conversion start.
    pushLayout.cancel();
    await saveChainRef.current;
    const saved = await enqueueSave(() =>
      updateMutation.mutateAsync({
        conversion: {
          name: conversion.name,
          settings: settingsDraftRef.current,
          layout: {
            title: draftRef.current.title,
            author: draftRef.current.author,
            coverColor: draftRef.current.coverColor,
            titleFont: draftRef.current.titleFont,
            bodyFont: draftRef.current.bodyFont,
            chapterTitles: draftRef.current.chapters.map((row) => row.title),
            frontMatter: draftRef.current.frontMatter,
            indentFirstLine: draftRef.current.indentFirstLine,
          },
        },
        updateMask: { paths: ['settings', 'layout'] },
      }),
    );
    if (saved) {
      onRun();
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="flex items-center gap-3 px-4 py-3.5">
        <span className="flex size-10 flex-none items-center justify-center rounded-md bg-muted text-muted-foreground">
          <FileText size={20} aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium">{conversion.filename}</div>
          <div className="text-muted-foreground text-sm tabular-nums">
            {conversion.charCount.toString()} 字 ·{' '}
            {FORMAT_LABEL[conversion.format] ?? ''} · {draft.chapters.length}{' '}
            {t('chTitle')}
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('clear')}
          onClick={onReset}
        >
          <X size={18} aria-hidden />
        </Button>
      </Card>

      <Card className="p-4">
        <SectionLabel gloss={tGloss('dirTitle')}>{t('dirTitle')}</SectionLabel>
        <div className="mt-2.5 grid grid-cols-2 gap-2.5">
          <DirectionButton
            selected={settingsDraft.direction === ConversionDirection.S2T}
            from="简"
            to="繁"
            sub="Simplified → Traditional"
            onClick={() =>
              patchSettings({ direction: ConversionDirection.S2T })
            }
          />
          <DirectionButton
            selected={settingsDraft.direction === ConversionDirection.T2S}
            from="繁"
            to="简"
            sub="Traditional → Simplified"
            onClick={() =>
              patchSettings({ direction: ConversionDirection.T2S })
            }
          />
        </div>
        <p className="mt-3 text-muted-foreground text-xs leading-normal">
          {t('quirkNote')}
        </p>
        <div className="mt-3 flex flex-col gap-2.5">
          <div className="flex items-center gap-2.5 text-sm">
            <Switch
              aria-label={t('localizeL')}
              checked={settingsDraft.localizeVocabulary}
              onCheckedChange={(checked) =>
                patchSettings({ localizeVocabulary: checked })
              }
            />
            {t('localizeL')}
          </div>
          <div className="flex items-center gap-2.5 text-sm">
            <Switch
              aria-label={t('punctL')}
              checked={settingsDraft.convertPunctuation}
              onCheckedChange={(checked) =>
                patchSettings({ convertPunctuation: checked })
              }
            />
            {t('punctL')}
          </div>
        </div>
      </Card>

      <Card className="p-4">
        <SectionLabel gloss={tGloss('chTitle')}>{t('chTitle')}</SectionLabel>
        <div className="mt-2.5 flex flex-wrap gap-2">
          {conversion.unitCounts.map((unit) => (
            <span
              key={unit.unit}
              className={cn(
                'rounded-full px-3 py-1 text-sm tabular-nums',
                unit.count > 0
                  ? 'bg-accent/16 font-semibold text-status-exact'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {unit.unit} {unit.count}
            </span>
          ))}
        </div>
        <p className="mt-2.5 text-muted-foreground text-sm leading-normal">
          {t('chNote')}
        </p>
        <Button
          variant="ghost"
          size="sm"
          className="mt-2 px-0 text-accent"
          onClick={() => setShowPattern((v) => !v)}
        >
          {showPattern ? t('patHide') : t('patShow')}
        </Button>
        {showPattern ? (
          <pre className="mt-2.5 overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 font-mono text-muted-foreground text-xs leading-relaxed">
            {CHAPTER_REGEX_DISPLAY}
          </pre>
        ) : null}
      </Card>

      <Card className="p-4">
        <div className="flex items-center justify-between">
          <SectionLabel gloss={tGloss('layoutTitle')}>
            {t('layoutTitle')}
          </SectionLabel>
          <span className="text-muted-foreground text-xs">{t('editHint')}</span>
        </div>

        <div className="mt-3.5 grid grid-cols-3 items-start gap-2 min-[480px]:flex min-[480px]:flex-wrap min-[480px]:justify-center min-[480px]:gap-4">
          <PagePreview
            label={t('cover')}
            selected={editor === 'cover'}
            onSelect={() => setEditor('cover')}
          >
            <div
              className="relative size-full rounded-md"
              // Dynamic user-chosen cover color.
              style={{ background: draft.coverColor }}
            >
              <span
                className={cn(
                  '-translate-x-1/2 absolute top-2 left-1/2 max-h-[70%] overflow-hidden font-display text-[11px] tracking-[0.1em] [writing-mode:vertical-rl]',
                  COVER_INK,
                )}
              >
                {draft.title}
              </span>
              <span
                className={cn(
                  'absolute right-0 bottom-2 left-0 text-center font-display text-[9px]',
                  COVER_INK_FADED,
                )}
              >
                {draft.author}
              </span>
            </div>
          </PagePreview>
          <PagePreview
            label={t('pgToc')}
            selected={editor === 'toc'}
            onSelect={() => setEditor('toc')}
          >
            <div className="size-full overflow-hidden rounded-md bg-reading-background p-2.5 text-reading-foreground">
              <div className="mb-2 text-center font-display text-[10px]">
                目錄
              </div>
              {draft.chapters.slice(0, 6).map((row) => (
                <div
                  key={row.id}
                  className="mb-1 truncate font-reading text-[7px] text-muted-foreground"
                >
                  {row.title}
                </div>
              ))}
            </div>
          </PagePreview>
          <PagePreview
            label={t('pgFirst')}
            selected={editor === 'front'}
            onSelect={() => setEditor('front')}
          >
            <div className="flex size-full flex-col gap-1.5 overflow-hidden rounded-md bg-reading-background p-2.5 text-reading-foreground">
              <div className="text-center font-display text-[10px]">
                {t('pgFirst')}
              </div>
              <div className="line-clamp-4 font-reading text-[7px] leading-relaxed">
                {draft.frontMatter}
              </div>
              <div className="h-0.75 rounded-xs bg-reading-foreground/10" />
              <div className="h-0.75 w-5/6 rounded-xs bg-reading-foreground/10" />
            </div>
          </PagePreview>
        </div>

        <div className="mt-3.5 border-foreground/7 border-t pt-3.5">
          {editor === 'cover' ? (
            <div className="flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-2.5">
                <label
                  htmlFor="cv-title"
                  className="flex flex-col gap-1.5 text-sm"
                >
                  <span className="font-medium">{t('bkTitle')}</span>
                  <Input
                    id="cv-title"
                    value={draft.title}
                    onChange={(e) => patchDraft({ title: e.target.value })}
                  />
                </label>
                <label
                  htmlFor="cv-author"
                  className="flex flex-col gap-1.5 text-sm"
                >
                  <span className="font-medium">{t('bkAuthor')}</span>
                  <Input
                    id="cv-author"
                    value={draft.author}
                    onChange={(e) => patchDraft({ author: e.target.value })}
                  />
                </label>
              </div>
              <div>
                <div className="mb-2 font-medium text-sm">{t('cover')}</div>
                <div className="flex flex-wrap items-center gap-2.5">
                  {COVER_SWATCHES.map((color) => (
                    <UiButton
                      variant="unstyled"
                      size="unstyled"
                      key={color}
                      type="button"
                      aria-label={`${t('cover')} ${color}`}
                      onClick={() => patchDraft({ coverColor: color })}
                      className={cn(
                        'size-8.5 rounded-full transition-shadow',
                        draft.coverColor === color
                          ? 'ring-2 ring-secondary ring-offset-2 ring-offset-card'
                          : 'shadow-xs',
                      )}
                      // Palette swatch — the color is the content.
                      style={{ background: color }}
                    />
                  ))}
                  <span className="text-muted-foreground text-xs">
                    {t('coverHint')}
                  </span>
                </div>
              </div>
              <FontChips
                label={t('titleFont')}
                value={draft.titleFont}
                onChange={(font) => patchDraft({ titleFont: font })}
              />
              <p className="text-muted-foreground text-xs leading-normal">
                {t('fontNote')}
              </p>
            </div>
          ) : null}

          {editor === 'toc' ? (
            <div className="flex flex-col gap-2">
              {draft.chapters.map((row, index) => (
                <div key={row.id} className="flex items-center gap-2.5">
                  <span className="w-6 flex-none text-right text-muted-foreground text-sm tabular-nums">
                    {index + 1}
                  </span>
                  <Input
                    value={row.title}
                    onChange={(e) =>
                      patchDraft({
                        chapters: draft.chapters.map((r) =>
                          r.id === row.id ? { ...r, title: e.target.value } : r,
                        ),
                      })
                    }
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`${t('clear')} ${index + 1}`}
                    onClick={() =>
                      patchDraft({
                        chapters: draft.chapters.filter((r) => r.id !== row.id),
                      })
                    }
                  >
                    <X size={15} aria-hidden />
                  </Button>
                </div>
              ))}
              <Button
                variant="ghost"
                size="sm"
                className="self-start"
                onClick={() =>
                  patchDraft({
                    chapters: [
                      ...draft.chapters,
                      { id: crypto.randomUUID(), title: '' },
                    ],
                  })
                }
              >
                ＋ {t('addCh')}
              </Button>
            </div>
          ) : null}

          {editor === 'front' ? (
            <div className="flex flex-col gap-3">
              <label
                htmlFor="cv-front"
                className="flex flex-col gap-1.5 text-sm"
              >
                <span className="font-medium">{t('pgFirst')}</span>
                <Textarea
                  id="cv-front"
                  rows={3}
                  value={draft.frontMatter}
                  onChange={(e) => patchDraft({ frontMatter: e.target.value })}
                />
                <span className="text-muted-foreground text-xs">
                  {t('frontNote')}
                </span>
              </label>
              <FontChips
                label={t('bodyFont')}
                value={draft.bodyFont}
                onChange={(font) => patchDraft({ bodyFont: font })}
              />
              <div className="flex items-center gap-2.5 text-sm">
                <Switch
                  aria-label={t('indent')}
                  checked={draft.indentFirstLine}
                  onCheckedChange={(checked) =>
                    patchDraft({ indentFirstLine: checked })
                  }
                />
                {t('indent')}
              </div>
            </div>
          ) : null}
        </div>
      </Card>

      <Button
        size="lg"
        className="w-full"
        onClick={() => void runAfterSaving()}
      >
        {t('startCv')}
      </Button>
    </div>
  );
}

function DirectionButton({
  selected,
  from,
  to,
  sub,
  onClick,
}: {
  selected: boolean;
  from: string;
  to: string;
  sub: string;
  onClick: () => void;
}) {
  return (
    <UiButton
      variant="unstyled"
      size="unstyled"
      type="button"
      onClick={onClick}
      className={cn(
        'flex cursor-pointer flex-col items-start gap-1 rounded-lg p-3.5 text-left transition-shadow focus-visible:ring-3 focus-visible:ring-ring/50',
        selected
          ? 'bg-gold-300/24 shadow-[inset_0_0_0_1.5px_var(--secondary)]'
          : 'bg-muted shadow-hairline',
      )}
    >
      <span className="flex items-center gap-2 font-display text-xl">
        {from} <span className="text-muted-foreground text-sm">→</span> {to}
      </span>
      <span className="text-muted-foreground text-xs tracking-[0.06em]">
        {sub}
      </span>
    </UiButton>
  );
}

function PagePreview({
  label,
  selected,
  onSelect,
  children,
}: {
  label: string;
  selected: boolean;
  onSelect: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="flex w-full min-w-0 flex-col items-center gap-1.5 min-[480px]:w-28">
      <UiButton
        variant="unstyled"
        size="unstyled"
        type="button"
        onClick={onSelect}
        aria-label={label}
        className="aspect-3/4 w-full max-w-28 cursor-pointer overflow-hidden rounded-md shadow-hairline focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        {children}
      </UiButton>
      <Chip
        selected={selected}
        onClick={onSelect}
        className="min-h-7.5 max-w-full px-2 text-xs min-[480px]:px-3.5"
      >
        {label}
      </Chip>
    </div>
  );
}

function FontChips({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (font: string) => void;
}) {
  const fonts = [
    { key: 'kai', label: '文楷', sub: 'TC', className: 'font-display' },
    { key: 'serif', label: '宋體', sub: 'TC+SC', className: 'font-reading' },
  ];
  return (
    <div>
      <div className="mb-2 font-medium text-sm">{label}</div>
      <div className="flex flex-wrap gap-2">
        {fonts.map((font) => (
          <Chip
            key={font.key}
            selected={value === font.key}
            onClick={() => onChange(font.key)}
            className={font.className}
          >
            {font.label}{' '}
            <span className="text-[9px] opacity-75">{font.sub}</span>
          </Chip>
        ))}
      </div>
    </div>
  );
}
