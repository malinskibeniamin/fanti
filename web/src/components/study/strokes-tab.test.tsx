import { create } from '@bufbuild/protobuf';
import { render, screen } from '@testing-library/react';
import { beforeEach, expect, test, vi } from 'vitest';

import { StrokesTab } from '@/components/study/strokes-tab';
import {
  CapabilityStatus,
  CharacterCapabilitiesSchema,
  CharacterGlyphSchema,
  CharacterSchema,
} from '@/gen/fanti/v1/dictionary_pb';
import { useLocaleStore } from '@/i18n/locale';

const mocks = vi.hoisted(() => ({ useQuery: vi.fn() }));

vi.mock('@connectrpc/connect-query', () => ({ useQuery: mocks.useQuery }));
vi.mock('@/components/study/stroke-learning-surface', () => ({
  StrokeLearningSurface: ({ glyph }: { glyph: string }) => (
    <div>Stroke learning {glyph}</div>
  ),
}));
vi.mock('@/lib/speak', () => ({ speak: vi.fn() }));

beforeEach(() => {
  vi.clearAllMocks();
  useLocaleStore.setState({ locale: 'en' });
  mocks.useQuery.mockReturnValue({
    data: {
      dueCards: [
        {
          character: create(CharacterSchema, {
            name: 'characters/馬',
            traditional: '馬',
            pinyin: 'mǎ',
            glyphs: [
              create(CharacterGlyphSchema, {
                glyph: '馬',
                primary: true,
                capabilities: create(CharacterCapabilitiesSchema, {
                  strokes: CapabilityStatus.AVAILABLE,
                }),
              }),
            ],
          }),
        },
      ],
    },
    isError: false,
  });
});

test('shows animated stroke learning for the selected deck character', () => {
  render(<StrokesTab />);

  expect(screen.getByText('Stroke learning 馬')).toBeVisible();
});

test('keeps characters without stroke data out of stroke practice', () => {
  mocks.useQuery.mockReturnValue({
    data: {
      dueCards: [
        {
          character: create(CharacterSchema, {
            name: 'characters/㐀',
            traditional: '㐀',
            pinyin: 'qiū',
            glyphs: [
              create(CharacterGlyphSchema, {
                glyph: '㐀',
                primary: true,
                capabilities: create(CharacterCapabilitiesSchema, {
                  strokes: CapabilityStatus.UNAVAILABLE,
                }),
              }),
            ],
          }),
        },
      ],
    },
    isError: false,
  });

  render(<StrokesTab />);

  expect(screen.queryByRole('button', { name: '㐀' })).not.toBeInTheDocument();
  expect(screen.getByText('No due characters have stroke data.')).toBeVisible();
  expect(screen.getByText('Stroke learning 永')).toBeVisible();
});
