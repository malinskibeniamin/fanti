import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Baby,
  Ban,
  BookOpen,
  Box,
  Brain,
  BriefcaseBusiness,
  CalendarDays,
  Check,
  CircleDot,
  CircleUserRound,
  Clock3,
  DoorOpen,
  Earth,
  Eye,
  Footprints,
  Hand,
  Handshake,
  Heart,
  House,
  Landmark,
  Link,
  type LucideIcon,
  Map as MapIcon,
  MapPin,
  MessageCircle,
  Minus,
  Mountain,
  Move,
  PackageCheck,
  Route,
  Scale,
  Sparkles,
  Sprout,
  Square,
  Sun,
  Target,
  TreePine,
  Users,
  Waves,
  Waypoints,
} from '@/components/icons';
import { LOCALE_IDX, useLocale } from '@/i18n/locale';

type IconKey =
  | 'above'
  | 'arrive'
  | 'arrow'
  | 'baby'
  | 'ban'
  | 'book'
  | 'box'
  | 'brain'
  | 'calendar'
  | 'check'
  | 'clock'
  | 'country'
  | 'door'
  | 'down'
  | 'earth'
  | 'eye'
  | 'footsteps'
  | 'hand'
  | 'handshake'
  | 'heart'
  | 'home'
  | 'link'
  | 'map'
  | 'message'
  | 'minus'
  | 'mountain'
  | 'move'
  | 'package'
  | 'person'
  | 'place'
  | 'route'
  | 'scale'
  | 'sparkles'
  | 'sprout'
  | 'square'
  | 'sun'
  | 'target'
  | 'tree'
  | 'up'
  | 'users'
  | 'waves'
  | 'waypoints'
  | 'work';

interface MnemonicEntry {
  glyph: string;
  parts: readonly string[];
  partEntries: readonly { id: string; part: string }[];
  icon: IconKey;
  gloss: readonly [string, string, string];
  cue?: readonly [string, string, string];
}

type RawMnemonic = readonly [
  string,
  readonly string[],
  IconKey,
  readonly [string, string, string],
];

const ICONS: Record<IconKey, LucideIcon> = {
  above: ArrowUp,
  arrive: MapPin,
  arrow: ArrowRight,
  baby: Baby,
  ban: Ban,
  book: BookOpen,
  box: Box,
  brain: Brain,
  calendar: CalendarDays,
  check: Check,
  clock: Clock3,
  country: Landmark,
  door: DoorOpen,
  down: ArrowDown,
  earth: Earth,
  eye: Eye,
  footsteps: Footprints,
  hand: Hand,
  handshake: Handshake,
  heart: Heart,
  home: House,
  link: Link,
  map: MapIcon,
  message: MessageCircle,
  minus: Minus,
  mountain: Mountain,
  move: Move,
  package: PackageCheck,
  person: CircleUserRound,
  place: CircleDot,
  route: Route,
  scale: Scale,
  sparkles: Sparkles,
  sprout: Sprout,
  square: Square,
  sun: Sun,
  target: Target,
  tree: TreePine,
  up: ArrowUp,
  users: Users,
  waves: Waves,
  waypoints: Waypoints,
  work: BriefcaseBusiness,
};

const TOP_100_ENTRIES = [
  ['的', ['白', '勺'], 'target', ['linking particle', '連接詞', '连接词']],
  ['一', ['一'], 'minus', ['one line', '一條線', '一条线']],
  ['是', ['日', '疋'], 'check', ['the right path', '正確之路', '正确之路']],
  ['不', ['不'], 'ban', ['a firm no', '堅定說不', '坚定说不']],
  [
    '了',
    ['乛', '亅'],
    'check',
    ['something completed', '事情完成', '事情完成'],
  ],
  ['在', ['才', '土'], 'place', ['being at a place', '身在此地', '身在此地']],
  ['人', ['人'], 'person', ['a walking person', '行走的人', '行走的人']],
  ['有', ['有'], 'package', ['something in hand', '手中所有', '手中所有']],
  ['我', ['扌', '戈'], 'hand', ['my own hand', '自己的手', '自己的手']],
  ['他', ['亻', '也'], 'person', ['another person', '另一個人', '另一个人']],
  [
    '這',
    ['辶', '言'],
    'arrive',
    ['words arriving here', '話來到這裡', '话来到这里'],
  ],
  ['個', ['亻', '固'], 'person', ['one individual', '一個個體', '一个个体']],
  ['們', ['亻', '門'], 'users', ['people at one gate', '眾人同門', '众人同门']],
  ['中', ['口', '丨'], 'target', ['the exact center', '正中紅心', '正中红心']],
  [
    '來',
    ['木', '从'],
    'arrive',
    ['people coming through a tree', '人從樹旁來', '人从树旁来'],
  ],
  [
    '上',
    ['⺊', '一'],
    'above',
    ['a mark above the line', '線條之上', '线条之上'],
  ],
  [
    '大',
    ['一', '人'],
    'person',
    ['a person stretching wide', '張開雙臂的人', '张开双臂的人'],
  ],
  ['為', ['為'], 'work', ['acting for a purpose', '為目標行動', '为目标行动']],
  [
    '和',
    ['禾', '口'],
    'handshake',
    ['grain and mouths in harmony', '禾與口相和', '禾与口相和'],
  ],
  [
    '國',
    ['囗', '或'],
    'country',
    ['a realm inside borders', '疆界中的國土', '疆界中的国土'],
  ],
  [
    '地',
    ['土', '也'],
    'earth',
    ['the ground beneath us', '腳下土地', '脚下土地'],
  ],
  [
    '到',
    ['至', '刂'],
    'arrive',
    ['reaching the destination', '抵達終點', '抵达终点'],
  ],
  [
    '以',
    ['以'],
    'link',
    ['using one thing by another', '以此連彼', '以此连彼'],
  ],
  [
    '說',
    ['言', '兌'],
    'message',
    ['words spoken aloud', '開口說話', '开口说话'],
  ],
  [
    '時',
    ['日', '寺'],
    'clock',
    ['sunlight marking time', '日光報時', '日光报时'],
  ],
  [
    '要',
    ['覀', '女'],
    'target',
    ['the thing you need', '心中所要', '心中所要'],
  ],
  [
    '就',
    ['京', '尤'],
    'arrive',
    ['arriving right then', '就在此刻', '就在此刻'],
  ],
  [
    '出',
    ['屮', '凵'],
    'door',
    ['a sprout leaving a box', '幼芽破框而出', '幼芽破框而出'],
  ],
  ['會', ['會'], 'users', ['people meeting together', '眾人相會', '众人相会']],
  ['可', ['丁', '口'], 'check', ['permission granted', '點頭許可', '点头许可']],
  ['也', ['也'], 'link', ['one more thing too', '也加一件', '也加一件']],
  [
    '你',
    ['亻', '尔'],
    'person',
    ['the person before me', '眼前的你', '眼前的你'],
  ],
  [
    '對',
    ['业', '羊', '寸'],
    'target',
    ['two sides aligned', '兩邊相對', '两边相对'],
  ],
  [
    '生',
    ['一', '土'],
    'sprout',
    ['life growing from earth', '生命破土', '生命破土'],
  ],
  [
    '能',
    ['厶', '⺼', '匕', '匕'],
    'sparkles',
    ['stored ability', '蓄積能力', '蓄积能力'],
  ],
  ['而', ['而'], 'link', ['ideas joined together', '前後相連', '前后相连']],
  [
    '子',
    ['了', '一'],
    'baby',
    ['a child with open arms', '張臂的孩子', '张臂的孩子'],
  ],
  ['那', ['那'], 'arrive', ['a point over there', '指向那邊', '指向那边']],
  [
    '得',
    ['彳', '旦', '寸'],
    'package',
    ['walking away with a prize', '走去取得', '走去取得'],
  ],
  [
    '于',
    ['二', '亅'],
    'route',
    ['a path leading toward', '通往某處', '通往某处'],
  ],
  [
    '著',
    ['艹', '者'],
    'target',
    ['a move placed on top', '落下一著', '落下一着'],
  ],
  [
    '下',
    ['一', '卜'],
    'down',
    ['a mark below the line', '線條之下', '线条之下'],
  ],
  ['自', ['自'], 'person', ['pointing to oneself', '指向自己', '指向自己']],
  ['之', ['之'], 'arrow', ['a stroke moving onward', '一筆向前', '一笔向前']],
  [
    '年',
    ['年'],
    'calendar',
    ['the yearly harvest', '一年一收成', '一年一收成'],
  ],
  [
    '過',
    ['辶', '咼'],
    'route',
    ['crossing along a road', '沿路走過', '沿路走过'],
  ],
  ['髮', ['髟', '犮'], 'waves', ['long flowing hair', '飄動長髮', '飘动长发']],
  ['后', ['后'], 'country', ['a crowned queen', '戴冠之后', '戴冠之后']],
  ['作', ['亻', '乍'], 'work', ['a person at work', '人在工作', '人在工作']],
  ['裡', ['衤', '里'], 'box', ['the inside lining', '衣物裡層', '衣物里层']],
  ['用', ['用'], 'hand', ['a useful tool in hand', '手中工具', '手中工具']],
  [
    '道',
    ['辶', '首'],
    'route',
    ['a head leading down the road', '首領引路', '首领引路'],
  ],
  [
    '行',
    ['彳', '亍'],
    'footsteps',
    ['steps in two directions', '左右腳步', '左右脚步'],
  ],
  ['所', ['户', '斤'], 'place', ['a marked place', '標出的場所', '标出的场所']],
  [
    '然',
    ['肰', '灬'],
    'check',
    ['a bright confirmation', '火光確認', '火光确认'],
  ],
  [
    '傢',
    ['亻', '家'],
    'home',
    ['a person with household things', '人與傢俱', '人与家具'],
  ],
  [
    '種',
    ['禾', '重'],
    'sprout',
    ['a heavy seed in grain', '禾中種子', '禾中种子'],
  ],
  [
    '事',
    ['一', '口', '彐', '亅'],
    'work',
    ['a task held together', '手中事情', '手中事情'],
  ],
  [
    '成',
    ['丁', '戈'],
    'check',
    ['the final successful stroke', '完成一擊', '完成一击'],
  ],
  ['方', ['方'], 'square', ['four square directions', '四方端正', '四方端正']],
  [
    '多',
    ['夕', '夕'],
    'sparkles',
    ['evening doubled into many', '兩夕成多', '两夕成多'],
  ],
  [
    '經',
    ['糹', '巠'],
    'book',
    ['thread binding a classic', '絲線裝經', '丝线装经'],
  ],
  [
    '麼',
    ['麻', '幺'],
    'message',
    ['a tiny questioning sound', '細小語尾', '细小语尾'],
  ],
  [
    '去',
    ['土', '厶'],
    'arrow',
    ['leaving the ground behind', '離地而去', '离地而去'],
  ],
  [
    '法',
    ['氵', '去'],
    'scale',
    ['water flowing by a rule', '水依方法流', '水依方法流'],
  ],
  [
    '學',
    ['學'],
    'book',
    ['hands learning under a roof', '屋下動手學', '屋下动手学'],
  ],
  [
    '如',
    ['女', '口'],
    'link',
    ['words comparing two things', '開口比作', '开口比作'],
  ],
  [
    '都',
    ['者', '阝'],
    'users',
    ['everyone gathered in the city', '眾人聚城', '众人聚城'],
  ],
  [
    '同',
    ['凡', '口'],
    'handshake',
    ['one shared opening', '同出一口', '同出一口'],
  ],
  [
    '現',
    ['王', '見'],
    'eye',
    ['something precious coming into view', '寶玉出現眼前', '宝玉出现眼前'],
  ],
  [
    '噹',
    ['口', '當'],
    'message',
    ['a bell saying ding-dong', '鐘聲噹噹', '钟声当当'],
  ],
  [
    '沒',
    ['沒'],
    'ban',
    ['something sinking from sight', '沉沒不見', '沉没不见'],
  ],
  [
    '動',
    ['重', '力'],
    'move',
    ['strength moving a heavy load', '用力推重物', '用力推重物'],
  ],
  ['面', ['面'], 'eye', ['a face looking forward', '正面人臉', '正面人脸']],
  [
    '起',
    ['走', '己'],
    'up',
    ['yourself rising to walk', '自己起身走', '自己起身走'],
  ],
  [
    '看',
    ['手', '目'],
    'eye',
    ['a hand shading the eyes', '手搭目上看', '手搭目上看'],
  ],
  [
    '定',
    ['宀', '疋'],
    'home',
    ['a steady step under a roof', '屋下站定', '屋下站定'],
  ],
  [
    '天',
    ['一', '大'],
    'sun',
    ['the sky above a great person', '大人頭上天', '大人头上天'],
  ],
  [
    '分',
    ['八', '刀'],
    'scale',
    ['a knife dividing in two', '一刀分兩半', '一刀分两半'],
  ],
  [
    '還',
    ['辶', '睘'],
    'route',
    ['circling back along the road', '繞路還回', '绕路还回'],
  ],
  [
    '進',
    ['辶', '隹'],
    'arrow',
    ['a bird advancing on the road', '隹沿路前進', '隹沿路前进'],
  ],
  [
    '好',
    ['女', '子'],
    'heart',
    ['woman and child together', '女與子相好', '女与子相好'],
  ],
  [
    '小',
    ['亅', '八'],
    'sparkles',
    ['tiny drops around a hook', '小點散開', '小点散开'],
  ],
  [
    '部',
    ['咅', '阝'],
    'box',
    ['one section beside another', '分成部門', '分成部门'],
  ],
  [
    '其',
    ['甘', '一', '八'],
    'package',
    ['a particular thing set apart', '指出其中之一', '指出其中之一'],
  ],
  [
    '些',
    ['此', '二'],
    'sparkles',
    ['a few beside this', '此處一些', '此处一些'],
  ],
  ['主', ['丶', '王'], 'country', ['a crowned master', '王上加冠', '王上加冠']],
  [
    '樣',
    ['木', '羕'],
    'tree',
    ['a wooden pattern to copy', '木製樣板', '木制样板'],
  ],
  [
    '理',
    ['王', '里'],
    'waypoints',
    ['veins running through jade', '玉中紋理', '玉中纹理'],
  ],
  ['心', ['心'], 'heart', ['the beating heart', '一顆心', '一颗心']],
  ['她', ['女', '也'], 'person', ['the woman being named', '指向她', '指向她']],
  [
    '本',
    ['木', '一'],
    'tree',
    ['a line marking the root', '木下標根', '木下标根'],
  ],
  [
    '前',
    ['丷', '一', '刖'],
    'arrow',
    ['the space directly ahead', '向前一步', '向前一步'],
  ],
  [
    '開',
    ['門', '开'],
    'door',
    ['two hands opening a gate', '雙手開門', '双手开门'],
  ],
  [
    '但',
    ['亻', '旦'],
    'sun',
    ['a person pausing at dawn', '人立旦日', '人立旦日'],
  ],
  [
    '因',
    ['囗', '大'],
    'link',
    ['a person enclosed by a cause', '原因圍住人', '原因围住人'],
  ],
  [
    '只',
    ['口', '八'],
    'minus',
    ['only one mouth remaining', '只留一口', '只留一口'],
  ],
  [
    '從',
    ['彳', '从', '足'],
    'footsteps',
    ['people following footsteps', '眾人循足跡', '众人循足迹'],
  ],
  [
    '想',
    ['相', '心'],
    'brain',
    ['an image resting on the heart', '心中有相', '心中有相'],
  ],
  [
    '實',
    ['宀', '貫'],
    'package',
    ['a full store beneath a roof', '屋下充實', '屋下充实'],
  ],
] as const satisfies readonly RawMnemonic[];

const FEATURED_ENTRIES = [
  [
    '勢',
    ['埶', '力'],
    'sprout',
    ['a seedling growing with strength', '幼苗蓄力成勢', '幼苗蓄力成势'],
  ],
  [
    '馬',
    ['馬'],
    'move',
    ['a horse with mane and four hooves', '鬃毛與四蹄', '鬃毛与四蹄'],
  ],
  [
    '愛',
    ['爫', '冖', '心', '夂'],
    'heart',
    ['a heart sheltered at the center', '中心藏著一顆心', '中心藏着一颗心'],
  ],
  [
    '龍',
    ['立', '月', '⺒'],
    'sparkles',
    ['a crowned dragon rising', '昂首擺尾的龍', '昂首摆尾的龙'],
  ],
  [
    '書',
    ['聿', '曰'],
    'book',
    ['a hand writing spoken words', '手執筆記下話語', '手执笔记下话语'],
  ],
] as const satisfies readonly RawMnemonic[];

function toEntry(value: RawMnemonic): MnemonicEntry {
  const occurrences = new Map<string, number>();
  return {
    glyph: value[0],
    parts: value[1],
    partEntries: value[1].map((part) => {
      const occurrence = occurrences.get(part) ?? 0;
      occurrences.set(part, occurrence + 1);
      return { id: `${part}-${occurrence}`, part };
    }),
    icon: value[2],
    gloss: value[3],
    cue:
      value[0] === '的'
        ? [
            'A white spoon points to the target: the linking particle 的.',
            '白色湯勺指向目標，記住連接詞「的」。',
            '白色汤勺指向目标，记住连接词“的”。',
          ]
        : undefined,
  };
}

const MNEMONICS = new Map(
  [...TOP_100_ENTRIES, ...FEATURED_ENTRIES].map((value) => {
    const entry = toEntry(value);
    return [entry.glyph, entry] as const;
  }),
);

const TOP_100_GLYPHS = TOP_100_ENTRIES.map(([glyph]) => glyph);

function visualMnemonicFor(glyph: string) {
  return MNEMONICS.get(glyph);
}

function VisualMnemonic({ glyph }: { glyph: string }) {
  const { locale } = useLocale();
  const entry = visualMnemonicFor(glyph);
  if (!entry) {
    return null;
  }

  const localeIndex = LOCALE_IDX[locale];
  const gloss = entry.gloss[localeIndex];
  const componentList = entry.parts.join('＋');
  const cue =
    entry.cue?.[localeIndex] ??
    (locale === 'en'
      ? `See ${componentList} as ${gloss} to remember ${glyph}.`
      : locale === 'tc'
        ? `把「${componentList}」看成${gloss}，記住「${glyph}」。`
        : `把“${componentList}”看成${gloss}，记住“${glyph}”。`);
  const Icon = ICONS[entry.icon];

  return (
    <figure
      role="img"
      aria-label={cue}
      className="w-full max-w-[360px] overflow-hidden rounded-xl border border-gold-300/45 bg-[linear-gradient(145deg,color-mix(in_srgb,var(--gold-300)_24%,var(--card)),var(--card))] p-4 shadow-hairline"
    >
      <div aria-hidden="true" className="relative h-36">
        <Icon
          strokeWidth={1.35}
          className="absolute left-1/2 top-1/2 size-28 -translate-x-1/2 -translate-y-1/2 text-accent/45"
        />
        <span className="absolute inset-0 flex items-center justify-center font-display text-[108px] text-reading-foreground/85 leading-none">
          {glyph}
        </span>
        <div className="absolute inset-x-0 bottom-0 flex justify-center gap-1.5">
          {entry.partEntries.map(({ id, part }) => (
            <span
              key={id}
              className="flex min-w-9 items-center justify-center rounded-full bg-card/90 px-2 py-1 font-display text-lg shadow-hairline backdrop-blur-sm"
            >
              {part}
            </span>
          ))}
        </div>
      </div>
      <figcaption
        aria-hidden="true"
        className="mt-2 text-center text-muted-foreground text-sm leading-normal"
      >
        {cue}
      </figcaption>
    </figure>
  );
}

export { TOP_100_GLYPHS, VisualMnemonic, visualMnemonicFor };
