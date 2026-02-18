import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.ts",
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL: "http://localhost:3009",
    screenshot: "on",
    viewport: { width: 1280, height: 900 },
  },
  outputDir: "./screenshots",
  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],
});
