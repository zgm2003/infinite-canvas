import { describe, expect, it } from "vitest";

import { resolveMissingStorageFallback } from "./storage-fallback";

describe("resolveMissingStorageFallback", () => {
    it("does not reuse stale blob URLs when local storage is missing", () => {
        expect(resolveMissingStorageFallback("blob:http://localhost/stale")).toBe("");
    });

    it("keeps non-blob fallback URLs", () => {
        expect(resolveMissingStorageFallback("https://example.test/image.png")).toBe("https://example.test/image.png");
        expect(resolveMissingStorageFallback("data:image/png;base64,xxx")).toBe("data:image/png;base64,xxx");
    });
});
