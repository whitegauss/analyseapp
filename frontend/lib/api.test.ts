import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The session comes from Supabase's server client; each test says whether
// there is one.
const getSession = vi.fn();
vi.mock("@/lib/supabase/server", () => ({
  createClient: async () => ({ auth: { getSession } }),
}));

const { callGoApi } = await import("./api");

const fetchMock = vi.fn();

function respond(status: number, body: unknown) {
  fetchMock.mockResolvedValueOnce(
    new Response(JSON.stringify(body), { status }),
  );
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  getSession.mockResolvedValue({
    data: { session: { access_token: "token-123" } },
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  fetchMock.mockReset();
  getSession.mockReset();
});

describe("callGoApi", () => {
  // No session: the caller redirects to /login. Nothing is sent without a
  // token.
  it("returns null without a session, and calls nothing", async () => {
    getSession.mockResolvedValue({ data: { session: null } });

    await expect(callGoApi("/api/v1/projects")).resolves.toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sends the session's token, JSON, uncached", async () => {
    respond(200, { data: [], error: null, meta: {} });

    await expect(callGoApi("/api/v1/projects")).resolves.toEqual([]);

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers).toMatchObject({
      Authorization: "Bearer token-123",
      "Content-Type": "application/json",
    });
    expect(init.cache).toBe("no-store");
  });

  it("lets the caller's headers override the defaults", async () => {
    respond(200, { data: null, error: null, meta: {} });

    await callGoApi("/x", { headers: { "Content-Type": "text/csv" } });

    expect(fetchMock.mock.calls[0][1].headers["Content-Type"]).toBe("text/csv");
  });

  it("falls back to localhost:8080 without API_INTERNAL_URL", async () => {
    vi.stubEnv("API_INTERNAL_URL", undefined);
    respond(200, { data: null, error: null, meta: {} });

    await callGoApi("/api/v1/projects");

    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://localhost:8080/api/v1/projects",
    );
  });

  it("uses API_INTERNAL_URL when set", async () => {
    vi.stubEnv("API_INTERNAL_URL", "http://api:8080");
    respond(200, { data: null, error: null, meta: {} });

    await callGoApi("/api/v1/projects");

    expect(fetchMock.mock.calls[0][0]).toBe("http://api:8080/api/v1/projects");
  });

  // The envelope decision itself is unwrapEnvelope's (envelope.test.ts);
  // this is only that callGoApi hands it the response.
  it("throws the envelope's error", async () => {
    respond(404, {
      data: null,
      error: { code: "not_found", message: "project not found" },
      meta: {},
    });

    await expect(callGoApi("/api/v1/projects/x")).rejects.toMatchObject({
      status: 404,
      code: "not_found",
    });
  });
});
