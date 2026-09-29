import path from "node:path";
import { defineConfig, type ViteUserConfig } from "vitest/config";

// The pure-function tests: node, no browser, no Storybook.
//
// `npm run test:unit` uses this file on its own, because merely having the
// storybook project in the config -- even filtered out with --project --
// costs about a second of startup (KAN-51). vitest.config.mts reuses
// `unitProject` for its `unit` project. It imports the object rather than
// pointing `extends` at this file: with `extends: "<file>"` the browser
// project running alongside failed every story with "Vitest failed to find
// the current suite".
export const unitProject = {
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "."),
    },
  },
  test: {
    name: "unit",
    environment: "node",
    include: ["**/*.test.ts"],
  },
} satisfies ViteUserConfig;

export default defineConfig(unitProject);
