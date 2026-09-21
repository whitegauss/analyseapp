// Where the app's configuration comes from, and what happens when it is
// missing.
//
// The Go API validates its own configuration at startup and refuses to run
// without it (KAN-62/KAN-53). The frontend had no equivalent: every reader
// was `process.env.X!` or `?? "http://localhost:8080"`, which on a
// misconfigured deployment produces either "supabaseUrl is required" from
// inside a library, or a connection refused to a localhost that does not
// exist in that environment. Both leave the operator guessing.

/** Where the Go API answers when nothing has been configured. */
const LOCAL_API_URL = "http://localhost:8080";

/**
 * True when running on Vercel, which sets this for every build and every
 * function invocation. Used here only to decide how loudly to complain
 * about missing configuration -- there is no localhost fallback that could
 * possibly be right there.
 */
function onVercel(): boolean {
  return Boolean(process.env.VERCEL);
}

/**
 * The Go API's base URL, for server-side calls.
 *
 * Named "internal" because the browser never calls it: every request goes
 * through a Server Component or Server Action (see lib/api.ts). On a
 * container host that means a private address like `http://api:8080`; on
 * Vercel there is no private network to the Go API, so it is a public
 * HTTPS URL and the API's own JWT check is what protects it.
 */
export function apiBaseUrl(): string {
  const configured = process.env.API_INTERNAL_URL;
  if (configured) {
    return configured;
  }
  if (onVercel()) {
    // Falling back to localhost here would turn a missing setting into a
    // connection error on every page, which reads like the Go API is down
    // rather than like it was never pointed at.
    throw new Error(
      "API_INTERNAL_URL is not set. On Vercel the Go API is not on localhost -- " +
        "set it to the API's public HTTPS base URL in the project's environment variables.",
    );
  }
  return LOCAL_API_URL;
}

/**
 * Reads one of the Supabase settings the browser also receives.
 *
 * These are inlined at build time, so an unset one is baked into the bundle
 * as `undefined` and only surfaces later, from inside @supabase/ssr. The
 * check costs nothing and names the variable that is actually missing.
 */
function requireSupabaseEnv(name: string, value: string | undefined): string {
  if (!value) {
    throw new Error(
      `${name} is not set. Supabase Auth cannot be configured without it; ` +
        "on Vercel it has to exist at build time as well as at runtime, because " +
        "NEXT_PUBLIC_* values are inlined into the bundle.",
    );
  }
  return value;
}

export function supabaseUrl(): string {
  // Referenced as a full literal rather than through a variable: Next.js
  // replaces `process.env.NEXT_PUBLIC_*` by textual substitution at build
  // time, so a computed key would not be inlined at all.
  return requireSupabaseEnv(
    "NEXT_PUBLIC_SUPABASE_URL",
    process.env.NEXT_PUBLIC_SUPABASE_URL,
  );
}

export function supabaseAnonKey(): string {
  return requireSupabaseEnv(
    "NEXT_PUBLIC_SUPABASE_ANON_KEY",
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY,
  );
}
