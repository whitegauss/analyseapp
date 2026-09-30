import { createClient } from "@/lib/supabase/server";
import { unwrapEnvelope, type Envelope } from "@/lib/envelope";

export { GoApiError, unwrapEnvelope, type Envelope } from "@/lib/envelope";

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
  return unwrapEnvelope(res.status, res.ok, body);
}
