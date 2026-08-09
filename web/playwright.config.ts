import { defineConfig, devices } from '@playwright/test';

const WEB_PORT = 3100;
const API_URL = 'http://localhost:8080';

// The specs exercise the real backend against a shared dev database, so
// they run sequentially — parallel workers would race on reviews and books.
export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e/test-results',
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: [['list']],
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: `http://localhost:${WEB_PORT}`,
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      testIgnore: /mobile\.spec\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'mobile-webkit',
      testMatch: /mobile\.spec\.ts/,
      use: { ...devices['iPhone 13'] },
    },
  ],
  webServer: [
    {
      command: `go build -o /tmp/fanti-e2e-bin ./cmd/fanti && /tmp/fanti-e2e-bin serve`,
      cwd: '../backend',
      url: `${API_URL}/healthz`,
      reuseExistingServer: true,
      timeout: 180_000,
    },
    {
      command: 'bun run dev:e2e',
      url: `http://localhost:${WEB_PORT}`,
      reuseExistingServer: true,
      timeout: 120_000,
    },
  ],
});
