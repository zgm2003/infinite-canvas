import { CanvasNodeType, type CanvasAssistantSession, type CanvasNodeData, type CanvasNodeMetadata } from "../types";

export type UploadedCanvasImage = {
    url: string;
    storageKey: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
};

export type HydrateCanvasImagesDeps = {
    resolveImageUrl: (storageKey?: string, fallback?: string) => Promise<string>;
    resolveMediaUrl: (storageKey?: string, fallback?: string) => Promise<string>;
    uploadImage: (input: string | Blob) => Promise<UploadedCanvasImage>;
};

export function imageMetadata(image: UploadedCanvasImage): CanvasNodeMetadata {
    return { content: image.url, storageKey: image.storageKey, status: "success", naturalWidth: image.width, naturalHeight: image.height, bytes: image.bytes, mimeType: image.mimeType };
}

export async function hydrateCanvasImages(nodes: CanvasNodeData[], deps: HydrateCanvasImagesDeps) {
    return Promise.all(
        nodes.map(async (node) => {
            try {
                return await hydrateCanvasNodeImage(node, deps);
            } catch {
                return markMediaHydrationError(node);
            }
        }),
    );
}

export async function hydrateCanvasAssistantImages(sessions: CanvasAssistantSession[], deps: HydrateCanvasImagesDeps) {
    const hydrateItem = async <T extends { dataUrl?: string; storageKey?: string }>(item: T) => {
        try {
            if (item.storageKey) return { ...item, dataUrl: await deps.resolveImageUrl(item.storageKey, item.dataUrl) };
            if (item.dataUrl?.startsWith("data:image/")) return { ...item, ...assistantImageMetadata(await deps.uploadImage(item.dataUrl)) };
        } catch {
            return item;
        }
        return item;
    };
    return Promise.all(
        sessions.map(async (session) => ({
            ...session,
            messages: await Promise.all(
                session.messages.map(async (message) => ({
                    ...message,
                    references: await Promise.all((message.references || []).map(hydrateItem)),
                    images: await Promise.all((message.images || []).map(hydrateItem)),
                })),
            ),
        })),
    );
}

async function hydrateCanvasNodeImage(node: CanvasNodeData, deps: HydrateCanvasImagesDeps) {
    const content = node.metadata?.content;
    if (node.type === CanvasNodeType.Video && node.metadata?.storageKey) return { ...node, metadata: { ...node.metadata, content: requireResolvedMedia(await deps.resolveMediaUrl(node.metadata.storageKey, content)) } };
    if (node.type !== CanvasNodeType.Image) return node;
    if (node.metadata?.storageKey) return { ...node, metadata: { ...node.metadata, content: requireResolvedMedia(await deps.resolveImageUrl(node.metadata.storageKey, content || "")) } };
    if (!content || !content.startsWith("data:image/")) return node;
    return { ...node, metadata: { ...node.metadata, ...imageMetadata(await deps.uploadImage(content)) } };
}

function requireResolvedMedia(url: string) {
    if (!url) throw new Error("本地媒体恢复失败");
    return url;
}

function markMediaHydrationError(node: CanvasNodeData) {
    if (node.type !== CanvasNodeType.Image && node.type !== CanvasNodeType.Video) return node;
    return { ...node, metadata: { ...node.metadata, status: "error" as const, errorDetails: "本地媒体恢复失败" } };
}

function assistantImageMetadata(image: UploadedCanvasImage) {
    return { dataUrl: image.url, storageKey: image.storageKey };
}
