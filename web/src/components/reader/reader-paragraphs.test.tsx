import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';

import { ReaderParagraphs } from '@/components/reader/reader-paragraphs';
import {
  ParagraphSchema,
  SentenceSpanSchema,
  TokenSchema,
} from '@/gen/fanti/v1/book_pb';

test('dictionary tokens are keyboard-operable inline buttons', async () => {
  const user = userEvent.setup();
  const onTokenTap = vi.fn();
  const paragraph = create(ParagraphSchema, {
    tokens: [
      create(TokenSchema, {
        text: '馬',
        pinyin: 'mǎ',
        character: 'characters/馬',
      }),
      create(TokenSchema, { text: '。' }),
    ],
    sentences: [create(SentenceSpanSchema, { start: 0, end: 2 })],
  });

  render(
    <ReaderParagraphs
      paragraphs={[paragraph]}
      size={19}
      lineHeight={1.8}
      fontFamily="serif"
      pinyin="all"
      onTokenTap={onTokenTap}
    />,
  );

  const token = screen.getByRole('button', { name: 'Look up 馬' });
  await user.tab();
  expect(token).toHaveFocus();
  await user.keyboard('{Enter}');
  expect(onTokenTap).toHaveBeenCalledWith({
    characterName: 'characters/馬',
    sentence: '馬。',
  });
});
