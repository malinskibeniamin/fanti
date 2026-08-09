import { TransportProvider } from '@connectrpc/connect-query';
import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';

import { queryClient, transport } from './api/transport';
import { router } from './router';
import './index.css';

function App() {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  );
}

export default App;
