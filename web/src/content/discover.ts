// Static reference content transcribed verbatim from the design prototype
// (Fanti.dc.html). Deliberately kept client-side as presentation content so
// the React Native app can share it. Tri-lingual copy is {en, tc, sc}.

import type { Locale } from '@/i18n/locale';

export interface LocalizedText {
  en: string;
  tc: string;
  sc: string;
}

/** Active-locale string for a tri-lingual content value. */
export function localized(locale: Locale, text: LocalizedText): string {
  return text[locale];
}

function tri(en: string, tc: string, sc: string): LocalizedText {
  return { en, tc, sc };
}

// ---- Topics ----

export interface TopicDef {
  id: string;
  label: LocalizedText;
}

export const TOPICS: readonly TopicDef[] = [
  { id: 'street', label: tri('Street survival', '街頭生存', '街头生存') },
  { id: 'food', label: tri('Food & ordering', '點餐吃飯', '点餐吃饭') },
  { id: 'travel', label: tri('Transit & travel', '交通旅行', '交通旅行') },
  { id: 'daily', label: tri('Daily life', '日常生活', '日常生活') },
  { id: 'work', label: tri('Office & work', '職場辦公', '职场办公') },
  { id: 'culture', label: tri('Culture & reading', '文化閱讀', '文化阅读') },
];

/** Relocation-readiness counts characters from these survival topics. */
export const SURVIVAL_TOPICS: readonly string[] = ['street', 'food', 'travel'];

// ---- Loanwords & calques ----

export interface Loanword {
  trad: string;
  simp: string;
  py: string;
  en: string;
  note: LocalizedText;
}

/** Easy wins — borrowed from English, learners already half-know these. */
export const LOAN_EN: readonly Loanword[] = [
  {
    trad: '派對',
    simp: '派对',
    py: 'pàiduì',
    en: 'party',
    note: tri(
      'Sounds just like it — 派對 is a straight sound-loan.',
      '音譯自英文 party — 聽一次就記住。',
      '音译自英文 party — 听一次就记住。',
    ),
  },
  {
    trad: '咖啡',
    simp: '咖啡',
    py: 'kāfēi',
    en: 'coffee',
    note: tri(
      'ka-fei ≈ coffee. Order one on day one.',
      '音近 coffee — 落地第一天就能點。',
      '音近 coffee — 落地第一天就能点。',
    ),
  },
  {
    trad: '巴士',
    simp: '巴士',
    py: 'bāshì',
    en: 'bus',
    note: tri(
      'ba-shi ≈ bus — common in HK/TW usage.',
      '音譯 bus — 港台常用。',
      '音译 bus — 港台常用。',
    ),
  },
  {
    trad: '沙發',
    simp: '沙发',
    py: 'shāfā',
    en: 'sofa',
    note: tri('sha-fa ≈ sofa.', '音近 sofa。', '音近 sofa。'),
  },
  {
    trad: '巧克力',
    simp: '巧克力',
    py: 'qiǎokèlì',
    en: 'chocolate',
    note: tri(
      'qiao-ke-li ≈ chocolate.',
      '音譯 chocolate。',
      '音译 chocolate。',
    ),
  },
  {
    trad: '卡拉OK',
    simp: '卡拉OK',
    py: 'kǎlā-OK',
    en: 'karaoke',
    note: tri(
      'Borrowed via Japanese — the OK is literal.',
      '經日文借入 — OK 原樣保留。',
      '经日文借入 — OK 原样保留。',
    ),
  },
];

/** Chinese that entered English. */
export const LOAN_ZH: readonly Loanword[] = [
  {
    trad: '好久不見',
    simp: '好久不见',
    py: 'hǎo jiǔ bú jiàn',
    en: 'long time no see',
    note: tri(
      'The English phrase is a word-for-word calque of this greeting.',
      '英文 long time no see 正是逐字翻譯自這句。',
      '英文 long time no see 正是逐字翻译自这句。',
    ),
  },
  {
    trad: '颱風',
    simp: '台风',
    py: 'táifēng',
    en: 'typhoon',
    note: tri(
      "English 'typhoon' echoes 颱風.",
      '英文 typhoon 源自 颱風。',
      '英文 typhoon 源自 台风。',
    ),
  },
  {
    trad: '豆腐',
    simp: '豆腐',
    py: 'dòufu',
    en: 'tofu',
    note: tri(
      'Tofu came from 豆腐 via Japanese.',
      'tofu 經日文借自 豆腐。',
      'tofu 经日文借自 豆腐。',
    ),
  },
  {
    trad: '功夫',
    simp: '功夫',
    py: 'gōngfu',
    en: 'kung fu',
    note: tri(
      "Kung fu is 功夫 — literally 'skill earned through effort'.",
      'kung fu 即 功夫 — 下工夫練出的本事。',
      'kung fu 即 功夫 — 下工夫练出的本事。',
    ),
  },
  {
    trad: '丟臉',
    simp: '丢脸',
    py: 'diū liǎn',
    en: 'lose face',
    note: tri(
      "'Lose face' is a calque of 丟臉.",
      '英文 lose face 逐字譯自 丟臉。',
      '英文 lose face 逐字译自 丢脸。',
    ),
  },
];

// ---- Regional vocabulary ----

export type RegionCode = 'cn' | 'tw' | 'hk' | 'sg';

export const REGION_LABELS: Record<RegionCode, LocalizedText> = {
  cn: tri('China', '中國', '中国'),
  tw: tri('Taiwan', '台灣', '台湾'),
  hk: tri('Hong Kong', '香港', '香港'),
  sg: tri('Singapore', '新加坡', '新加坡'),
};

export interface RegionalVariant {
  region: RegionCode;
  word: string;
  py: string;
}

export interface RegionalWord {
  en: string;
  variants: readonly RegionalVariant[];
  note: LocalizedText;
}

/** One thing, many names — like lift vs elevator. */
export const REGIONAL: readonly RegionalWord[] = [
  {
    en: 'metro / subway',
    variants: [
      { region: 'cn', word: '地铁', py: 'dìtiě' },
      { region: 'tw', word: '捷運', py: 'jiéyùn' },
      { region: 'hk', word: '地鐵', py: 'dìtiě' },
      { region: 'sg', word: '地鐵 MRT', py: 'dìtiě' },
    ],
    note: tri(
      'Like tube vs subway in English.',
      '如同英式 tube 對美式 subway。',
      '如同英式 tube 对美式 subway。',
    ),
  },
  {
    en: 'hotel',
    variants: [
      { region: 'cn', word: '酒店', py: 'jiǔdiàn' },
      { region: 'tw', word: '飯店', py: 'fàndiàn' },
      { region: 'hk', word: '酒店', py: 'jiǔdiàn' },
    ],
    note: tri(
      'Careful: in Taiwan 酒店 can mean a hostess bar.',
      '注意：在台灣「酒店」另有聲色場所之意。',
      '注意：在台湾「酒店」另有声色场所之意。',
    ),
  },
  {
    en: 'taxi',
    variants: [
      { region: 'cn', word: '出租车', py: 'chūzūchē' },
      { region: 'tw', word: '計程車', py: 'jìchéngchē' },
      { region: 'hk', word: '的士', py: 'dīshì' },
      { region: 'sg', word: '德士', py: 'déshì' },
    ],
    note: tri(
      "的士 and 德士 are sound-loans of 'taxi'.",
      '的士、德士皆音譯自 taxi。',
      '的士、德士皆音译自 taxi。',
    ),
  },
  {
    en: 'software',
    variants: [
      { region: 'cn', word: '软件', py: 'ruǎnjiàn' },
      { region: 'tw', word: '軟體', py: 'ruǎntǐ' },
    ],
    note: tri(
      "The converter's vocabulary localization handles exactly this pair.",
      '轉換器的「詞彙在地化」正是處理這組。',
      '转换器的「词汇本地化」正是处理这组。',
    ),
  },
  {
    en: 'bicycle',
    variants: [
      { region: 'cn', word: '自行车', py: 'zìxíngchē' },
      { region: 'tw', word: '腳踏車', py: 'jiǎotàchē' },
      { region: 'hk', word: '單車', py: 'dānchē' },
      { region: 'sg', word: '腳車', py: 'jiǎochē' },
    ],
    note: tri(
      'Four regions, four everyday words.',
      '四地四種日常說法。',
      '四地四种日常说法。',
    ),
  },
  {
    en: 'potato',
    variants: [
      { region: 'cn', word: '土豆', py: 'tǔdòu' },
      { region: 'tw', word: '馬鈴薯', py: 'mǎlíngshǔ' },
    ],
    note: tri(
      'In Taiwan 土豆 means peanut — a classic trap.',
      '在台灣「土豆」是花生 — 經典陷阱。',
      '在台湾「土豆」是花生 — 经典陷阱。',
    ),
  },
  {
    en: 'video',
    variants: [
      { region: 'cn', word: '视频', py: 'shìpín' },
      { region: 'tw', word: '影片', py: 'yǐngpiàn' },
    ],
    note: tri(
      'Both are understood, each marks where you learned Chinese.',
      '兩者都聽得懂，但一開口就知道你在哪學的中文。',
      '两者都听得懂，但一开口就知道你在哪学的中文。',
    ),
  },
];

// ---- Proverbs & idioms ----

export interface Proverb {
  trad: string;
  simp: string;
  py: string;
  lit: string;
  fig: string;
}

export const PROVERBS: readonly Proverb[] = [
  {
    trad: '一舉兩得',
    simp: '一举两得',
    py: 'yī jǔ liǎng dé',
    lit: 'one move, two gains',
    fig: 'kill two birds with one stone',
  },
  {
    trad: '入鄉隨俗',
    simp: '入乡随俗',
    py: 'rù xiāng suí sú',
    lit: 'enter the village, follow its customs',
    fig: 'when in Rome, do as the Romans do',
  },
  {
    trad: '熟能生巧',
    simp: '熟能生巧',
    py: 'shú néng shēng qiǎo',
    lit: 'familiarity breeds skill',
    fig: 'practice makes perfect',
  },
  {
    trad: '馬到成功',
    simp: '马到成功',
    py: 'mǎ dào chéng gōng',
    lit: 'the horse arrives, success follows',
    fig: 'swift and immediate success',
  },
  {
    trad: '對牛彈琴',
    simp: '对牛弹琴',
    py: 'duì niú tán qín',
    lit: 'playing the zither to a cow',
    fig: 'wasted words on an unreceptive audience',
  },
];

// ---- Guides ----

export type GuideId = 'pinyin' | 'zhuyin' | 'typing' | 'strokes';

export const GUIDE_IDS: readonly GuideId[] = [
  'pinyin',
  'zhuyin',
  'typing',
  'strokes',
];

export function isGuideId(value: string): value is GuideId {
  return (GUIDE_IDS as readonly string[]).includes(value);
}

export const GUIDE_GLYPHS: Record<GuideId, string> = {
  pinyin: '拼',
  zhuyin: 'ㄅ',
  typing: '鍵',
  strokes: '永',
};

// ---- Pinyin ----

export interface PinyinTone {
  mark: string;
  name: LocalizedText;
  ch: string;
  py: string;
  en: string;
}

export const PY_TONES: readonly PinyinTone[] = [
  {
    mark: 'ā',
    name: tri('1st — high & level', '一聲（陰平）', '一声（阴平）'),
    ch: '媽',
    py: 'mā',
    en: 'mother',
  },
  {
    mark: 'á',
    name: tri('2nd — rising', '二聲（陽平）', '二声（阳平）'),
    ch: '麻',
    py: 'má',
    en: 'hemp',
  },
  {
    mark: 'ǎ',
    name: tri('3rd — dip & rise', '三聲（上聲）', '三声（上声）'),
    ch: '馬',
    py: 'mǎ',
    en: 'horse',
  },
  {
    mark: 'à',
    name: tri('4th — sharp fall', '四聲（去聲）', '四声（去声）'),
    ch: '罵',
    py: 'mà',
    en: 'to scold',
  },
  {
    mark: 'a',
    name: tri('Neutral — short & light', '輕聲', '轻声'),
    ch: '嗎',
    py: 'ma',
    en: 'question particle',
  },
];

export interface SoundGroup {
  g: string;
  n: LocalizedText;
}

export const PY_INITIALS: readonly SoundGroup[] = [
  {
    g: 'b p m f · d t n l · g k h',
    n: tri(
      'Close to their English sounds.',
      '與英文發音相近。',
      '与英文发音相近。',
    ),
  },
  {
    g: 'j q x',
    n: tri(
      'Tongue low, lips spread — roughly jee / chee / shee.',
      '舌面音 — 近似 jee／chee／shee。',
      '舌面音 — 近似 jee／chee／shee。',
    ),
  },
  {
    g: 'zh ch sh r',
    n: tri(
      'Retroflex — tongue curled back.',
      '翹舌音 — 舌尖後捲。',
      '翘舌音 — 舌尖后卷。',
    ),
  },
  {
    g: 'z c s',
    n: tri(
      'Flat — z = dz, c = ts. The classic trap.',
      '平舌音 — c 讀如 ts，最易誤讀。',
      '平舌音 — c 读如 ts，最易误读。',
    ),
  },
];

export const PY_FINALS: readonly SoundGroup[] = [
  {
    g: 'a o e i u ü',
    n: tri('The six simple vowels.', '六個單韻母。', '六个单韵母。'),
  },
  {
    g: 'ai ei ao ou',
    n: tri(
      'Diphthongs — glide between two vowels.',
      '複韻母 — 兩音滑動。',
      '复韵母 — 两音滑动。',
    ),
  },
  {
    g: 'an en ang eng ong er',
    n: tri(
      'Nasal and r-colored endings.',
      '鼻音與捲舌韻尾。',
      '鼻音与卷舌韵尾。',
    ),
  },
];

// ---- Zhuyin ----

export interface ZhuyinSymbol {
  z: string;
  p: string;
}

export const ZY_MAP: readonly ZhuyinSymbol[] = [
  { z: 'ㄅ', p: 'b' },
  { z: 'ㄆ', p: 'p' },
  { z: 'ㄇ', p: 'm' },
  { z: 'ㄈ', p: 'f' },
  { z: 'ㄉ', p: 'd' },
  { z: 'ㄊ', p: 't' },
  { z: 'ㄋ', p: 'n' },
  { z: 'ㄌ', p: 'l' },
  { z: 'ㄍ', p: 'g' },
  { z: 'ㄎ', p: 'k' },
  { z: 'ㄏ', p: 'h' },
  { z: 'ㄐ', p: 'j' },
  { z: 'ㄑ', p: 'q' },
  { z: 'ㄒ', p: 'x' },
  { z: 'ㄓ', p: 'zh' },
  { z: 'ㄔ', p: 'ch' },
  { z: 'ㄕ', p: 'sh' },
  { z: 'ㄖ', p: 'r' },
  { z: 'ㄗ', p: 'z' },
  { z: 'ㄘ', p: 'c' },
  { z: 'ㄙ', p: 's' },
  { z: 'ㄚ', p: 'a' },
  { z: 'ㄛ', p: 'o' },
  { z: 'ㄜ', p: 'e' },
  { z: 'ㄝ', p: 'ê' },
  { z: 'ㄞ', p: 'ai' },
  { z: 'ㄟ', p: 'ei' },
  { z: 'ㄠ', p: 'ao' },
  { z: 'ㄡ', p: 'ou' },
  { z: 'ㄢ', p: 'an' },
  { z: 'ㄣ', p: 'en' },
  { z: 'ㄤ', p: 'ang' },
  { z: 'ㄥ', p: 'eng' },
  { z: 'ㄦ', p: 'er' },
  { z: 'ㄧ', p: 'i' },
  { z: 'ㄨ', p: 'u' },
  { z: 'ㄩ', p: 'ü' },
];

export interface ZhuyinTone {
  m: string;
  n: LocalizedText;
}

export const ZY_TONES: readonly ZhuyinTone[] = [
  { m: '—', n: tri('1st tone — unmarked', '一聲 — 不標調', '一声 — 不标调') },
  { m: 'ˊ', n: tri('2nd tone — rising', '二聲', '二声') },
  { m: 'ˇ', n: tri('3rd tone — dip & rise', '三聲', '三声') },
  { m: 'ˋ', n: tri('4th tone — falling', '四聲', '四声') },
  {
    m: '˙',
    n: tri('Neutral tone — dot above', '輕聲 — 點於上方', '轻声 — 点于上方'),
  },
];

// ---- Digital input methods ----

export interface InputMethod {
  glyph: string;
  name: LocalizedText;
  desc: LocalizedText;
}

export const INPUT_METHODS: readonly InputMethod[] = [
  {
    glyph: '拼',
    name: tri('Pinyin keyboard', '拼音鍵盤', '拼音键盘'),
    desc: tri(
      'Type the romanisation (ma) and pick 馬 from the candidate bar. Default on most phones.',
      '輸入羅馬拼音（ma），從候選列選「馬」。大多數手機的預設中文鍵盤。',
      '输入罗马拼音（ma），从候选栏选「马」。大多数手机的默认中文键盘。',
    ),
  },
  {
    glyph: '注',
    name: tri('Zhuyin keyboard', '注音鍵盤', '注音键盘'),
    desc: tri(
      "Taiwan's ㄅㄆㄇ layout — type ㄇㄚˇ, tone key last.",
      '台灣常用的ㄅㄆㄇ排列 — 輸入ㄇㄚˇ，最後按聲調。',
      '台湾常用的ㄅㄆㄇ排列 — 输入ㄇㄚˇ，最后按声调。',
    ),
  },
  {
    glyph: '寫',
    name: tri('Handwriting input', '手寫輸入', '手写输入'),
    desc: tri(
      'Draw the character on the keyboard panel — rough stroke order helps recognition.',
      '在鍵盤面板上手寫 — 大致正確的筆順能提高辨識率。',
      '在键盘面板上手写 — 大致正确的笔顺能提高识别率。',
    ),
  },
  {
    glyph: '聲',
    name: tri('Voice input', '語音輸入', '语音输入'),
    desc: tri(
      'Speak the sentence — a good way to check your tones.',
      '直接口說 — 也是檢查聲調的好方法。',
      '直接口说 — 也是检查声调的好方法。',
    ),
  },
];

// ---- 永字八法 — the eight basic strokes ----

export interface StrokePrinciple {
  n: string;
  g: string;
  d: LocalizedText;
}

export const STROKES8: readonly StrokePrinciple[] = [
  {
    n: '點 diǎn',
    g: '丶',
    d: tri(
      'Dot — couch the brush, fill top to bottom.',
      '頓筆成點，由上而下。',
      '顿笔成点，由上而下。',
    ),
  },
  {
    n: '橫 héng',
    g: '一',
    d: tri(
      'Horizontal — drawn left to right.',
      '由左至右平行。',
      '由左至右平行。',
    ),
  },
  {
    n: '豎 shù',
    g: '丨',
    d: tri(
      'Vertical — falls straight down.',
      '由上而下直落。',
      '由上而下直落。',
    ),
  },
  {
    n: '鉤 gōu',
    g: '亅',
    d: tri(
      'Hook — a sharp flick ending another stroke.',
      '收筆時急轉出鋒。',
      '收笔时急转出锋。',
    ),
  },
  {
    n: '提 tí',
    g: '㇀',
    d: tri('Rise — flick up and to the right.', '向右上挑出。', '向右上挑出。'),
  },
  {
    n: '彎 wān',
    g: '乚',
    d: tri('Bend — a concave curve left or right.', '弧形彎曲。', '弧形弯曲。'),
  },
  {
    n: '撇 piě',
    g: '丿',
    d: tri(
      'Throw — falls leftwards with a slight curve.',
      '向左下撇出，略帶弧度。',
      '向左下撇出，略带弧度。',
    ),
  },
  {
    n: '捺 nà',
    g: '乀',
    d: tri(
      'Press — falls rightwards, widening at the end.',
      '向右下按出，末端漸寬。',
      '向右下按出，末端渐宽。',
    ),
  },
];

// ---- HSK → CEFR ----

export const CEFR_MAP: Record<number, string> = {
  1: 'A1',
  2: 'A2',
  3: 'B1',
  4: 'B2',
  5: 'C1',
  6: 'C2',
};
