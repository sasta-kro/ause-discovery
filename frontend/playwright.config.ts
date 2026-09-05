import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  retries: 0,
  reporter: 'line',
  use: {
    baseURL: process.env.AUSE_E2E_BASE_URL ?? 'http://localhost:8088/ause-discovery/',
    trace: 'retain-on-failure',
  },
})
