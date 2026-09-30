import { describe, expect, it } from "vitest";
import { GoApiError, unwrapEnvelope, type Envelope } from "./envelope";

function envelope<T>(
  data: T | null,
  error: Envelope<T>["error"] = null,
): Envelope<T> {
  return { data, error, meta: {} };
}

describe("unwrapEnvelope", () => {
  it.each([
    ["an object", 200, { id: "a" }],
    ["null (a success with nothing to return)", 200, null],
    ["an array, from a create", 201, [1, 2]],
    ["an empty array (falsy, still data)", 200, []],
  ])("returns the data as it is: %s", (_, status, data) => {
    expect(unwrapEnvelope(status, true, envelope(data))).toEqual(data);
  });

  it.each([
    [
      "the envelope's error for a 404",
      404,
      false,
      { code: "not_found", message: "experiment not found" },
      { status: 404, code: "not_found", message: "experiment not found" },
    ],
    [
      "the envelope's error for a 401",
      401,
      false,
      { code: "unauthorized", message: "missing authenticated user" },
      {
        status: 401,
        code: "unauthorized",
        message: "missing authenticated user",
      },
    ],
    [
      "the status for a 429",
      429,
      false,
      { code: "rate_limited", message: "too many requests, please slow down" },
      { status: 429, code: "rate_limited" },
    ],
    // A failure whose body says nothing: both fallbacks.
    [
      "fallbacks for a 500 with no error in the body",
      500,
      false,
      null,
      { status: 500, code: "unknown_error", message: "HTTP 500" },
    ],
    // A 200 that carries an error is still an error.
    [
      "the error from a 200 whose envelope has one",
      200,
      true,
      { code: "internal_error", message: "boom" },
      { status: 200, code: "internal_error", message: "boom" },
    ],
    // Pinned: `??` falls back only for a missing message, so an empty one
    // stays empty rather than becoming "HTTP 400".
    [
      "an empty message as it is",
      400,
      false,
      { code: "invalid_body", message: "" },
      { status: 400, code: "invalid_body", message: "" },
    ],
  ])("throws %s", (_, status, ok, error, want) => {
    let thrown: unknown;
    try {
      unwrapEnvelope(status, ok, envelope(null, error));
    } catch (e) {
      thrown = e;
    }
    // Callers branch on instanceof, so the class matters, not just the shape.
    expect(thrown).toBeInstanceOf(GoApiError);
    expect(thrown).toMatchObject(want);
  });
});

describe("GoApiError", () => {
  it("keeps the status, code and message", () => {
    const err = new GoApiError(404, "not_found", "project not found");
    expect(err).toMatchObject({
      status: 404,
      code: "not_found",
      message: "project not found",
    });
  });

  it("is an Error, with a stack", () => {
    const err = new GoApiError(500, "internal_error", "boom");
    expect(err).toBeInstanceOf(Error);
    expect(err.stack).toContain("boom");
    // Pinned: the name is not set, so it is Error's.
    expect(err.name).toBe("Error");
  });
});
