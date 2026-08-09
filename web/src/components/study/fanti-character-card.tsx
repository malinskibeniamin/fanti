import { Button } from '@/components/fanti/button';
import { Card } from '@/components/fanti/card';
import { Volume2 } from '@/components/icons';
import { hskCefrLabel } from '@/components/study/study-content';
import {
  type Character,
  CharacterCatalogKind,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocale } from '@/i18n/locale';
import { speak } from '@/lib/speak';
import { cn } from '@/lib/utils';

interface FantiCharacterCardProps {
  character: Character;
  className?: string;
}

function compactGlyphSizeClass(glyph: string): string {
  const length = Array.from(glyph).length;
  if (length >= 5) {
    return 'text-[14px]';
  }
  if (length >= 4) {
    return 'text-[18px]';
  }
  if (length === 3) {
    return 'text-[26px]';
  }
  return length === 2 ? 'text-[40px]' : 'text-[58px]';
}

/**
 * The design system's CharacterCard — the flashcard answer face: 田-grid
 * glyph tile, pinyin with pronunciation, meaning, 简/繁 forms, origin story,
 * and a learned/new status badge.
 */
function FantiCharacterCard({ character, className }: FantiCharacterCardProps) {
  const { t } = useLocale();
  const levelLabel = hskCefrLabel(character.hskLevel) || t('properTag');

  return (
    <Card className={cn('flex flex-col gap-3.5', className)}>
      <div className="flex items-start gap-4">
        {/* 田-grid glyph tile */}
        <div className="relative flex size-[92px] flex-none items-center justify-center rounded-lg bg-reading-background shadow-[inset_0_0_0_1.5px_color-mix(in_srgb,var(--primary)_28%,transparent)]">
          <div
            aria-hidden="true"
            className="absolute inset-y-1.5 left-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-l border-dashed"
          />
          <div
            aria-hidden="true"
            className="absolute inset-x-1.5 top-1/2 border-[color-mix(in_srgb,var(--primary)_20%,transparent)] border-t border-dashed"
          />
          <span
            className={cn(
              'relative whitespace-nowrap font-display text-reading-foreground leading-none',
              compactGlyphSizeClass(character.traditional),
            )}
          >
            {character.traditional}
          </span>
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-[5px]">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-lg">
              {character.pinyin || t('capUnavailable')}
            </span>
            {character.zhuyin ? (
              <span className="text-muted-foreground text-sm">
                {character.zhuyin}
              </span>
            ) : null}
            <Button
              variant="ghost"
              aria-label="Pronounce"
              onClick={() => speak(character.traditional)}
              className="size-9 min-h-9 rounded-full bg-muted text-foreground hover:bg-secondary hover:text-secondary-foreground"
            >
              <Volume2 aria-hidden="true" className="size-[15px]" />
            </Button>
            <span
              className={cn(
                'whitespace-nowrap rounded-full px-2.5 py-[3px] font-semibold text-[10px] tracking-[0.08em]',
                character.learned
                  ? 'bg-accent/16 text-status-exact'
                  : 'bg-muted text-muted-foreground',
              )}
            >
              {character.learned ? t('learnedTag') : t('newTag')}
            </span>
            <span className="whitespace-nowrap rounded-full bg-muted px-2.5 py-[3px] text-[10px] text-muted-foreground tabular-nums tracking-[0.08em]">
              {levelLabel}
            </span>
            {character.catalogKind === CharacterCatalogKind.REFERENCE ? (
              <span className="whitespace-nowrap rounded-full bg-gold-300/30 px-2.5 py-[3px] font-semibold text-[10px] text-foreground">
                {t('referenceEntry')}
              </span>
            ) : null}
          </div>

          <div className="text-md leading-snug">
            {character.pos ? (
              <span className="mr-1.5 text-[11px] text-muted-foreground uppercase tracking-[0.1em]">
                {character.pos}
              </span>
            ) : null}
            {character.meaning || t('capUnavailable')}
          </div>

          <div className="mt-1 flex gap-2 font-reading">
            {character.simplified ? (
              <span className="rounded-md bg-muted px-2.5 py-1 text-md">
                <span className="mr-[5px] text-[10px] text-muted-foreground">
                  简
                </span>
                {character.simplified}
              </span>
            ) : null}
            {character.traditional ? (
              <span className="rounded-md bg-muted px-2.5 py-1 text-md">
                <span className="mr-[5px] text-[10px] text-muted-foreground">
                  繁
                </span>
                {character.traditional}
              </span>
            ) : null}
          </div>
        </div>
      </div>

      {character.story ? (
        <p className="font-reading text-muted-foreground text-sm leading-loose">
          {character.story}
        </p>
      ) : null}
    </Card>
  );
}

export { FantiCharacterCard };
