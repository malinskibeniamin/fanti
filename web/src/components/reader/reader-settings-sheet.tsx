import { useShallow } from 'zustand/react/shallow';

import { BottomSheet } from '@/components/fanti/bottom-sheet';
import { Chip } from '@/components/fanti/chip';
import { SectionLabel } from '@/components/fanti/section-label';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useLocale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';
import {
  READER_FONT_VAR,
  type ReaderFont,
  type ReaderLineHeight,
  useReaderPrefs,
} from '@/stores/reader-prefs';

import { PINYIN_LABEL_KEY } from './pinyin';

const FONT_CHIPS: ReadonlyArray<{ font: ReaderFont; label: string }> = [
  { font: 'serif', label: '宋體' },
  { font: 'kai', label: '文楷' },
];

const LINE_HEIGHT_CHIPS: ReadonlyArray<{
  value: ReaderLineHeight;
  labelKey: StringKey;
}> = [
  { value: 1.6, labelKey: 'lineTight' },
  { value: 2, labelKey: 'lineNormal' },
  { value: 2.4, labelKey: 'lineLoose' },
];

const PINYIN_CHIPS = ['off', 'hints', 'all'] as const;

const STEPPER_CLASS =
  'size-10 cursor-pointer rounded-md border-none bg-muted font-reading text-foreground outline-none transition-colors duration-(--duration-fast) hover:bg-border focus-visible:ring-3 focus-visible:ring-ring/50';

interface ReaderSettingsSheetProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Reader typography settings — size steppers, font, pinyin mode, line
 * height, and the Traditional/Simplified switch. Writes straight to the
 * persisted reader-prefs store so the open reader re-renders live.
 */
function ReaderSettingsSheet({ open, onClose }: ReaderSettingsSheetProps) {
  const { t, tGloss } = useLocale();
  const prefs = useReaderPrefs(
    useShallow((state) => ({
      size: state.size,
      font: state.font,
      pinyin: state.pinyin,
      lineHeight: state.lineHeight,
      traditional: state.traditional,
      increaseSize: state.increaseSize,
      decreaseSize: state.decreaseSize,
      setFont: state.setFont,
      setPinyin: state.setPinyin,
      setLineHeight: state.setLineHeight,
      setTraditional: state.setTraditional,
    })),
  );

  return (
    <BottomSheet
      open={open}
      onClose={onClose}
      ariaLabel={t('rdSettings')}
      className="flex flex-col gap-[18px]"
    >
      <SectionLabel gloss={tGloss('rdSettings')}>
        {t('rdSettings')}
      </SectionLabel>

      <div className="flex items-center justify-between">
        <span className="font-medium text-sm">{t('size')}</span>
        <div className="flex items-center gap-3">
          <Button
            variant="unstyled"
            size="unstyled"
            type="button"
            aria-label="Decrease text size"
            onClick={prefs.decreaseSize}
            className={`${STEPPER_CLASS} text-[15px]`}
          >
            A−
          </Button>
          <span className="w-[42px] text-center text-sm tabular-nums">
            {prefs.size}px
          </span>
          <Button
            variant="unstyled"
            size="unstyled"
            type="button"
            aria-label="Increase text size"
            onClick={prefs.increaseSize}
            className={`${STEPPER_CLASS} text-[18px]`}
          >
            A＋
          </Button>
        </div>
      </div>

      <div className="flex items-center justify-between gap-3">
        <span className="font-medium text-sm">{t('font')}</span>
        <div className="flex gap-2">
          {FONT_CHIPS.map((chip) => (
            <Chip
              key={chip.font}
              selected={prefs.font === chip.font}
              onClick={() => prefs.setFont(chip.font)}
              style={{ fontFamily: READER_FONT_VAR[chip.font] }}
            >
              {chip.label}
            </Chip>
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between gap-3">
          <span className="font-medium text-sm">{t('pinyinL')}</span>
          <div className="flex gap-2">
            {PINYIN_CHIPS.map((mode) => (
              <Chip
                key={mode}
                selected={prefs.pinyin === mode}
                onClick={() => prefs.setPinyin(mode)}
              >
                {t(PINYIN_LABEL_KEY[mode])}
              </Chip>
            ))}
          </div>
        </div>
        <span className="text-[11px] text-muted-foreground">
          {t('pinyinSub')}
        </span>
      </div>

      <div className="flex items-center justify-between gap-3">
        <span className="font-medium text-sm">{t('lineL')}</span>
        <div className="flex gap-2">
          {LINE_HEIGHT_CHIPS.map((chip) => (
            <Chip
              key={chip.value}
              selected={prefs.lineHeight === chip.value}
              onClick={() => prefs.setLineHeight(chip.value)}
            >
              {t(chip.labelKey)}
            </Chip>
          ))}
        </div>
      </div>

      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="font-medium text-sm">{t('tradLabel')}</div>
          <div className="text-[11px] text-muted-foreground">
            {t('tradSub')}
          </div>
        </div>
        <Switch
          aria-label={t('tradLabel')}
          checked={prefs.traditional}
          onCheckedChange={(checked) => prefs.setTraditional(checked)}
          className="data-checked:bg-accent"
        />
      </div>
    </BottomSheet>
  );
}

export { ReaderSettingsSheet };
