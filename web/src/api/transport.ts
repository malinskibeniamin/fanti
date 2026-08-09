import { createConnectTransport } from '@connectrpc/connect-web';
import { QueryClient } from '@tanstack/react-query';

import { env } from '@/env';

const QUERY_STALE_TIME_MS = 30_000;

export const transport = createConnectTransport({
  baseUrl: env.apiBaseUrl,
});

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: QUERY_STALE_TIME_MS,
    },
  },
});
