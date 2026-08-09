import { useMutation, useQuery } from '@connectrpc/connect-query';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { ReportStep } from '@/components/convert/report-step';
import { SettingsStep } from '@/components/convert/settings-step';
import { UploadStep } from '@/components/convert/upload-step';
import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { PageHeading } from '@/components/fanti/page-heading';
import { Skeleton } from '@/components/fanti/skeleton';
import {
  Conversion_State,
  ConversionService,
} from '@/gen/fanti/v1/conversion_pb';
import { useLocale } from '@/i18n/locale';
import { resourceId, toastRpcError } from '@/lib/book-format';
import { cn } from '@/lib/utils';

interface ConvertSearch {
  /** Active conversion id — refresh resumes the wizard. */
  c?: string;
}

export const Route = createFileRoute('/convert')({
  component: ConvertPage,
  validateSearch: (search: unknown): ConvertSearch => {
    const candidate =
      typeof search === 'object' && search !== null
        ? (search as { c?: unknown })
        : {};
    return { c: typeof candidate.c === 'string' ? candidate.c : undefined };
  },
});

const RUNNING_POLL_MS = 300;

function ConvertPage() {
  const { t, tGloss, locale } = useLocale();
  const { c: conversionId } = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  // SUCCEEDED + adjusting shows settings again without discarding the report.
  const [adjusting, setAdjusting] = useState(false);

  // Poll while the job runs. Explicit interval: connect-query does not
  // forward refetchInterval to the underlying query.
  const [polling, setPolling] = useState(false);

  const conversionQuery = useQuery(
    ConversionService.method.getConversion,
    { name: `conversions/${conversionId ?? ''}` },
    { enabled: Boolean(conversionId) },
  );

  const { refetch } = conversionQuery;

  useEffect(
    function pollRunningConversion() {
      if (!polling) {
        return;
      }

      const id = setInterval(() => {
        void refetch();
      }, RUNNING_POLL_MS);

      return () => clearInterval(id);
    },
    [polling, refetch],
  );

  const liveState = conversionQuery.data?.state;

  if (
    polling &&
    liveState !== undefined &&
    liveState !== Conversion_State.RUNNING
  ) {
    setPolling(false);
  }

  const runMutation = useMutation(ConversionService.method.runConversion, {
    onSuccess: () => {
      setAdjusting(false);
      setPolling(true);
      void conversionQuery.refetch();
    },
    onError: toastRpcError,
  });
  const deleteMutation = useMutation(
    ConversionService.method.deleteConversion,
    {
      onSuccess: () => setConversion(undefined),
      onError: toastRpcError,
    },
  );

  function setConversion(id: string | undefined) {
    void navigate({ search: { c: id } });
  }

  const conversion = conversionId ? conversionQuery.data : undefined;
  const state = conversion?.state;

  if (!polling && state === Conversion_State.RUNNING) {
    setPolling(true);
  }

  const stepIndex =
    !conversionId || !conversion
      ? 0
      : state === Conversion_State.SUCCEEDED && !adjusting
        ? 2
        : 1;

  const stepLabels =
    locale === 'en'
      ? ['1 Upload', '2 Set up', '3 Review']
      : locale === 'tc'
        ? ['1 上傳', '2 設定', '3 校對']
        : ['1 上传', '2 设置', '3 校对'];

  return (
    <section className="flex animate-fanti-fade flex-col gap-5">
      <div className="flex items-end justify-between gap-3">
        <PageHeading gloss={tGloss('navCv')} title={t('navCv')} />
        <div className="flex gap-1.5 pb-1">
          {stepLabels.map((label, index) => (
            <span
              key={label}
              className={cn(
                'whitespace-nowrap rounded-full px-2.5 py-1 text-xs tabular-nums',
                index === stepIndex
                  ? 'bg-primary font-semibold text-primary-foreground'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {label}
            </span>
          ))}
        </div>
      </div>

      {!conversionId ? (
        <UploadStep onCreated={(name) => setConversion(resourceId(name))} />
      ) : conversionQuery.isError ? (
        <ErrorState
          title={t('navCv')}
          description={conversionQuery.error.message}
          onRetry={() => conversionQuery.refetch()}
        />
      ) : !conversion ? (
        <Skeleton className="h-60 rounded-xl" />
      ) : state === Conversion_State.RUNNING ? (
        <ConvertingCard
          progressPercent={conversion.progressPercent}
          charCount={conversion.charCount}
        />
      ) : state === Conversion_State.FAILED ? (
        <ErrorState
          title={t('navCv')}
          description={conversion.errorMessage}
          onRetry={() => runMutation.mutate({ name: conversion.name })}
          retryLabel={t('adjust')}
        />
      ) : state === Conversion_State.SUCCEEDED &&
        conversion.report &&
        !adjusting ? (
        <ReportStep
          conversion={conversion}
          onAdjust={() => setAdjusting(true)}
        />
      ) : (
        <SettingsStep
          conversion={conversion}
          onRun={() => runMutation.mutate({ name: conversion.name })}
          onReset={() => deleteMutation.mutate({ name: conversion.name })}
        />
      )}
    </section>
  );
}

function ConvertingCard({
  progressPercent,
  charCount,
}: {
  progressPercent: number;
  charCount: bigint;
}) {
  const { t } = useLocale();
  return (
    <Card className="flex flex-col items-center gap-3.5 px-5 py-7">
      <div className="font-display text-lg">{t('converting')}</div>
      <div
        className="h-1.5 w-full max-w-[380px] overflow-hidden rounded-[3px] bg-muted"
        role="progressbar"
        aria-valuenow={progressPercent}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={t('converting')}
      >
        <div
          className="h-full bg-secondary transition-[width] duration-(--duration-base)"
          // Live progress width from the polled job.
          style={{ width: `${progressPercent}%` }}
        />
      </div>
      <div className="text-muted-foreground text-sm tabular-nums">
        {progressPercent}% · {charCount.toString()} 字
      </div>
    </Card>
  );
}
