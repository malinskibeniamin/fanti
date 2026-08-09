import { SpeakButton } from '@/components/fanti/speak-button';
import type { ExampleSentence } from '@/gen/fanti/v1/common_pb';

interface ExampleSentencesProps {
  examples: ExampleSentence[];
}

/**
 * The bilingual example-sentence list shared by the character page's
 * sentences tab and the flashcard answer face. The HSK pill sits inline
 * with the sentence so unlevelled corpus rows keep the same height as
 * curated ones.
 */
function ExampleSentences({ examples }: ExampleSentencesProps) {
  // Distinct corpus rows can normalize to identical display text, so the
  // key carries an occurrence count to stay unique.
  const seen = new Map<string, number>();
  const keyed = examples.map((example) => {
    const occurrence = (seen.get(example.chinese) ?? 0) + 1;
    seen.set(example.chinese, occurrence);
    return { example, key: `${example.chinese}#${occurrence}` };
  });

  return (
    <ul className="m-0 list-none p-0">
      {keyed.map(({ example, key }) => (
        <li
          key={key}
          className="flex flex-col gap-1 border-foreground/7 border-t py-3 first:border-t-0"
        >
          <div className="flex items-center gap-2">
            <span className="font-reading text-[17px] leading-loose">
              {example.chinese}
            </span>
            <SpeakButton text={example.chinese} iconClassName="size-3.5" />
            {example.hskLevel > 0 ? (
              <span className="whitespace-nowrap rounded-full bg-gold-300/26 px-2 py-0.5 text-[10px] text-foreground tabular-nums">
                HSK {example.hskLevel}
              </span>
            ) : null}
          </div>
          <span className="text-muted-foreground text-xs">
            {example.english}
          </span>
        </li>
      ))}
    </ul>
  );
}

export { ExampleSentences };
