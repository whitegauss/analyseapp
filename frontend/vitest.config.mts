import path from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "."),
    },
  },
  test: {
    environment: "node",
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
  },
});
