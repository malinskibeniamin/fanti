import { useMutation } from '@connectrpc/connect-query';
import { useRef, useState } from 'react';

import { ErrorState } from '@/components/fanti/error-state';
import { Upload } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ConversionService } from '@/gen/fanti/v1/conversion_pb';
import { useLocale } from '@/i18n/locale';
import { cn } from '@/lib/utils';

interface UploadStepProps {
  onCreated: (conversionName: string) => void;
}

const FORMAT_CHIPS = ['EPUB', 'AZW / MOBI', 'SRT', 'TXT'];
export const MAX_UPLOAD_BYTES = 64 * 1024 * 1024;

export function isUploadWithinLimit(file: Pick<File, 'size'>): boolean {
  return file.size <= MAX_UPLOAD_BYTES;
}

/** Step 1 — drop zone that uploads and analyzes the file. */
export function UploadStep({ onCreated }: UploadStepProps) {
  const { t } = useLocale();
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [fileError, setFileError] = useState<string>();

  const createMutation = useMutation(
    ConversionService.method.createConversion,
    {
      onSuccess: (conversion) => onCreated(conversion.name),
      // The inline error card below renders the failure; nothing extra here.
      onError: () => undefined,
    },
  );

  async function handleFile(file: File) {
    createMutation.reset();
    if (!isUploadWithinLimit(file)) {
      setFileError(t('uploadTooLarge'));

      return;
    }

    setFileError(undefined);
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      createMutation.mutate({ filename: file.name, data: bytes });
    } catch (error) {
      setFileError(
        error instanceof Error ? error.message : t('uploadReadFail'),
      );
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Button
        variant="ghost"
        size="lg"
        type="button"
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragging(false);
          const file = e.dataTransfer.files[0];
          if (file) {
            void handleFile(file);
          }
        }}
        className={cn(
          'relative h-auto min-h-60 w-full cursor-pointer flex-col gap-3 overflow-hidden rounded-xl border-1.5 border-dashed bg-card px-5 py-7 transition-colors hover:bg-card focus-visible:ring-3 focus-visible:ring-ring/50',
          dragging
            ? 'border-secondary'
            : 'border-border hover:border-secondary',
        )}
        disabled={createMutation.isPending}
        aria-label={t('fileDrop')}
      >
        <span className="flex size-13 items-center justify-center rounded-full bg-gold-300/30 text-foreground">
          <Upload size={24} aria-hidden />
        </span>
        <span className="text-center">
          <span className="block font-display text-lg">
            {createMutation.isPending ? t('converting') : t('fileDrop')}
          </span>
          <span className="mt-0.5 block text-muted-foreground text-sm">
            {t('fileDropSub')}
          </span>
        </span>
        <span className="flex flex-wrap justify-center gap-2">
          {FORMAT_CHIPS.map((label) => (
            <span
              key={label}
              className="rounded-full bg-muted px-2.5 py-0.5 text-muted-foreground text-xs"
            >
              {label}
            </span>
          ))}
        </span>
      </Button>
      <Input
        ref={inputRef}
        type="file"
        accept=".epub,.txt,.srt,.mobi,.azw,.azw3"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            void handleFile(file);
          }
          e.target.value = '';
        }}
      />

      {fileError || createMutation.isError ? (
        <ErrorState
          title={t('fileDrop')}
          description={fileError ?? createMutation.error?.message}
          onRetry={() => inputRef.current?.click()}
        />
      ) : null}
    </div>
  );
}
