import {
  CapabilityStatus,
  type CharacterCapabilities,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';
import { cn } from '@/lib/utils';

type CapabilityName =
  | 'reading'
  | 'meaning'
  | 'strokes'
  | 'components'
  | 'history';

const ENTRY_CAPABILITIES = ['reading', 'meaning'] as const;
const GLYPH_CAPABILITIES = ['strokes', 'components', 'history'] as const;

const CAPABILITY_LABELS: Record<CapabilityName, StringKey> = {
  reading: 'capReading',
  meaning: 'capMeaning',
  strokes: 'capStrokes',
  components: 'capComponents',
  history: 'capHistory',
};

function getCapabilityStatus(
  capabilities: CharacterCapabilities | undefined,
  capability: CapabilityName,
): CapabilityStatus {
  return capabilities?.[capability] ?? CapabilityStatus.UNSPECIFIED;
}

function capabilityLabelKey(capability: CapabilityName): StringKey {
  return CAPABILITY_LABELS[capability];
}

function statusLabelKey(status: CapabilityStatus): StringKey {
  switch (status) {
    case CapabilityStatus.AVAILABLE:
      return 'capAvailable';
    case CapabilityStatus.NOT_APPLICABLE:
      return 'capNotApplicable';
    case CapabilityStatus.UNAVAILABLE:
      return 'capUnavailable';
    case CapabilityStatus.UNSPECIFIED:
      return 'capUnreported';
    default:
      return status satisfies never;
  }
}

function CapabilityStatusBadge({
  status,
  className,
}: {
  status: CapabilityStatus;
  className?: string;
}) {
  const { t } = useLocale();

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 text-xs',
        status === CapabilityStatus.AVAILABLE
          ? 'text-status-exact'
          : 'text-muted-foreground',
        className,
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          'size-1.5 rounded-full bg-current',
          status === CapabilityStatus.UNAVAILABLE && 'opacity-55',
          status === CapabilityStatus.NOT_APPLICABLE && 'rounded-none',
        )}
      />
      {t(statusLabelKey(status))}
    </span>
  );
}

export {
  type CapabilityName,
  CapabilityStatusBadge,
  capabilityLabelKey,
  ENTRY_CAPABILITIES,
  GLYPH_CAPABILITIES,
  getCapabilityStatus,
};
