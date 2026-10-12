import { describe, expect, it } from "vitest";

import { getGreeting } from "./greeting";

describe("getGreeting", () => {
    it.each([5, 8, 11])("returns Good morning at %i h", (hour) => {
        expect(getGreeting(hour)).toBe("Good morning");
    });

    it.each([12, 15, 17])("returns Good afternoon at %i h", (hour) => {
        expect(getGreeting(hour)).toBe("Good afternoon");
    });

    it.each([18, 21, 23, 0, 4])("returns Good evening at %i h", (hour) => {
        expect(getGreeting(hour)).toBe("Good evening");
    });
});
