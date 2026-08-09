import { useState } from 'react';

import { Button } from '@/components/fanti/button';
import {
  CapabilityStatus,
  type RadicalPart,
} from '@/gen/fanti/v1/dictionary_pb';
import { type Locale, useLocale } from '@/i18n/locale';
import { cn } from '@/lib/utils';

interface RadicalAssemblyProps {
  glyph: string;
  parts: RadicalPart[];
  status?: CapabilityStatus;
}

type AssemblyMessage = 'complete' | 'hint' | 'idle' | 'mistake' | 'progress';

function identifyParts(parts: RadicalPart[]) {
  const occurrences = new Map<string, number>();
  return parts.map((part) => {
    const occurrence = occurrences.get(part.part) ?? 0;
    occurrences.set(part.part, occurrence + 1);
    return { id: `${part.part}-${occurrence}`, part };
  });
}

function statusCopy(
  locale: Locale,
  message: AssemblyMessage,
  glyph: string,
  completed: number,
  total: number,
  next?: string,
) {
  if (locale === 'en') {
    if (message === 'mistake') return 'Try another component';
    if (message === 'hint') return `Next component: ${next ?? ''}`;
    if (message === 'progress') {
      return `${completed} of ${total} components added`;
    }
    if (message === 'complete') {
      return `${glyph} built from ${total} components`;
    }
    return 'Choose the first component';
  }

  if (locale === 'tc') {
    if (message === 'mistake') return '請選另一個部件';
    if (message === 'hint') return `下一個部件：${next ?? ''}`;
    if (message === 'progress') return `已加入 ${completed}／${total} 個部件`;
    if (message === 'complete') return `已用 ${total} 個部件組成「${glyph}」`;
    return '請選第一個部件';
  }

  if (message === 'mistake') return '请选择另一个部件';
  if (message === 'hint') return `下一个部件：${next ?? ''}`;
  if (message === 'progress') return `已加入 ${completed}／${total} 个部件`;
  if (message === 'complete') return `已用 ${total} 个部件组成“${glyph}”`;
  return '请选择第一个部件';
}

function labelCopy(locale: Locale, key: 'again' | 'add' | 'hint') {
  if (locale === 'en') {
    return key === 'again'
      ? 'Build again'
      : key === 'hint'
        ? 'Hint next component'
        : 'Add component';
  }
  if (locale === 'tc') {
    return key === 'again'
      ? '再組一次'
      : key === 'hint'
        ? '提示下一個部件'
        : '加入部件';
  }
  return key === 'again'
    ? '再组一次'
    : key === 'hint'
      ? '提示下一个部件'
      : '加入部件';
}

function RadicalAssembly({ glyph, parts, status }: RadicalAssemblyProps) {
  const { locale, t } = useLocale();
  const [usedIndexes, setUsedIndexes] = useState<number[]>([]);
  const [message, setMessage] = useState<AssemblyMessage>('idle');

  if (
    parts.length === 0 ||
    status === CapabilityStatus.UNAVAILABLE ||
    status === CapabilityStatus.NOT_APPLICABLE
  ) {
    return (
      <p className="mt-3 text-center text-muted-foreground text-sm">
        {status === CapabilityStatus.NOT_APPLICABLE
          ? t('capNotApplicable')
          : t('componentsUnavailable')}
      </p>
    );
  }

  const partEntries = identifyParts(parts);
  const nextPart = parts[usedIndexes.length];
  const complete = usedIndexes.length === parts.length;
  const choiceIndexes = partEntries.map((_part, index) => index);
  if (choiceIndexes.length > 1) {
    choiceIndexes.push(choiceIndexes.shift() ?? 0);
  }

  function choose(index: number) {
    const choice = parts[index];
    if (!choice || !nextPart || choice.part !== nextPart.part) {
      setMessage('mistake');
      return;
    }

    const nextUsed = [...usedIndexes, index];
    setUsedIndexes(nextUsed);
    setMessage(nextUsed.length === parts.length ? 'complete' : 'progress');
  }

  function reset() {
    setUsedIndexes([]);
    setMessage('idle');
  }

  return (
    <div className="mt-3 flex flex-col items-center gap-3.5">
      <div
        aria-hidden="true"
        className="flex flex-wrap items-center justify-center gap-2"
      >
        {partEntries.map(({ id, part }, index) => (
          <span
            key={id}
            className="flex size-13 items-center justify-center rounded-md bg-muted font-display text-[26px]"
          >
            {index < usedIndexes.length ? part.part : '？'}
          </span>
        ))}
        <span aria-hidden="true" className="text-muted-foreground">
          =
        </span>
        <span className="flex size-13 items-center justify-center rounded-md bg-gold-300/25 font-display text-[30px]">
          {complete ? glyph : '？'}
        </span>
      </div>

      <div className="flex flex-wrap justify-center gap-2">
        {choiceIndexes.map((index) => {
          const { id, part } = partEntries[index];
          const used = usedIndexes.includes(index);
          const hinted = message === 'hint' && part.part === nextPart?.part;
          return (
            <Button
              key={id}
              variant="outline"
              aria-label={`${labelCopy(locale, 'add')} ${part.part}`}
              disabled={used || complete}
              onClick={() => choose(index)}
              className={cn(
                'min-h-14 min-w-16 flex-col gap-0.5 px-3',
                hinted && 'ring-3 ring-accent/55',
              )}
            >
              <span className="font-display text-2xl">{part.part}</span>
              {part.note ? (
                <span className="max-w-28 text-wrap text-[10px] text-muted-foreground">
                  {part.note}
                </span>
              ) : null}
            </Button>
          );
        })}
      </div>

      <p role="status" aria-live="polite" className="text-center text-sm">
        {statusCopy(
          locale,
          message,
          glyph,
          usedIndexes.length,
          parts.length,
          nextPart?.part,
        )}
      </p>

      {complete ? (
        <Button variant="outline" onClick={reset}>
          {labelCopy(locale, 'again')}
        </Button>
      ) : (
        <Button variant="ghost" onClick={() => setMessage('hint')}>
          {labelCopy(locale, 'hint')}
        </Button>
      )}
    </div>
  );
}

export { RadicalAssembly };
