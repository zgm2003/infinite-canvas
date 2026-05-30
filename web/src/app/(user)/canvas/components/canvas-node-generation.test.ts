import { describe, expect, it } from "vitest";

import { hydrateReferenceImages } from "./canvas-node-generation";
import type { ReferenceImage } from "@/types/image";

const reference = (image: Partial<ReferenceImage> = {}): ReferenceImage => ({
    id: "image-1",
    name: "reference.png",
    type: "image/png",
    dataUrl: "blob:http://localhost/stale",
    storageKey: "image:missing",
    ...image,
});

describe("hydrateReferenceImages", () => {
    it("rejects missing reference images instead of returning empty data urls", async () => {
        await expect(hydrateReferenceImages([reference()], async () => "")).rejects.toThrow("参考图片已丢失");
    });

    it("keeps hydrated reference images when a data url is available", async () => {
        const hydrated = await hydrateReferenceImages([reference()], async () => "data:image/png;base64,xxx");

        expect(hydrated).toEqual([{ ...reference(), dataUrl: "data:image/png;base64,xxx" }]);
    });
});
