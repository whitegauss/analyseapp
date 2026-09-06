import { describe, expect, it } from "vitest";
import {
  buildCsv,
  contentDispositionValue,
  csvFilename,
  type CsvExperiment,
} from "./experimentCsv";
import type { LinearRegressionResult } from "./experiment";

function makeExperiment(overrides: Partial<CsvExperiment> = {}): CsvExperiment {
  return {
    id: "exp-1",
    title: "落体実験",
    raw_data: { columns: { x: [1, 2], y: [10, 20] } },
    config: {},
    created_at: "2026-08-30T12:00:00Z",
    ...overrides,
  };
}

const regression: LinearRegressionResult = {
  slope: 2,
  intercept: 1,
  slope_stderr: 0.1,
  intercept_stderr: 0.2,
  r_squared: 0.99,
  weighted: false,
  x_log: false,
  y_log: false,
};

const lines = (csv: string) => csv.split("\r\n");

describe("buildCsv", () => {
  it("leads with the title and timestamp as comment lines", () => {
    expect(lines(buildCsv(makeExperiment(), null)).slice(0, 2)).toEqual([
      "# title: 落体実験",
      "# created_at: 2026-08-30T12:00:00Z",
    ]);
  });

  it("labels an untitled experiment rather than leaving it blank", () => {
    expect(buildCsv(makeExperiment({ title: null }), null)).toContain(
      "# title: (無題)",
    );
  });

  it("omits axis-label comments when no label is set", () => {
    expect(buildCsv(makeExperiment(), null)).not.toContain("axis_label");
  });

  it("emits axis-label comments when labels are set", () => {
    const csv = buildCsv(
      makeExperiment({ config: { x_axis_label: "t (s)", y_axis_label: "v" } }),
      null,
    );
    expect(csv).toContain("# x_axis_label: t (s)");
    expect(csv).toContain("# y_axis_label: v");
  });

  it("omits the regression comment when there is no fit", () => {
    expect(buildCsv(makeExperiment(), null)).not.toContain("# regression:");
  });

  it("records the fitted model with its uncertainties", () => {
    const csv = buildCsv(makeExperiment(), regression);
    expect(csv).toContain(
      "# regression: slope=2 ± 0.1, intercept=1 ± 0.2, " +
        "r_squared=0.99, x_log=false, y_log=false",
    );
  });

  it("omits the ± term when the fit reports no uncertainty", () => {
    // A two-point fit. "± null" would be noise and "± 0" would claim a
    // perfect measurement (KAN-57).
    const csv = buildCsv(makeExperiment(), {
      ...regression,
      slope_stderr: null,
      intercept_stderr: null,
    });
    expect(csv).toContain(
      "# regression: slope=2, intercept=1, " +
        "r_squared=0.99, x_log=false, y_log=false",
    );
    expect(csv).not.toContain("±");
  });

  it("uses the raw column keys as headers by default", () => {
    expect(lines(buildCsv(makeExperiment(), null))[2]).toBe("x,y");
  });

  it("substitutes axis labels for the x and y headers when set", () => {
    const csv = buildCsv(
      makeExperiment({ config: { x_axis_label: "t (s)", y_axis_label: "v" } }),
      null,
    );
    // two metadata lines + two axis-label lines, then the header
    expect(lines(csv)[4]).toBe("t (s),v");
  });

  it("leaves extra column headers untouched", () => {
    const csv = buildCsv(
      makeExperiment({
        raw_data: { columns: { x: [1], y: [2], y_error: [0.5] } },
        config: { x_axis_label: "t" },
      }),
      null,
    );
    expect(csv).toContain("t,y,y_error");
  });

  it("writes one row per data point", () => {
    expect(lines(buildCsv(makeExperiment(), null)).slice(3, 5)).toEqual([
      "1,10",
      "2,20",
    ]);
  });

  it("pads short columns with empty cells instead of truncating", () => {
    const csv = buildCsv(
      makeExperiment({ raw_data: { columns: { x: [1, 2, 3], y: [10] } } }),
      null,
    );
    expect(lines(csv).slice(3, 6)).toEqual(["1,10", "2,", "3,"]);
  });

  it("terminates every row with CRLF, including the last", () => {
    expect(buildCsv(makeExperiment(), null).endsWith("\r\n")).toBe(true);
  });

  it("produces only metadata and a header when there are no rows", () => {
    const csv = buildCsv(
      makeExperiment({ raw_data: { columns: { x: [], y: [] } } }),
      null,
    );
    expect(lines(csv).filter(Boolean)).toEqual([
      "# title: 落体実験",
      "# created_at: 2026-08-30T12:00:00Z",
      "x,y",
    ]);
  });

  it("quotes a header containing a comma so it stays one field", () => {
    const csv = buildCsv(
      makeExperiment({ config: { x_axis_label: "t, in seconds" } }),
      null,
    );
    expect(csv).toContain('"t, in seconds",y');
  });
});

describe("csvFilename", () => {
  it("names the file after the experiment", () => {
    expect(csvFilename("落体実験", "exp-1")).toBe("落体実験.csv");
  });

  it("falls back to the id when there is no title", () => {
    expect(csvFilename(null, "exp-1")).toBe("exp-1.csv");
  });

  it.each([
    ["blank", "   "],
    ["empty", ""],
  ])("falls back to the id when the title is %s", (_label, title) => {
    expect(csvFilename(title, "exp-1")).toBe("exp-1.csv");
  });

  it.each([
    ["a quote", 'a"b', "a_b.csv"],
    ["a forward slash", "a/b", "a_b.csv"],
    ["a backslash", "a\\b", "a_b.csv"],
    ["a newline", "a\nb", "a_b.csv"],
  ])("replaces %s that would break the header or a path", (_l, title, want) => {
    expect(csvFilename(title, "exp-1")).toBe(want);
  });

  it("trims surrounding whitespace", () => {
    expect(csvFilename("  実験  ", "exp-1")).toBe("実験.csv");
  });
});

describe("contentDispositionValue", () => {
  it("passes an ASCII name through unchanged in both parameters", () => {
    expect(contentDispositionValue("data.csv")).toBe(
      `attachment; filename="data.csv"; filename*=UTF-8''data.csv`,
    );
  });

  it("masks non-ASCII in the fallback but preserves it in filename*", () => {
    const value = contentDispositionValue("落体.csv");
    expect(value).toContain('filename="__.csv"');
    expect(value).toContain(
      `filename*=UTF-8''${encodeURIComponent("落体.csv")}`,
    );
  });

  it("percent-encodes a space so the parameter stays unquoted-safe", () => {
    expect(contentDispositionValue("a b.csv")).toContain(
      "filename*=UTF-8''a%20b.csv",
    );
  });

  // encodeURIComponent leaves these alone, but RFC 8187's attr-char set does
  // not include them. The apostrophe matters most: ext-value is
  // charset'language'value, so a bare ' hands a strict parser a third
  // delimiter.
  it.each([
    ["an apostrophe", "a'b.csv", "a%27b.csv"],
    ["parentheses", "実験(1).csv", "%E5%AE%9F%E9%A8%93%281%29.csv"],
    ["an asterisk", "star*.csv", "star%2A.csv"],
  ])("percent-encodes %s, which attr-char excludes", (_l, name, want) => {
    expect(contentDispositionValue(name)).toContain(`filename*=UTF-8''${want}`);
  });

  // Both are in attr-char, so encoding them would be needless noise.
  it.each([
    ["an exclamation mark", "ok!.csv"],
    ["a tilde", "a~b.csv"],
  ])("leaves %s unencoded, since attr-char allows it", (_label, name) => {
    expect(contentDispositionValue(name)).toContain(`filename*=UTF-8''${name}`);
  });
});
