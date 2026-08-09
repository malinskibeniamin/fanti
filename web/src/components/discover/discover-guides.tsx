import { Link } from '@tanstack/react-router';

import { Card } from '@/components/fanti/card';
import { ChevronRight } from '@/components/icons';
import { GUIDE_GLYPHS, GUIDE_IDS, type GuideId } from '@/content/discover';
import { useLocale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';

const GUIDE_NAME_KEY: Record<GuideId, StringKey> = {
  pinyin: 'guidesPinyinT',
  zhuyin: 'guidesZhuyinT',
  typing: 'typingTitle',
  strokes: 'strokes8T',
};

const GUIDE_DESC_KEY: Record<GuideId, StringKey> = {
  pinyin: 'pinyinDesc',
  zhuyin: 'zhuyinDesc',
  typing: 'typingDesc',
  strokes: 'strokesDesc',
};

/** Discover · Guides: index rows linking into the four guide pages. */
function DiscoverGuides() {
  const { t } = useLocale();
  return (
    <Card className="flex flex-col px-4 pt-1.5 pb-2.5">
      {GUIDE_IDS.map((guideId) => (
        <Link
          key={guideId}
          to="/guides/$guideId"
          params={{ guideId }}
          className="flex items-center gap-3 rounded-sm py-3 shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_7%,transparent)] outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <span className="flex size-11 flex-none items-center justify-center rounded-md bg-muted font-display text-[22px]">
            {GUIDE_GLYPHS[guideId]}
          </span>
          <span className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="font-semibold text-sm">
              {t(GUIDE_NAME_KEY[guideId])}
            </span>
            <span className="text-muted-foreground text-xs leading-snug">
              {t(GUIDE_DESC_KEY[guideId])}
            </span>
          </span>
          <ChevronRight
            aria-hidden="true"
            className="size-4 flex-none text-muted-foreground"
          />
        </Link>
      ))}
    </Card>
  );
}

export { DiscoverGuides };
