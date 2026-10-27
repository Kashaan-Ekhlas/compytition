import { describe, expect, it } from "vitest";
import {
  cn,
  formatDate,
  formatRange,
  fromDateTimeInput,
  initials,
  relativeTime,
  statusLabel,
  toDateTimeInput,
} from "./utils";

describe("cn", () => {
  it("merges conflicting tailwind classes with the last value winning", () => {
    expect(cn("px-2", "px-4")).toBe("px-4");
  });

  it("keeps conditional classes", () => {
    expect(cn("border-2", false && "hidden", "bg-surface")).toBe("border-2 bg-surface");
  });
});

describe("formatDate", () => {
  it("formats a valid ISO timestamp", () => {
    expect(formatDate("2026-03-04T10:00:00.000Z")).toBe("Mar 4, 2026");
  });

  it("returns an em dash for missing or invalid values", () => {
    expect(formatDate(null)).toBe("—");
    expect(formatDate(undefined)).toBe("—");
    expect(formatDate("not-a-date")).toBe("—");
  });
});

describe("formatRange", () => {
  it("announces missing dates", () => {
    expect(formatRange()).toBe("Dates to be announced");
  });

  it("renders a single date when only one side exists", () => {
    expect(formatRange("2026-03-04T10:00:00.000Z")).toBe("Mar 4, 2026");
  });

  it("renders both dates with an arrow", () => {
    expect(formatRange("2026-03-04T10:00:00.000Z", "2026-03-05T10:00:00.000Z")).toContain("→");
  });
});

describe("datetime-local conversion", () => {
  it("round-trips a timestamp through the input format", () => {
    const iso = "2026-03-04T10:30:00.000Z";
    const local = toDateTimeInput(iso);
    expect(local).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
    expect(fromDateTimeInput(local)).toBe(iso);
  });

  it("returns empty/null for bad input", () => {
    expect(toDateTimeInput(null)).toBe("");
    expect(toDateTimeInput("bad")).toBe("");
    expect(fromDateTimeInput("")).toBeNull();
    expect(fromDateTimeInput("bad")).toBeNull();
  });
});

describe("relativeTime", () => {
  it("returns an em dash for missing values", () => {
    expect(relativeTime(null)).toBe("—");
    expect(relativeTime("bad")).toBe("—");
  });

  it("describes past and future values", () => {
    const now = Date.now();
    expect(relativeTime(new Date(now - 2 * 86_400_000).toISOString())).toMatch(/2 days ago/);
    expect(relativeTime(new Date(now + 3 * 86_400_000).toISOString())).toMatch(/in 3 days/);
  });
});

describe("statusLabel", () => {
  it("upper-cases and humanises snake_case statuses", () => {
    expect(statusLabel("results_published")).toBe("RESULTS PUBLISHED");
    expect(statusLabel("draft")).toBe("DRAFT");
  });
});

describe("initials", () => {
  it("takes up to two initials", () => {
    expect(initials("Ada Lovelace")).toBe("AL");
    expect(initials("grace")).toBe("G");
    expect(initials("  ")).toBe("");
  });
});
