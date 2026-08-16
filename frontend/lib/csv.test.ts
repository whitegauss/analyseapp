import { describe, expect, it } from "vitest";
import { buildCsvRow, csvEscape } from "./csv";

describe("csvEscape", () => {
  it("leaves plain fields untouched", () => {
    expect(csvEscape("hello")).toBe("hello");
    expect(csvEscape("1.5")).toBe("1.5");
  });

  it("quotes fields containing a comma", () => {
    expect(csvEscape("a,b")).toBe('"a,b"');
  });

  it("quotes fields containing a newline", () => {
    expect(csvEscape("a\nb")).toBe('"a\nb"');
    expect(csvEscape("a\r\nb")).toBe('"a\r\nb"');
  });

  it("quotes and doubles internal quotes", () => {
    expect(csvEscape('say "hi"')).toBe('"say ""hi"""');
  });
});

describe("buildCsvRow", () => {
  it("joins fields with commas", () => {
    expect(buildCsvRow(["x", "y", "y_error"])).toBe("x,y,y_error");
  });

  it("stringifies numbers", () => {
    expect(buildCsvRow([1, 2.5, -3])).toBe("1,2.5,-3");
  });

  it("escapes fields that need it within a row", () => {
    expect(buildCsvRow(["温度, ℃", "圧力"])).toBe('"温度, ℃",圧力');
  });
});
