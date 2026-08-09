import { create } from '@bufbuild/protobuf';
import { ConnectError, createRouterTransport } from '@connectrpc/connect';
import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, test, vi } from 'vitest';

import { SpeakableCard } from '@/components/study/speakable-card';
import {
  type GetSpeakableSummaryResponse,
  GetSpeakableSummaryResponseSchema,
  StudyService,
} from '@/gen/fanti/v1/study_pb';
import { useLocaleStore } from '@/i18n/locale';

vi.mock('@/lib/speak', () => ({ speak: vi.fn() }));

beforeEach(() => {
  useLocaleStore.setState({ locale: 'en' });
});

function renderCard(response?: GetSpeakableSummaryResponse) {
  const transport = createRouterTransport(({ service }) => {
    service(StudyService, {
      getSpeakableSummary: () => {
        if (!response) {
          throw ConnectError.from(new Error('corpus offline'));
        }
        return response;
      },
    });
  });

  const rootRoute = createRootRoute({ component: SpeakableCard });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory(),
  });

  render(
    <TransportProvider transport={transport}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>,
  );
}

test('shows unlocked count, sentences, and topics', async () => {
  renderCard(
    create(GetSpeakableSummaryResponseSchema, {
      unlockedCount: 12,
      totalCount: 6000,
      sentences: [{ id: 1n, traditional: '妳好。', english: 'Hello.' }],
      almostUnlocked: [],
      topics: ['street'],
    }),
  );

  await waitFor(() => {
    expect(screen.getByText('What you can say now')).toBeVisible();
  });
  expect(screen.getByText('12')).toBeVisible();
  expect(screen.getByText('妳好。')).toBeVisible();
  expect(
    screen.getByRole('button', { name: 'Pronounce 妳好。' }),
  ).toBeVisible();
  expect(screen.getByText('Street survival')).toBeVisible();
});

test('zero unlocked: motivational copy plus nearest challenge with character links', async () => {
  renderCard(
    create(GetSpeakableSummaryResponseSchema, {
      unlockedCount: 0,
      totalCount: 6000,
      sentences: [],
      almostUnlocked: [
        {
          id: 2n,
          traditional: '妳好。',
          english: 'Hello.',
          missingCharacters: ['妳', '好'],
        },
      ],
      topics: [],
    }),
  );

  await waitFor(() => {
    expect(
      screen.getByText('Learn your first characters to unlock real sentences.'),
    ).toBeVisible();
  });
  expect(screen.getByText('Almost there')).toBeVisible();
  expect(screen.getByRole('link', { name: 'Learn 妳' })).toBeVisible();
  expect(screen.getByRole('link', { name: 'Learn 好' })).toBeVisible();
});

test('shows a retryable error state when the summary fails', async () => {
  renderCard();

  await waitFor(() => {
    expect(screen.getByText('corpus offline', { exact: false })).toBeVisible();
  });
  expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
});
