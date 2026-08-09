import { createFileRoute, Link, notFound } from '@tanstack/react-router';
import { HanziTile } from '@/components/character/hanzi-tile';
import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { ErrorState } from '@/components/fanti/error-state';
import { PageHeading } from '@/components/fanti/page-heading';
import { SectionLabel } from '@/components/fanti/section-label';
import { Volume2 } from '@/components/icons';
import { buttonVariants } from '@/components/ui/button';
import {
  INPUT_METHODS,
  isGuideId,
  localized,
  PY_FINALS,
  PY_INITIALS,
  PY_TONES,
  STROKES8,
  ZY_MAP,
  ZY_TONES,
} from '@/content/discover';
import { useLocale } from '@/i18n/locale';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

export const Route = createFileRoute('/guides/$guideId')({
  component: GuidePage,
  errorComponent: GuideError,
  notFoundComponent: GuideError,
  loader: ({ params }) => {
    if (!isGuideId(params.guideId)) {
      throw notFound();
    }
  },
});

function GuideError() {
  const { t } = useLocale();
  return (
    <section className="mx-auto max-w-[680px]">
      <ErrorState title={t('dcSubGuides')} description={t('noMatch')} />
    </section>
  );
}

function GuidePage() {
  const { guideId } = Route.useParams();
  const { t, tGloss, locale } = useLocale();

  const titles: Record<string, { title: string; gloss: string }> = {
    pinyin: { title: t('guidesPinyinT'), gloss: tGloss('guidesPinyinT') },
    zhuyin: { title: t('guidesZhuyinT'), gloss: tGloss('guidesZhuyinT') },
    typing: { title: t('typingTitle'), gloss: tGloss('typingTitle') },
    strokes: { title: t('strokes8T'), gloss: tGloss('strokes8T') },
  };

  const heading = titles[guideId] ?? titles.pinyin;

  return (
    <section className="mx-auto flex max-w-[680px] animate-fanti-fade flex-col gap-4">
      <PageHeading gloss={heading.gloss} title={heading.title} />

      {guideId === 'pinyin' ? (
        <>
          <Card className="p-4 text-muted-foreground text-sm leading-normal">
            {t('pinyinDesc')}
          </Card>
          <Card className="px-4 py-3.5">
            <SectionLabel gloss={tGloss('tonesT')}>{t('tonesT')}</SectionLabel>
            {PY_TONES.map((tone) => (
              <div
                key={tone.mark}
                className="flex flex-wrap items-center gap-3.5 border-foreground/7 border-t py-2.5 first:border-t-0"
              >
                <span className="w-10 flex-none text-center font-reading font-semibold text-2xl">
                  {tone.mark}
                </span>
                <span className="min-w-[140px] flex-1 text-sm">
                  {localized(locale, tone.name)}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="rounded-full bg-muted font-reading"
                  onClick={() => speak(tone.ch)}
                >
                  {tone.ch} {tone.py}
                  <Volume2 size={13} aria-hidden />
                </Button>
              </div>
            ))}
          </Card>
          <Card className="px-4 py-3.5">
            <SectionLabel gloss={tGloss('initialsT')}>
              {t('initialsT')}
            </SectionLabel>
            {PY_INITIALS.map((group) => (
              <SoundGroupRow key={group.g} group={group} />
            ))}
          </Card>
          <Card className="px-4 py-3.5">
            <SectionLabel gloss={tGloss('finalsT')}>
              {t('finalsT')}
            </SectionLabel>
            {PY_FINALS.map((group) => (
              <SoundGroupRow key={group.g} group={group} />
            ))}
            <p className="mt-2.5 text-muted-foreground text-xs">{t('uNote')}</p>
          </Card>
        </>
      ) : null}

      {guideId === 'zhuyin' ? (
        <>
          <Card className="p-4">
            <p className="text-muted-foreground text-sm leading-normal">
              {t('zhuyinDesc')}
            </p>
            <p className="mt-2 text-sm leading-normal">{t('zyUseNote')}</p>
          </Card>
          <Card className="p-4">
            <SectionLabel gloss={tGloss('zySymbolsT')}>
              {t('zySymbolsT')}
            </SectionLabel>
            <div className="mt-3 grid grid-cols-[repeat(auto-fill,minmax(60px,1fr))] gap-2">
              {ZY_MAP.map((symbol) => (
                <div
                  key={symbol.z}
                  className="flex flex-col items-center gap-px rounded-md bg-muted px-1 py-2"
                >
                  <span className="font-display text-xl leading-tight">
                    {symbol.z}
                  </span>
                  <span className="font-reading text-muted-foreground text-xs">
                    {symbol.p}
                  </span>
                </div>
              ))}
            </div>
          </Card>
          <Card className="px-4 py-3.5">
            <SectionLabel gloss={tGloss('zyTonesT')}>
              {t('zyTonesT')}
            </SectionLabel>
            {ZY_TONES.map((tone) => (
              <div
                key={tone.m}
                className="flex items-center gap-3.5 border-foreground/7 border-t py-2.5 first:border-t-0"
              >
                <span className="w-10 flex-none text-center font-display text-[22px]">
                  {tone.m}
                </span>
                <span className="text-sm">{localized(locale, tone.n)}</span>
              </div>
            ))}
          </Card>
        </>
      ) : null}

      {guideId === 'typing' ? (
        <Card className="px-4 py-3.5">
          <p className="py-1 text-muted-foreground text-sm leading-normal">
            {t('typingDesc')}
          </p>
          {INPUT_METHODS.map((method) => (
            <div
              key={method.glyph}
              className="flex items-start gap-3 border-foreground/7 border-t py-3"
            >
              <span className="flex size-10 flex-none items-center justify-center rounded-md bg-muted font-display text-xl">
                {method.glyph}
              </span>
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="font-semibold text-sm">
                  {localized(locale, method.name)}
                </span>
                <span className="text-muted-foreground text-sm leading-normal">
                  {localized(locale, method.desc)}
                </span>
              </div>
            </div>
          ))}
          <p className="mt-2.5 text-muted-foreground text-xs">
            {t('typingNote')}
          </p>
        </Card>
      ) : null}

      {guideId === 'strokes' ? (
        <Card className="p-4">
          <div className="flex flex-wrap items-center gap-4">
            <HanziTile glyph="永" size={92} fontSize={60} />
            <p className="min-w-[200px] flex-1 text-muted-foreground text-sm leading-normal">
              {t('strokes8Intro')}
            </p>
          </div>
          <div className="mt-2 grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-x-5">
            {STROKES8.map((stroke) => (
              <div
                key={stroke.g}
                className="flex items-center gap-2.5 border-foreground/7 border-t py-2.5"
              >
                <span className="flex size-7.5 flex-none items-center justify-center rounded-md bg-muted font-display text-lg">
                  {stroke.g}
                </span>
                <span className="flex-none whitespace-nowrap font-display font-semibold text-base">
                  {stroke.n}
                </span>
                <span className="text-muted-foreground text-sm leading-snug">
                  {localized(locale, stroke.d)}
                </span>
              </div>
            ))}
          </div>
          <div className="mt-3.5">
            <Link
              to="/study"
              search={{ tab: 'strokes' }}
              className={cn(buttonVariants({ variant: 'secondary' }))}
            >
              {t('practiceYong')}
            </Link>
          </div>
        </Card>
      ) : null}
    </section>
  );
}

function SoundGroupRow({
  group,
}: {
  group: { g: string; n: { en: string; tc: string; sc: string } };
}) {
  const { locale } = useLocale();
  return (
    <div className="flex flex-col gap-0.5 border-foreground/7 border-t py-2.5 first:border-t-0">
      <span className="font-reading font-semibold text-base tracking-[0.06em]">
        {group.g}
      </span>
      <span className="text-muted-foreground text-sm">
        {localized(locale, group.n)}
      </span>
    </div>
  );
}
