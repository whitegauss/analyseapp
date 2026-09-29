import type { Preview } from "@storybook/nextjs-vite";
import { sb } from "storybook/test";

// Tailwind, so components render as they do in the app.
import "../app/globals.css";

// Server Actions run on the server, reading the session and calling the Go
// API; in a story there is neither. Mocked whole (not spied), so a story
// that submits one records the call instead of running it, and can assert
// on the FormData it was handed with `mocked(...)` (KAN-55).
sb.mock(import("../app/experiments/actions.ts"));

const preview: Preview = {};

export default preview;
