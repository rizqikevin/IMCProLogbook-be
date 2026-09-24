import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: ".",
  testMatch: "pwa.spec.js",
  use: { baseURL: process.env.PWA_BASE_URL || "http://localhost:3000" },
});
