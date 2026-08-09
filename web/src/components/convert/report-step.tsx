import {
  createConnectQueryKey,
  useMutation,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { SectionLabel } from '@/components/fanti/section-label';
import { CircleCheck } from '@/components/icons';
import { Button as UiButton } from '@/components/ui/button';
import { MappingStatus } from '@/gen/fanti/v1/common_pb';
import {
  type Conversion,
  type ConversionException,
  ConversionService,
} from '@/gen/fanti/v1/conversion_pb';
import { type Locale, useLocale } from '@/i18n/locale';
import { resourceId, toastRpcError } from '@/lib/book-format';
import { saveFile } from '@/lib/save-file';
import { cn } from '@/lib/utils';

interface ReportStepProps {
  conversion: Conversion;
  onAdjust: () => void;
}

/** Step 3 — the conversion report: stats, exceptions, diff, actions. */
export function ReportStep({ conversion, onAdjust }: ReportStepProps) {
  const { t, tGloss, locale } = useLocale();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const report = conversion.report;

  const resolveMutation = useMutation(
    ConversionService.method.resolveException,
    {
      onSuccess: () =>
        queryClient.invalidateQueries({
          queryKey: createConnectQueryKey({
            schema: ConversionService.method.getConversion,
            transport,
            input: { name: conversion.name },
            cardinality: 'finite',
          }),
        }),
      onError: toastRpcError,
    },
  );
  const exportMutation = useMutation(
    ConversionService.method.exportConversion,
    {
      onSuccess: (resp) => saveFile(resp.filename, resp.data),
      onError: toastRpcError,
    },
  );
  const addMutation = useMutation(
    ConversionService.method.addConversionToLibrary,
    {
      onSuccess: (book) => {
        toast.success(`${t('addLib')} ✓`);
        void navigate({
          to: '/books/$bookId',
          params: { bookId: resourceId(book.name) },
        });
      },
      onError: toastRpcError,
    },
  );

  if (!report) {
    return null;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2.5">
        <span className="flex text-status-exact">
          <CircleCheck size={24} aria-hidden />
        </span>
        <div>
          <div className="font-display text-xl">{t('cvDone')}</div>
          <div className="text-muted-foreground text-sm tabular-nums">
            {conversion.charCount.toString()} 字 ·{' '}
            {report.totalSubstitutions.toString()} · {t('dirTitle')}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-2.5">
        <StatCard
          value={report.exactCount}
          label={t('exact')}
          sub="Exact"
          className="text-status-exact"
        />
        <StatCard
          value={report.ambiguousCount}
          label={t('ambig')}
          sub="Ambiguous"
          className="text-status-ambiguous"
        />
        <StatCard
          value={report.manualCount}
          label={t('manual')}
          sub="Manual"
          className="text-status-manual"
        />
      </div>

      {report.exceptions.length > 0 ? (
        <Card className="flex flex-col gap-3.5 p-4">
          <div className="flex items-center justify-between">
            <SectionLabel gloss={tGloss('reviewTitle')}>
              {t('reviewTitle')}
            </SectionLabel>
            <span className="text-muted-foreground text-xs tabular-nums">
              {t('reviewHint')}
            </span>
          </div>
          {report.exceptions.map((exception) => (
            <ExceptionRow
              key={exception.sourceChar}
              exception={exception}
              locale={locale}
              onResolve={(resolved) =>
                resolveMutation.mutate({
                  name: conversion.name,
                  sourceChar: exception.sourceChar,
                  resolved,
                })
              }
            />
          ))}
        </Card>
      ) : null}

      <Card className="p-4">
        <SectionLabel gloss={tGloss('diffTitle')}>
          {t('diffTitle')}
        </SectionLabel>
        <div className="mt-2.5 font-reading text-muted-foreground text-sm leading-loose">
          {report.diff?.sourceText}
        </div>
        <DiffLine
          tokens={report.diff?.tokens.map((token, index) => ({
            key: `${index}:${token.text}`,
            text: token.text,
            status: token.status,
          }))}
        />
        <div className="mt-2.5 flex gap-3.5">
          <LegendDot className="bg-status-exact" label={t('legendExact')} />
          <LegendDot className="bg-status-ambiguous" label={t('legendAmb')} />
        </div>
      </Card>

      <div className="flex flex-wrap gap-2.5">
        <Button
          variant="outline"
          size="lg"
          className="min-w-[150px] flex-1"
          onClick={onAdjust}
        >
          {t('adjust')}
        </Button>
        <Button
          variant="secondary"
          size="lg"
          className="min-w-[150px] flex-1"
          disabled={exportMutation.isPending}
          onClick={() => exportMutation.mutate({ name: conversion.name })}
        >
          {t('download')}
        </Button>
        <Button
          variant="accent"
          size="lg"
          className="min-w-[150px] flex-1"
          disabled={addMutation.isPending}
          onClick={() => addMutation.mutate({ name: conversion.name })}
        >
          {t('addLib')}
        </Button>
      </div>
    </div>
  );
}

function DiffLine({
  tokens,
}: {
  tokens?: { key: string; text: string; status: MappingStatus }[];
}) {
  return (
    <div className="mt-1.5 font-reading text-lg leading-loose">
      {tokens?.map((token) => (
        <span
          key={token.key}
          className={cn(
            'rounded-[4px] px-px',
            token.status === MappingStatus.EXACT && 'bg-accent/18',
            token.status === MappingStatus.AMBIGUOUS && 'bg-gold-300/45',
          )}
        >
          {token.text}
        </span>
      ))}
    </div>
  );
}

function StatCard({
  value,
  label,
  sub,
  className,
}: {
  value: bigint;
  label: string;
  sub: string;
  className: string;
}) {
  return (
    <Card className="flex flex-col gap-1 p-3.5">
      <span className={cn('font-semibold text-[22px] tabular-nums', className)}>
        {value.toString()}
      </span>
      <span className="text-sm">{label}</span>
      <span className="text-[10px] text-muted-foreground uppercase tracking-[0.12em]">
        {sub}
      </span>
    </Card>
  );
}

function ExceptionRow({
  exception,
  locale,
  onResolve,
}: {
  exception: ConversionException;
  locale: Locale;
  onResolve: (resolved: string) => void;
}) {
  const note =
    locale === 'en'
      ? exception.note?.en
      : locale === 'tc'
        ? exception.note?.tc
        : exception.note?.sc;

  return (
    <div className="flex items-start gap-3.5 border-foreground/7 border-t pt-3">
      <span className="w-10 flex-none text-center font-display text-3xl leading-tight">
        {exception.sourceChar}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2">
          {exception.options.map((option) => (
            <UiButton
              variant="unstyled"
              size="unstyled"
              key={option}
              type="button"
              onClick={() => onResolve(option)}
              className={cn(
                'min-h-10 min-w-11 cursor-pointer rounded-md px-3 font-display text-xl transition-colors focus-visible:ring-3 focus-visible:ring-ring/50',
                exception.resolved === option
                  ? 'bg-accent text-accent-foreground'
                  : 'bg-muted text-foreground hover:bg-gold-300/30',
              )}
            >
              {option}
            </UiButton>
          ))}
        </div>
        {note ? (
          <p className="text-muted-foreground text-sm leading-snug">{note}</p>
        ) : null}
        <p className="font-reading text-muted-foreground text-sm">
          {exception.context}
        </p>
      </div>
      <span
        className={cn(
          'whitespace-nowrap rounded-full px-2.5 py-0.5 font-semibold text-[10px]',
          exception.resolved
            ? 'bg-accent/16 text-status-exact'
            : exception.status === MappingStatus.MANUAL
              ? 'bg-primary/12 text-status-manual'
              : 'bg-gold-300/30 text-status-ambiguous',
        )}
      >
        {exception.resolved ? '已確認' : statusLabel(exception.status, locale)}
      </span>
    </div>
  );
}

function statusLabel(status: MappingStatus, locale: Locale): string {
  const labels: Record<Locale, Record<number, string>> = {
    en: {
      [MappingStatus.AMBIGUOUS]: 'context',
      [MappingStatus.MANUAL]: 'manual',
    },
    tc: { [MappingStatus.AMBIGUOUS]: '語境', [MappingStatus.MANUAL]: '人工' },
    sc: { [MappingStatus.AMBIGUOUS]: '语境', [MappingStatus.MANUAL]: '人工' },
  };
  return labels[locale][status] ?? '';
}

function LegendDot({ className, label }: { className: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5 text-muted-foreground text-xs">
      <span className={cn('inline-block size-2 rounded-full', className)} />
      {label}
    </span>
  );
}
