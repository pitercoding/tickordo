import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { formatLocalDateTime, LocalDateTime } from "./local-date-time";

const value = "2026-10-20T17:28:20Z";

describe("LocalDateTime", () => {
    it("renders a placeholder before mounting in the browser", () => {
        const html = renderToString(createElement(LocalDateTime, { value }));

        expect(html).toBe(`<time dateTime="${value}"> </time>`);
    });
});

describe("formatLocalDateTime", () => {
    it("formats the instant in the local timezone", () => {
        vi.stubEnv("TZ", "Europe/Berlin");

        expect(formatLocalDateTime(value)).toBe("20/10/2026, 19:28:20");
    });

    it("formats the same instant differently in another timezone", () => {
        vi.stubEnv("TZ", "UTC");

        expect(formatLocalDateTime(value)).toBe("20/10/2026, 17:28:20");
    });
});
