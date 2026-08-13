import { createClient } from "@/lib/supabase/server";

// Mirrors the Go API's {data, error, meta} envelope (PDR.md section 8).
export type Envelope<T> = {
  data: T | null;
  error: { code: string; message: string } | null;
  meta: Record<string, unknown>;
};

export class GoApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

/**
 * Calls the Go API on behalf of the current Supabase session. The browser
 * never talks to the Go API directly -- this always runs server-side (in a
 * Server Component or Server Action), reads the session's access_token, and
 * forwards it as a Bearer token. Returns null if there is no logged-in
 * session, so callers can redirect to /login.
 */
export async function callGoApi<T>(
  path: string,
  init?: RequestInit,
): Promise<T | null> {
  const supabase = await createClient();
  const {
    data: { session },
  } = await supabase.auth.getSession();
  if (!session) {
    return null;
  }

  const apiBaseUrl = process.env.API_INTERNAL_URL ?? "http://localhost:8080";
  const res = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${session.access_token}`,
      ...init?.headers,
    },
    cache: "no-store",
  });

  const body = (await res.json()) as Envelope<T>;
  if (!res.ok || body.error) {
    throw new GoApiError(
      res.status,
      body.error?.code ?? "unknown_error",
      body.error?.message ?? `HTTP ${res.status}`,
    );
  }
  return body.data;
}
