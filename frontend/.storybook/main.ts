import type { StorybookConfig } from "@storybook/nextjs-vite";

// Stories double as the component tests: their play functions run in a real
// browser through @storybook/addon-vitest (the `storybook` project in
// vitest.config.mts), so a story written for the catalogue is also a test
// (KAN-36, KAN-51).
const config: StorybookConfig = {
  stories: ["../components/**/*.stories.tsx"],
  addons: ["@storybook/addon-vitest"],
  framework: "@storybook/nextjs-vite",
};

export default config;
