import path from "node:path";
import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig } from "vitest/config";
import { unitProject } from "./vitest.unit.config.mjs";

// Both projects: `npm run test`, CI and scripts/test.sh run everything.
// `npm run test:unit` runs vitest.unit.config.mts alone, which is faster.
export default defineConfig({
  test: {
    coverage: {
      provider: "v8",
      // `include` is what makes untested files count. Without it v8 reports
      // only files a test happened to import, which flatters the percentage and
      // hides whole untested trees. (Vitest 4 dropped the old `all` flag; the
      // include list now carries that meaning.)
      include: [
        "app/**/*.{ts,tsx}",
        "components/**/*.{ts,tsx}",
        "lib/**/*.{ts,tsx}",
      ],
      exclude: ["**/*.test.{ts,tsx}", "**/*.stories.{ts,tsx}"],
      reporter: ["text-summary"],
    },
    projects: [
      // Pure-function tests in node.
      { extends: true, ...unitProject },
      // Every story's play function, in headless Chromium (KAN-51): a story
      // is both a catalogue entry and a component test. The browser comes
      // from Playwright -- `npx playwright install chromium` once.
      {
        extends: true,
        plugins: [
          storybookTest({
            configDir: path.join(import.meta.dirname, ".storybook"),
          }),
        ],
        test: {
          name: "storybook",
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({}),
            instances: [{ browser: "chromium" }],
          },
        },
      },
    ],
  },
});
