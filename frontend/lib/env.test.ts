import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { apiBaseUrl, supabaseAnonKey, supabaseUrl } from "./env";

// These read process.env, so each case sets exactly what it needs and the
// original is put back afterwards.
const ORIGINAL = { ...process.env };

beforeEach(() => {
  delete process.env.VERCEL;
  delete process.env.API_INTERNAL_URL;
  delete process.env.NEXT_PUBLIC_SUPABASE_URL;
  delete process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
});

afterEach(() => {
  process.env = { ...ORIGINAL };
});

describe("apiBaseUrl", () => {
  it("uses the configured URL when there is one", () => {
    process.env.API_INTERNAL_URL = "https://api.example.com";

    expect(apiBaseUrl()).toBe("https://api.example.com");
  });

  it("falls back to localhost off Vercel, so local dev needs no setup", () => {
    expect(apiBaseUrl()).toBe("http://localhost:8080");
  });

  it("prefers the configured URL over the fallback even locally", () => {
    // docker compose sets http://api:8080; the container has no Go API on
    // its own localhost.
    process.env.API_INTERNAL_URL = "http://api:8080";

    expect(apiBaseUrl()).toBe("http://api:8080");
  });

  it("refuses to fall back to localhost on Vercel", () => {
    // The whole point of this module. Falling back there turns a missing
    // setting into a connection error on every page, which reads like the
    // Go API is down rather than like it was never pointed at.
    process.env.VERCEL = "1";

    expect(() => apiBaseUrl()).toThrow(/API_INTERNAL_URL/);
  });

  it("is happy on Vercel once the URL is set", () => {
    process.env.VERCEL = "1";
    process.env.API_INTERNAL_URL = "https://api.example.com";

    expect(apiBaseUrl()).toBe("https://api.example.com");
  });
});

describe("the Supabase settings", () => {
  it("returns them when set", () => {
    process.env.NEXT_PUBLIC_SUPABASE_URL = "https://project.supabase.co";
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY = "anon-key";

    expect(supabaseUrl()).toBe("https://project.supabase.co");
    expect(supabaseAnonKey()).toBe("anon-key");
  });

  it("names the missing variable instead of failing inside the library", () => {
    // Unset, @supabase/ssr reports "supabaseUrl is required", which says
    // nothing about which environment variable to go and set.
    expect(() => supabaseUrl()).toThrow(/NEXT_PUBLIC_SUPABASE_URL/);
    expect(() => supabaseAnonKey()).toThrow(/NEXT_PUBLIC_SUPABASE_ANON_KEY/);
  });

  it("treats an empty string as missing", () => {
    // How an environment variable that was declared but never given a
    // value arrives -- and it is no more usable than an absent one.
    process.env.NEXT_PUBLIC_SUPABASE_URL = "";

    expect(() => supabaseUrl()).toThrow(/NEXT_PUBLIC_SUPABASE_URL/);
  });
});
