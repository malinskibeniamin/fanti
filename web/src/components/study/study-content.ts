import type { LocalizedText } from '@/gen/fanti/v1/common_pb';
import { Goal } from '@/gen/fanti/v1/study_pb';
import { LOCALE_IDX, type Locale } from '@/i18n/locale';
import type { StringKey } from '@/i18n/strings';

/** [en, tc, sc] copy triple, mirroring the shape of src/i18n/strings.ts. */
export type CopyTriple = readonly [string, string, string];

export function pickTriple(locale: Locale, triple: CopyTriple): string {
  return triple[LOCALE_IDX[locale]];
}

/** Active-locale string from a proto LocalizedText. */
export function pickLocalized(
  locale: Locale,
  text: LocalizedText | undefined,
): string {
  if (!text) {
    return '';
  }
  return locale === 'en' ? text.en : locale === 'tc' ? text.tc : text.sc;
}

const CEFR_BY_HSK: Record<number, string> = {
  1: 'A1',
  2: 'A2',
  3: 'B1',
  4: 'B2',
  5: 'C1',
  6: 'C2',
};

/** "HSK 2 · A2" pill copy; empty when unlevelled (proper names). */
export function hskCefrLabel(hskLevel: number): string {
  if (hskLevel <= 0) {
    return '';
  }
  const cefr = CEFR_BY_HSK[hskLevel];
  return cefr ? `HSK ${hskLevel} · ${cefr}` : `HSK ${hskLevel}`;
}

/** Goals the profile can hold; UNSPECIFIED falls back to PRACTICAL. */
export type KnownGoal = Goal.PRACTICAL | Goal.EXAM | Goal.READING;

export const GOAL_ORDER: readonly KnownGoal[] = [
  Goal.PRACTICAL,
  Goal.EXAM,
  Goal.READING,
];

export const GOAL_LABEL_KEY: Record<KnownGoal, StringKey> = {
  [Goal.PRACTICAL]: 'goalPractical',
  [Goal.EXAM]: 'goalExamL',
  [Goal.READING]: 'goalReading',
};

export function knownGoal(goal: Goal): KnownGoal {
  return goal === Goal.EXAM || goal === Goal.READING ? goal : Goal.PRACTICAL;
}

/** "Success looks like" bullets per goal — the design's SUCCESS constant. */
export const SUCCESS_BULLETS: Record<KnownGoal, readonly CopyTriple[]> = {
  [Goal.PRACTICAL]: [
    ['Order food without pointing', '點菜不用比手畫腳', '点菜不用比手画脚'],
    ['Read station and street signs', '看懂車站與路標', '看懂车站与路标'],
    [
      'Small talk about your day',
      '用中文聊今天過得如何',
      '用中文聊今天过得如何',
    ],
  ],
  [Goal.EXAM]: [
    ['Pass HSK 2 (300 words)', '通過 HSK 2（300 詞）', '通过 HSK 2（300 词）'],
    ['Read exam instructions calmly', '從容讀懂考題說明', '从容读懂考题说明'],
    [
      'Type 150 characters from memory',
      '憑記憶打出 150 個字',
      '凭记忆打出 150 个字',
    ],
  ],
  [Goal.READING]: [
    [
      "Finish a children's picture book",
      '讀完一本兒童繪本',
      '读完一本儿童绘本',
    ],
    [
      'Read a chapter of 三國演義 with hints',
      '靠提示讀完《三國演義》一回',
      '靠提示读完《三国演义》一回',
    ],
    [
      'Order from a menu without lookups',
      '讀菜單不用查字典',
      '读菜单不用查字典',
    ],
  ],
};

export interface EightStroke {
  /** Stroke name with pinyin, e.g. "點 diǎn". */
  name: string;
  /** The stroke glyph itself, e.g. 丶. */
  glyph: string;
  description: CopyTriple;
}

/** 永字八法 — the eight basic strokes, all present in 永. */
export const EIGHT_STROKES: readonly EightStroke[] = [
  {
    name: '點 diǎn',
    glyph: '丶',
    description: [
      'Dot — couch the brush, fill top to bottom.',
      '頓筆成點，由上而下。',
      '顿笔成点，由上而下。',
    ],
  },
  {
    name: '橫 héng',
    glyph: '一',
    description: [
      'Horizontal — drawn left to right.',
      '由左至右平行。',
      '由左至右平行。',
    ],
  },
  {
    name: '豎 shù',
    glyph: '丨',
    description: [
      'Vertical — falls straight down.',
      '由上而下直落。',
      '由上而下直落。',
    ],
  },
  {
    name: '鉤 gōu',
    glyph: '亅',
    description: [
      'Hook — a sharp flick ending another stroke.',
      '收筆時急轉出鋒。',
      '收笔时急转出锋。',
    ],
  },
  {
    name: '提 tí',
    glyph: '㇀',
    description: [
      'Rise — flick up and to the right.',
      '向右上挑出。',
      '向右上挑出。',
    ],
  },
  {
    name: '彎 wān',
    glyph: '乚',
    description: [
      'Bend — a concave curve left or right.',
      '弧形彎曲。',
      '弧形弯曲。',
    ],
  },
  {
    name: '撇 piě',
    glyph: '丿',
    description: [
      'Throw — falls leftwards with a slight curve.',
      '向左下撇出，略帶弧度。',
      '向左下撇出，略带弧度。',
    ],
  },
  {
    name: '捺 nà',
    glyph: '乀',
    description: [
      'Press — falls rightwards, widening at the end.',
      '向右下按出，末端漸寬。',
      '向右下按出，末端渐宽。',
    ],
  },
];
