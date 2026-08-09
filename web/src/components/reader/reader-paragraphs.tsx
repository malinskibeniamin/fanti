import { Button } from '@/components/ui/button';
import type { Paragraph, Token } from '@/gen/fanti/v1/book_pb';
import type { PinyinMode } from '@/stores/reader-prefs';

/** Payload for a tap on a dictionary-linked token. */
export interface TokenTapTarget {
  /** Resource name, e.g. characters/馬. */
  characterName: string;
  /** The sentence containing the tapped token. */
  sentence: string;
}

interface ReaderParagraphsProps {
  paragraphs: Paragraph[];
  /** Body font size in px. */
  size: number;
  lineHeight: number;
  /** CSS font-family value, e.g. var(--font-reading). */
  fontFamily: string;
  pinyin: PinyinMode;
  onTokenTap: (target: TokenTapTarget) => void;
}

/** The sentence span covering a token, joined back into text. */
export function sentenceAt(paragraph: Paragraph, tokenIndex: number): string {
  const span = paragraph.sentences.find(
    (candidate) => tokenIndex >= candidate.start && tokenIndex < candidate.end,
  );
  const tokens = span
    ? paragraph.tokens.slice(span.start, span.end)
    : paragraph.tokens;
  return tokens.map((token) => token.text).join('');
}

function showsPinyin(token: Token, mode: PinyinMode): boolean {
  if (token.pinyin === '') {
    return false;
  }
  if (mode === 'all') {
    return true;
  }
  return mode === 'hints' && token.character !== '';
}

/* The design's glowing dictionary-token wash: gold fill + gold bottom hairline. */
const LINKED_TOKEN_STYLE: React.CSSProperties = {
  background: 'color-mix(in srgb, var(--gold-300) 30%, transparent)',
  boxShadow: '0 1px 0 color-mix(in srgb, var(--gold-500) 55%, transparent)',
};

/**
 * Tokenized chapter body. Every token renders as ruby text; the pinyin
 * annotation stays in the DOM in all modes (display:none when hidden) so
 * toggling modes never reflows the layout structure. Dictionary-linked
 * tokens glow gold and open the dictionary sheet by pointer or keyboard.
 */
function ReaderParagraphs({
  paragraphs,
  size,
  lineHeight,
  fontFamily,
  pinyin,
  onTokenTap,
}: ReaderParagraphsProps) {
  return (
    <div>
      {paragraphs.map((paragraph, paragraphIndex) => (
        <p
          // biome-ignore lint/suspicious/noArrayIndexKey: chapter paragraphs are static positional content
          key={paragraphIndex}
          className="mb-[18px] last:mb-0"
          style={{ fontSize: `${size}px`, lineHeight, fontFamily }}
        >
          {paragraph.tokens.map((token, tokenIndex) => {
            const linked = token.character !== '';
            const ruby = (
              <ruby>
                {token.text}
                <rt
                  className="select-none font-normal font-ui text-[10px] text-muted-foreground tracking-normal"
                  style={
                    showsPinyin(token, pinyin) ? undefined : { display: 'none' }
                  }
                >
                  {token.pinyin}
                </rt>
              </ruby>
            );

            if (linked) {
              return (
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  // biome-ignore lint/suspicious/noArrayIndexKey: tokens are static positional content within a chapter
                  key={tokenIndex}
                  aria-label={`Look up ${token.text}`}
                  onClick={() =>
                    onTokenTap({
                      characterName: token.character,
                      sentence: sentenceAt(paragraph, tokenIndex),
                    })
                  }
                  className="inline-flex h-auto rounded-[4px] border-0 px-px py-0 align-baseline font-inherit text-inherit hover:bg-gold-300/30"
                  style={LINKED_TOKEN_STYLE}
                >
                  {ruby}
                </Button>
              );
            }

            return (
              <span
                // biome-ignore lint/suspicious/noArrayIndexKey: tokens are static positional content within a chapter
                key={tokenIndex}
              >
                {ruby}
              </span>
            );
          })}
        </p>
      ))}
    </div>
  );
}

export { ReaderParagraphs };
