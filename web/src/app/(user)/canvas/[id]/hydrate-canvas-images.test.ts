import { describe, expect, it, vi } from "vitest";

import { hydrateCanvasAssistantImages, hydrateCanvasImages, type HydrateCanvasImagesDeps } from "./hydrate-canvas-images";
import { CanvasNodeType, type CanvasAssistantSession, type CanvasNodeData } from "../types";

const baseNode = (node: Partial<CanvasNodeData> & Pick<CanvasNodeData, "id" | "type">): CanvasNodeData => ({
    title: "node",
    position: { x: 0, y: 0 },
    width: 100,
    height: 100,
    ...node,
});

const deps = (): HydrateCanvasImagesDeps => ({
    resolveImageUrl: vi.fn(async (_key, fallback = "") => fallback || "blob:image"),
    resolveMediaUrl: vi.fn(async (_key, fallback = "") => fallback || "blob:media"),
    uploadImage: vi.fn(async () => ({
        url: "blob:uploaded",
        storageKey: "image:uploaded",
        width: 640,
        height: 480,
        bytes: 123,
        mimeType: "image/png",
    })),
});

describe("hydrateCanvasImages", () => {
    it("restores storageKey-only image nodes", async () => {
        const services = deps();
        const node = baseNode({ id: "image-1", type: CanvasNodeType.Image, metadata: { storageKey: "image:key" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(services.resolveImageUrl).toHaveBeenCalledWith("image:key", "");
        expect(hydrated.metadata?.content).toBe("blob:image");
    });

    it("uses existing image content as storage fallback", async () => {
        const services = deps();
        const node = baseNode({ id: "image-1", type: CanvasNodeType.Image, metadata: { storageKey: "image:key", content: "old-url" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(services.resolveImageUrl).toHaveBeenCalledWith("image:key", "old-url");
        expect(hydrated.metadata?.content).toBe("old-url");
    });

    it("restores video nodes from media storage", async () => {
        const services = deps();
        const node = baseNode({ id: "video-1", type: CanvasNodeType.Video, metadata: { storageKey: "video:key", content: "old-video" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(services.resolveMediaUrl).toHaveBeenCalledWith("video:key", "old-video");
        expect(hydrated.metadata?.content).toBe("old-video");
    });

    it("restores storageKey-only video nodes", async () => {
        const services = deps();
        const node = baseNode({ id: "video-1", type: CanvasNodeType.Video, metadata: { storageKey: "video:key" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(services.resolveMediaUrl).toHaveBeenCalledWith("video:key", undefined);
        expect(hydrated.metadata?.content).toBe("blob:media");
    });

    it("uploads inline data image nodes and stores normalized metadata", async () => {
        const services = deps();
        const node = baseNode({ id: "image-1", type: CanvasNodeType.Image, metadata: { content: "data:image/png;base64,xxx" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(services.uploadImage).toHaveBeenCalledWith("data:image/png;base64,xxx");
        expect(hydrated.metadata).toMatchObject({
            content: "blob:uploaded",
            storageKey: "image:uploaded",
            status: "success",
            naturalWidth: 640,
            naturalHeight: 480,
            bytes: 123,
            mimeType: "image/png",
        });
    });

    it("keeps loading when one media node cannot be restored", async () => {
        const services = deps();
        vi.mocked(services.resolveImageUrl).mockRejectedValueOnce(new Error("missing blob"));
        const broken = baseNode({
            id: "image-1",
            type: CanvasNodeType.Image,
            metadata: { storageKey: "image:missing", content: "old-url", naturalWidth: 320, naturalHeight: 180, bytes: 99, mimeType: "image/png" },
        });
        const text = baseNode({ id: "text-1", type: CanvasNodeType.Text, metadata: { content: "hello" } });

        const hydrated = await hydrateCanvasImages([broken, text], services);

        expect(hydrated[0].metadata).toMatchObject({
            storageKey: "image:missing",
            content: "old-url",
            naturalWidth: 320,
            naturalHeight: 180,
            bytes: 99,
            mimeType: "image/png",
            status: "error",
            errorDetails: "本地媒体恢复失败",
        });
        expect(hydrated[1]).toBe(text);
    });

    it("marks storageKey-only image nodes as failed when storage is missing", async () => {
        const services = deps();
        vi.mocked(services.resolveImageUrl).mockResolvedValueOnce("");
        const node = baseNode({ id: "image-1", type: CanvasNodeType.Image, metadata: { storageKey: "image:missing" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(hydrated.metadata).toMatchObject({
            storageKey: "image:missing",
            status: "error",
            errorDetails: "本地媒体恢复失败",
        });
    });

    it("marks storageKey-only video nodes as failed when storage is missing", async () => {
        const services = deps();
        vi.mocked(services.resolveMediaUrl).mockResolvedValueOnce("");
        const node = baseNode({ id: "video-1", type: CanvasNodeType.Video, metadata: { storageKey: "video:missing" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(hydrated.metadata).toMatchObject({
            storageKey: "video:missing",
            status: "error",
            errorDetails: "本地媒体恢复失败",
        });
    });

    it("leaves image nodes without content or storageKey unchanged", async () => {
        const services = deps();
        const node = baseNode({ id: "image-1", type: CanvasNodeType.Image });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(hydrated).toBe(node);
        expect(services.uploadImage).not.toHaveBeenCalled();
    });

    it("leaves non-media nodes unchanged", async () => {
        const services = deps();
        const node = baseNode({ id: "text-1", type: CanvasNodeType.Text, metadata: { content: "hello" } });

        const [hydrated] = await hydrateCanvasImages([node], services);

        expect(hydrated).toBe(node);
        expect(services.resolveImageUrl).not.toHaveBeenCalled();
        expect(services.resolveMediaUrl).not.toHaveBeenCalled();
        expect(services.uploadImage).not.toHaveBeenCalled();
    });
});

describe("hydrateCanvasAssistantImages", () => {
    it("keeps loading when one assistant image cannot be restored", async () => {
        const services = deps();
        vi.mocked(services.resolveImageUrl).mockRejectedValueOnce(new Error("missing assistant image"));
        const sessions: CanvasAssistantSession[] = [
            {
                id: "session-1",
                title: "chat",
                createdAt: "now",
                updatedAt: "now",
                messages: [
                    {
                        id: "message-1",
                        role: "user",
                        mode: "ask",
                        text: "hello",
                        references: [{ id: "ref-1", type: CanvasNodeType.Image, title: "missing", storageKey: "image:missing", dataUrl: "old-ref-url" }],
                        images: [{ id: "img-1", prompt: "result", storageKey: "image:ok", dataUrl: "old-image-url" }],
                    },
                ],
            },
        ];

        const hydrated = await hydrateCanvasAssistantImages(sessions, services);

        expect(hydrated[0].messages[0].references?.[0]).toMatchObject({ storageKey: "image:missing", dataUrl: "old-ref-url" });
        expect(hydrated[0].messages[0].images?.[0].dataUrl).toBe("old-image-url");
    });
});
