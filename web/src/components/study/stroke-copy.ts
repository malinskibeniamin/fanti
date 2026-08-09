import { LOCALE_IDX, type Locale } from '@/i18n/locale';

const STROKE_COPY = {
  invalid: [
    'The stroke data has an invalid format.',
    '筆順資料格式無效。',
    '笔顺数据格式无效。',
  ],
  loadError: [
    'Could not load stroke animation',
    '無法載入筆順動畫',
    '无法加载笔顺动画',
  ],
  hintNext: ['Hint next stroke', '提示下一筆', '提示下一笔'],
  hintRecall: ['Use a Recall hint', '使用回想提示', '使用回想提示'],
  hintVisual: ['Show visual hint', '顯示圖像提示', '显示图像提示'],
  drawFromMemory: ['Draw from memory', '憑記憶默寫', '凭记忆默写'],
  difficulty: ['Writing difficulty', '書寫難度', '书写难度'],
  difficultyLoading: [
    'Loading writing difficulty',
    '正在載入書寫難度',
    '正在加载书写难度',
  ],
  difficultyGuided: ['Guided', '引導', '引导'],
  difficultyMastery: ['Mastery', '精通', '精通'],
  difficultyRecall: ['Recall', '回想', '回想'],
  continue: ['Continue', '繼續', '继续'],
  checkingStrokes: ['Checking your strokes', '正在檢查筆畫', '正在检查笔画'],
  gradeFail: ['Keep practicing', '繼續練習', '继续练习'],
  gradeError: ['Could not grade your strokes', '無法評分筆畫', '无法评分笔画'],
  gradePass: ['Correct', '答對了', '答对了'],
  loading: ['Loading stroke order', '正在載入筆順', '正在加载笔顺'],
  mode: ['Stroke learning mode', '筆順學習模式', '笔顺学习模式'],
  next: ['Next stroke', '下一筆', '下一笔'],
  pause: ['Pause', '暫停', '暂停'],
  play: ['Play', '播放', '播放'],
  practice: ['Practice writing', '練習書寫', '练习书写'],
  practiceLoadError: [
    'Could not load memory practice',
    '無法載入默寫練習',
    '无法加载默写练习',
  ],
  practiceNoAnimation: [
    'Practice without animation',
    '不看動畫直接練習',
    '不看动画直接练习',
  ],
  practiceNoHints: [
    'Practice without hints',
    '不使用提示直接練習',
    '不使用提示直接练习',
  ],
  practiceStopped: [
    'Memory practice stopped',
    '默寫練習已停止',
    '默写练习已停止',
  ],
  practiceUnavailable: [
    'Memory practice is unavailable.',
    '目前無法進行默寫練習。',
    '目前无法进行默写练习。',
  ],
  replay: ['Replay', '重新播放', '重新播放'],
  startMemory: ['Start memory practice', '開始默寫練習', '开始默写练习'],
  unavailable: [
    'Stroke animation is not available for this character.',
    '這個字目前沒有筆順動畫。',
    '这个字目前没有笔顺动画。',
  ],
  watch: ['Watch stroke order', '觀看筆順', '观看笔顺'],
  watchOnce: [
    'Watch the stroke order once, then write it from memory.',
    '先看一次筆順，再憑記憶默寫。',
    '先看一次笔顺，再凭记忆默写。',
  ],
} as const satisfies Record<string, readonly [string, string, string]>;

type StrokeCopyKey = keyof typeof STROKE_COPY;

function strokeCopy(locale: Locale, key: StrokeCopyKey) {
  return STROKE_COPY[key][LOCALE_IDX[locale]];
}

export { strokeCopy };
