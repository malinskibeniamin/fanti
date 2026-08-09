import { Card } from '@/components/fanti/card';
import { SectionLabel } from '@/components/fanti/section-label';
import { SpeakButton } from '@/components/fanti/speak-button';
import { Button } from '@/components/ui/button';
import {
  LOAN_EN,
  LOAN_ZH,
  type Loanword,
  localized,
  PROVERBS,
  REGION_LABELS,
  REGIONAL,
} from '@/content/discover';
import { useLocale } from '@/i18n/locale';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

const HAIRLINE_ROW =
  'shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_7%,transparent)]';

/** Discover · Words & phrases: loanwords, regional vocabulary, proverbs. */
function DiscoverWords() {
  return (
    <div className="flex flex-col gap-4">
      <CognatesCard />
      <RegionalCard />
      <ProverbsCard />
    </div>
  );
}

/** Loanwords in both directions — the "you already know these" wins. */
function CognatesCard() {
  const { t, tGloss } = useLocale();
  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('cognatesT')}>{t('cognatesT')}</SectionLabel>
      <div className="mt-3 font-semibold text-[11px] text-status-exact uppercase tracking-[0.1em]">
        {t('loanG1')}
      </div>
      {LOAN_EN.map((loanword) => (
        <LoanwordRow key={loanword.trad} loanword={loanword} group="en" />
      ))}
      <div className="mt-3.5 font-semibold text-[11px] text-status-ambiguous uppercase tracking-[0.1em]">
        {t('loanG2')}
      </div>
      {LOAN_ZH.map((loanword) => (
        <LoanwordRow key={loanword.trad} loanword={loanword} group="zh" />
      ))}
    </Card>
  );
}

function LoanwordRow({
  loanword,
  group,
}: {
  loanword: Loanword;
  group: 'en' | 'zh';
}) {
  const { locale } = useLocale();
  return (
    <div className={cn('flex flex-col gap-[3px] pt-2.5 pb-1', HAIRLINE_ROW)}>
      <div className="flex flex-wrap items-center gap-2.5">
        <span className="font-display text-[20px] leading-[1.2]">
          {loanword.trad}
        </span>
        <span className="text-[11px] text-muted-foreground">{loanword.py}</span>
        <span
          className={cn(
            'whitespace-nowrap rounded-full px-2 py-0.5 font-semibold text-[11px]',
            group === 'en'
              ? 'bg-[color-mix(in_srgb,var(--jade-600)_14%,transparent)] text-status-exact'
              : 'bg-gold-300/30 text-[color:var(--ink-700)]',
          )}
        >
          {loanword.en}
        </span>
        <SpeakButton text={loanword.trad} iconClassName="size-3" />
      </div>
      <span className="text-muted-foreground text-xs">
        {localized(locale, loanword.note)}
      </span>
    </div>
  );
}

/** One thing, many names — with a speakable chip per region. */
function RegionalCard() {
  const { t, tGloss, locale } = useLocale();
  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('regionalT')} className="whitespace-nowrap">
        {t('regionalT')}
      </SectionLabel>
      <div className="mt-2 text-muted-foreground text-sm leading-normal">
        {t('regionalIntro')}
      </div>
      {REGIONAL.map((entry) => (
        <div
          key={entry.en}
          className={cn('flex flex-col gap-2 pt-3 pb-1.5', HAIRLINE_ROW)}
        >
          <span className="self-start whitespace-nowrap rounded-full bg-[color-mix(in_srgb,var(--jade-600)_14%,transparent)] px-2 py-0.5 font-semibold text-[11px] text-status-exact">
            {entry.en}
          </span>
          <div className="flex flex-wrap gap-2">
            {entry.variants.map((variant) => (
              <Button
                variant="unstyled"
                size="unstyled"
                key={`${variant.region}-${variant.word}`}
                type="button"
                aria-label={`Pronounce ${variant.word}`}
                onClick={() => speak(variant.word)}
                className="flex min-w-[76px] cursor-pointer flex-col items-center gap-0.5 rounded-md border-none bg-muted px-3 py-1.5 font-ui text-foreground outline-none transition-colors duration-(--duration-fast) hover:bg-[color-mix(in_srgb,var(--gold-300)_30%,var(--muted))] focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <span className="whitespace-nowrap font-display text-[18px] leading-[1.3]">
                  {variant.word}
                </span>
                <span className="whitespace-nowrap text-[10px] text-muted-foreground">
                  {variant.py}
                </span>
                <span className="whitespace-nowrap text-[9px] text-muted-foreground uppercase tracking-[0.1em]">
                  {localized(locale, REGION_LABELS[variant.region])}
                </span>
              </Button>
            ))}
          </div>
          <span className="text-muted-foreground text-xs">
            {localized(locale, entry.note)}
          </span>
        </div>
      ))}
    </Card>
  );
}

/** Classical chengyu with literal and figurative readings. */
function ProverbsCard() {
  const { t, tGloss } = useLocale();
  return (
    <Card className="px-4 py-3.5">
      <SectionLabel gloss={tGloss('proverbsTitle')} className="mb-1">
        {t('proverbsTitle')}
      </SectionLabel>
      {PROVERBS.map((proverb) => (
        <div
          key={proverb.trad}
          className={cn('flex flex-col gap-1 pt-3 pb-1.5', HAIRLINE_ROW)}
        >
          <div className="flex flex-wrap items-center gap-2.5">
            <span className="font-display text-[22px] leading-[1.2]">
              {proverb.trad}
            </span>
            <span className="text-[11px] text-muted-foreground">
              {proverb.py}
            </span>
            <SpeakButton text={proverb.trad} iconClassName="size-[13px]" />
          </div>
          <span className="text-muted-foreground text-xs">{proverb.lit}</span>
          <span className="text-sm">{proverb.fig}</span>
        </div>
      ))}
    </Card>
  );
}

export { DiscoverWords };
