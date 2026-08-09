import { ConnectError } from '@connectrpc/connect';
import { toast } from 'sonner';

import { FileFormat, Script } from '@/gen/fanti/v1/common_pb';

const SHORT_COUNT_THRESHOLD = 10_000;

/** Grouped count for stat lines, e.g. 96,000. */
export function formatCount(value: bigint | number): string {
  return Number(value).toLocaleString('en-US');
}

/** Compact count for card meta, e.g. 612k. */
export function formatCountShort(value: bigint | number): string {
  const n = Number(value);
  if (n >= SHORT_COUNT_THRESHOLD) {
    return `${Math.round(n / 1000)}k`;
  }
  return formatCount(n);
}

export function scriptChar(script: Script): string {
  switch (script) {
    case Script.SIMPLIFIED:
      return '简';
    case Script.TRADITIONAL:
      return '繁';
    default:
      return '—';
  }
}

export function scriptLabel(script: Script): string {
  switch (script) {
    case Script.SIMPLIFIED:
      return '简体 Simplified';
    case Script.TRADITIONAL:
      return '繁體 Traditional';
    default:
      return '—';
  }
}

export function fileFormatLabel(format: FileFormat): string {
  switch (format) {
    case FileFormat.EPUB:
      return 'EPUB';
    case FileFormat.TXT:
      return 'TXT';
    case FileFormat.SRT:
      return 'SRT';
    case FileFormat.MOBI:
      return 'MOBI';
    default:
      return '';
  }
}

/** books/{book} -> {book}; also used for characters/{character}. */
export function resourceId(name: string): string {
  return name.split('/').pop() ?? name;
}

/** Cover gradient shared by library tiles, detail page, and layout preview. */
export function coverGradient(color: string): string {
  const base = color || '#8f1d18';
  return `linear-gradient(160deg, ${base}, color-mix(in srgb, ${base} 78%, #1f1710))`;
}

/** Surfaces an RPC failure as a toast without losing the server detail. */
export function toastRpcError(error: unknown) {
  toast.error(ConnectError.from(error).rawMessage);
}
