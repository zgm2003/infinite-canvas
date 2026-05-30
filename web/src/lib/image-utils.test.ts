import { describe, expect, it } from "vitest";

import { dataUrlToFile } from "./image-utils";

describe("dataUrlToFile", () => {
    it("rejects empty reference images instead of creating empty files", () => {
        expect(() => dataUrlToFile({ id: "ref-1", name: "missing.png", type: "image/png", dataUrl: "" })).toThrow("参考图片已丢失");
    });

    it("creates a file from a valid data url", async () => {
        const file = dataUrlToFile({ id: "ref-1", name: "reference.png", type: "image/png", dataUrl: "data:image/png;base64,eA==" });

        expect(file.name).toBe("reference.png");
        expect(file.type).toBe("image/png");
        expect(await file.text()).toBe("x");
    });
});
