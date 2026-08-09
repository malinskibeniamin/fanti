import {
  createRootRoute,
  Link,
  Outlet,
  useRouter,
} from '@tanstack/react-router';

import { chipVariants } from '@/components/fanti/chip';
import {
  ArrowLeftRight,
  ChevronLeft,
  GraduationCap,
  Library,
  Search,
} from '@/components/icons';
import { ThemeToggle } from '@/components/theme-toggle';
import { Button } from '@/components/ui/button';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useShellState } from '@/hooks/use-shell-state';
import { type Locale, useLocale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';
import { cn } from '@/lib/utils';

export const Route = createRootRoute({
  component: RootLayout,
});

const TAB_ITEMS = [
  { to: '/', labelKey: 'navLib', icon: Library },
  { to: '/convert', labelKey: 'navCv', icon: ArrowLeftRight },
  { to: '/study', labelKey: 'navSt', icon: GraduationCap },
  { to: '/discover', labelKey: 'navDc', icon: Search },
] as const satisfies ReadonlyArray<{
  to: string;
  labelKey: StringKey;
  icon: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
}>;

const LOCALE_CHIPS: ReadonlyArray<{ value: Locale; label: string }> = [
  { value: 'en', label: 'EN' },
  { value: 'sc', label: '简' },
  { value: 'tc', label: '繁' },
];

function RootLayout() {
  const { isTabScreen, isReader } = useShellState();

  return (
    <TooltipProvider>
      <div className="min-h-dvh bg-[radial-gradient(900px_340px_at_50%_-80px,color-mix(in_srgb,var(--gold-300)_22%,transparent),transparent_72%)] text-foreground">
        <ShellHeader isTabScreen={isTabScreen} />
        <main
          className={cn(
            'mx-auto w-full px-5 py-5 pb-[calc(112px+env(safe-area-inset-bottom))] min-[880px]:pb-10',
            isReader ? 'max-w-[720px]' : 'max-w-[1040px]',
          )}
        >
          <Outlet />
        </main>
        {isTabScreen ? <BottomNav /> : null}
      </div>
    </TooltipProvider>
  );
}

function ShellHeader({ isTabScreen }: { isTabScreen: boolean }) {
  return (
    <header className="sticky top-0 z-40 bg-[color-mix(in_srgb,var(--background)_84%,transparent)] pt-[env(safe-area-inset-top)] shadow-[inset_0_-1px_0_color-mix(in_srgb,var(--foreground)_8%,transparent)] backdrop-blur-[10px]">
      <div className="mx-auto flex w-full max-w-[1040px] items-center justify-between gap-3 px-5 py-2">
        {isTabScreen ? <BrandLockup /> : <BackTitle />}
        {isTabScreen ? <TopNav /> : null}
        <div className="flex flex-none items-center gap-2">
          <LocaleChips />
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}

function BrandLockup() {
  return (
    <div className="flex flex-none items-center gap-2.5">
      <img
        src="/fanti-mark.svg"
        alt="Fanti"
        className="size-8 rounded-lg shadow-xs"
      />
      <div className="flex items-baseline gap-2">
        <span className="font-display text-[22px] leading-none">繁体</span>
        <span className="whitespace-nowrap font-semibold text-[10px] text-muted-foreground uppercase tracking-[0.16em] max-[479px]:hidden">
          Fanti · 玉簡閣
        </span>
      </div>
    </div>
  );
}

/** Detail-screen header: back button plus the current page's title and gloss. */
function BackTitle() {
  const router = useRouter();
  const { pathname } = useShellState();
  const { t, tGloss } = useLocale();

  const detail = detailHeading(pathname, t, tGloss);

  function goBack() {
    if (router.history.canGoBack()) {
      router.history.back();
    } else {
      router.navigate({ to: '/' });
    }
  }

  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <Button
        variant="ghost"
        aria-label={t('backL')}
        onClick={goBack}
        className="size-10 rounded-lg text-foreground"
      >
        <ChevronLeft aria-hidden="true" className="size-[22px]" />
      </Button>
      <div className="min-w-0">
        <div className="overflow-hidden text-ellipsis whitespace-nowrap font-display text-[17px]">
          {detail.title}
        </div>
        <div className="overflow-hidden text-ellipsis whitespace-nowrap text-[11px] text-muted-foreground tracking-[0.1em]">
          {detail.sub}
        </div>
      </div>
    </div>
  );
}

function detailHeading(
  pathname: string,
  t: (key: StringKey) => string,
  tGloss: (key: StringKey) => string,
): { title: string; sub: string } {
  const segments = pathname.split('/').filter(Boolean);
  const param = decodeURIComponent(segments[1] ?? '');
  switch (segments[0]) {
    case 'books':
      return {
        title: param,
        sub: `${t('aboutBook')} · ${tGloss('aboutBook')}`,
      };
    case 'read':
      return { title: param, sub: `${t('resume')} · ${tGloss('resume')}` };
    case 'characters':
      return { title: param, sub: `${t('stDictL')} · ${tGloss('stDictL')}` };
    case 'guides':
      return {
        title: param,
        sub: `${t('dcSubGuides')} · ${tGloss('dcSubGuides')}`,
      };
    default:
      return { title: t('backL'), sub: tGloss('backL') };
  }
}

/** Wide-viewport nav pills (≥880px). */
function TopNav() {
  const { t } = useLocale();
  return (
    <nav className="hidden gap-1 min-[880px]:flex">
      {TAB_ITEMS.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          activeOptions={{ exact: item.to === '/' }}
          className={cn(
            chipVariants({ selected: false }),
            'px-[18px] py-[7px]',
          )}
          activeProps={{
            className: cn(
              chipVariants({ selected: true }),
              'px-[18px] py-[7px]',
            ),
          }}
        >
          {t(item.labelKey)}
        </Link>
      ))}
    </nav>
  );
}

function LocaleChips() {
  const { locale, setLocale } = useLocale();
  return (
    // biome-ignore lint/a11y/useSemanticElements: design ships a styled chip group; fieldset carries unwanted form semantics and default styling
    <div
      role="group"
      aria-label="Language"
      className="flex gap-0.5 rounded-full bg-muted p-[3px]"
    >
      {LOCALE_CHIPS.map((chip) => {
        const selected = locale === chip.value;
        return (
          <Button
            variant="unstyled"
            size="unstyled"
            key={chip.value}
            type="button"
            aria-pressed={selected}
            onClick={() => setLocale(chip.value)}
            className={cn(
              'min-h-7 cursor-pointer rounded-full border-none px-2.5 font-ui text-[11px] outline-none transition-colors duration-(--duration-fast) focus-visible:ring-3 focus-visible:ring-ring/50',
              selected
                ? 'bg-card font-semibold text-foreground shadow-xs'
                : 'bg-transparent font-normal text-muted-foreground',
            )}
          >
            {chip.label}
          </Button>
        );
      })}
    </div>
  );
}

/** Narrow-viewport bottom tab bar (<880px). */
function BottomNav() {
  const { t, tGloss } = useLocale();
  const navItemClass =
    'flex min-h-14 flex-col items-center justify-center gap-0.5 rounded-lg px-1 py-2 outline-none focus-visible:ring-3 focus-visible:ring-ring/50';
  return (
    <nav className="fixed inset-x-0 bottom-0 z-40 bg-[color-mix(in_srgb,var(--card)_88%,transparent)] shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_8%,transparent)] backdrop-blur-[10px] min-[880px]:hidden">
      <div className="mx-auto grid max-w-[480px] grid-cols-4 px-2 pt-1.5 pb-[calc(6px+env(safe-area-inset-bottom))]">
        {TAB_ITEMS.map((item) => {
          const Icon = item.icon;
          return (
            <Link
              key={item.to}
              to={item.to}
              activeOptions={{ exact: item.to === '/' }}
              className={cn(navItemClass, 'text-muted-foreground')}
              activeProps={{ className: cn(navItemClass, 'text-primary') }}
            >
              <Icon aria-hidden={true} className="size-[21px]" />
              <span className="text-xs">{t(item.labelKey)}</span>
              <span className="text-[8px] uppercase tracking-[0.14em] opacity-70">
                {tGloss(item.labelKey)}
              </span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
