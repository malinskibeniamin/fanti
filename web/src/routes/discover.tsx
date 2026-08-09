import { createFileRoute, Link } from '@tanstack/react-router';

import { DiscoverChars } from '@/components/discover/discover-chars';
import { DiscoverGuides } from '@/components/discover/discover-guides';
import { DiscoverWords } from '@/components/discover/discover-words';
import { PageHeading } from '@/components/fanti/page-heading';
import { useLocale } from '@/i18n/locale';
import { cn } from '@/lib/utils';

type DiscoverTab = 'chars' | 'words' | 'guides';

interface DiscoverSearch {
  tab: DiscoverTab;
}

export const Route = createFileRoute('/discover')({
  component: DiscoverPage,
  validateSearch: (search: unknown): DiscoverSearch => {
    const candidate =
      typeof search === 'object' && search !== null
        ? (search as { tab?: unknown })
        : {};
    return {
      tab:
        candidate.tab === 'words' || candidate.tab === 'guides'
          ? candidate.tab
          : 'chars',
    };
  },
});

function DiscoverPage() {
  const { t, tGloss } = useLocale();
  const { tab } = Route.useSearch();

  const tabs: { key: DiscoverTab; label: string }[] = [
    { key: 'chars', label: t('dcSubChars') },
    { key: 'words', label: t('dcSubPhrases') },
    { key: 'guides', label: t('dcSubGuides') },
  ];

  return (
    <section className="flex animate-fanti-fade flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <PageHeading gloss={tGloss('navDc')} title={t('navDc')} />
        <nav
          className="flex gap-1 rounded-full bg-muted p-0.75"
          aria-label={t('navDc')}
        >
          {tabs.map(({ key, label }) => (
            <Link
              key={key}
              from={Route.fullPath}
              search={{ tab: key }}
              className={cn(
                'inline-flex min-h-9 items-center justify-center rounded-full px-3.5 py-2 font-ui text-sm tracking-[0.02em] transition-colors focus-visible:ring-3 focus-visible:ring-ring/50',
                tab === key
                  ? 'bg-card font-semibold text-foreground shadow-xs'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {label}
            </Link>
          ))}
        </nav>
      </div>

      {tab === 'chars' ? <DiscoverChars /> : null}
      {tab === 'words' ? <DiscoverWords /> : null}
      {tab === 'guides' ? <DiscoverGuides /> : null}
    </section>
  );
}
