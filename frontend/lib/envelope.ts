// The Go API's {data, error, meta} envelope (PDR.md section 8), and the one
// decision made about every response: is this the data, or an error?
//
// Kept apart from lib/api.ts so the decision can be tested without a
// Supabase session or a fetch (KAN-50). lib/api.ts re-exports all of this,
// so callers keep importing from "@/lib/api".

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
 * The data from a Go API response, or a GoApiError for a failed one.
 *
 * Failed means either the HTTP status says so or the envelope carries an
 * error -- both are checked, so neither a 200 with an error in the body nor
 * a 5xx with an empty error slips through as data. A missing code or
 * message falls back to "unknown_error" / "HTTP <status>" (only a missing
 * one: an empty message is kept as it is).
 *
 * Success returns `data` as it is, null and empty arrays included.
 */
export function unwrapEnvelope<T>(
  status: number,
  ok: boolean,
  body: Envelope<T>,
): T | null {
  if (!ok || body.error) {
    throw new GoApiError(
      status,
      body.error?.code ?? "unknown_error",
      body.error?.message ?? `HTTP ${status}`,
    );
  }
  return body.data;
}
