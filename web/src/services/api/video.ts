import axios from "axios";

import { dataUrlToFile } from "@/lib/image-utils";
import { imageToDataUrl } from "@/services/image-storage";
import { buildApiUrl, defaultConfig, type AiConfig } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";
import type { ReferenceImage } from "@/types/image";
import { parseErrorPayloadAsync } from "./error-payload";

type VideoResponse = { id: string; status?: string; error?: { message?: string } };
type ApiVideoResponse = VideoResponse | { code: number; data?: VideoResponse | null; msg?: string };
type VideoGenerationOptions = { signal?: AbortSignal; pollIntervalMs?: number; maxPollAttempts?: number; maxWaitMs?: number };

function aiApiUrl(config: AiConfig, path: string) {
    return config.channelMode === "remote" ? `/api/v1${path}` : buildApiUrl(config.baseUrl, path);
}

function aiHeaders(config: AiConfig) {
    const token = useUserStore.getState().token;
    return config.channelMode === "remote" ? (token ? { Authorization: `Bearer ${token}` } : undefined) : { Authorization: `Bearer ${config.apiKey}` };
}

function refreshRemoteUser(config: AiConfig) {
    if (config.channelMode === "remote") void useUserStore.getState().hydrateUser();
}

export async function requestVideoGeneration(config: AiConfig, prompt: string, references: ReferenceImage[] = [], options: VideoGenerationOptions = {}) {
    const model = resolveVideoModel(config);
    const pollIntervalMs = Math.max(1, options.pollIntervalMs ?? 2500);
    const maxPollAttempts = Math.max(1, options.maxPollAttempts ?? 240);
    const deadline = Date.now() + Math.max(pollIntervalMs, options.maxWaitMs ?? pollIntervalMs * maxPollAttempts);
    const body = new FormData();
    body.append("model", model);
    body.append("prompt", prompt);
    body.append("seconds", normalizeVideoSeconds(config.videoSeconds));
    if (normalizeVideoSize(config.size)) body.append("size", normalizeVideoSize(config.size)!);
    body.append("resolution_name", normalizeVideoResolution(config.vquality));
    body.append("preset", "normal");
    const files = await Promise.all(references.slice(0, 7).map(async (image) => dataUrlToFile({ ...image, dataUrl: await imageToDataUrl(image) })));
    files.forEach((file) => body.append("input_reference[]", file));
    try {
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), body, { headers: aiHeaders(config), signal: options.signal })).data);
        if (!created.id) throw new Error("视频接口没有返回任务 ID");
        for (let attempt = 0; ; attempt++) {
            throwIfAborted(options.signal);
            const video = unwrapVideoResponse((await axios.get<ApiVideoResponse>(aiApiUrl(config, `/videos/${created.id}`), { headers: aiHeaders(config), params: config.channelMode === "remote" ? { model } : undefined, signal: options.signal })).data);
            if (video.status === "completed") break;
            if (video.status === "failed" || video.status === "cancelled") throw new Error(video.error?.message || "视频生成失败");
            if (attempt + 1 >= maxPollAttempts || Date.now() >= deadline) throw new Error("视频生成超时");
            await waitForNextPoll(Math.min(pollIntervalMs, Math.max(1, deadline - Date.now())), options.signal);
        }
        const content = await axios.get<Blob>(aiApiUrl(config, `/videos/${created.id}/content`), { headers: aiHeaders(config), params: config.channelMode === "remote" ? { model } : undefined, responseType: "blob", signal: options.signal });
        await assertVideoBlob(content.data);
        refreshRemoteUser(config);
        return content.data;
    } catch (error) {
        throw new Error(await readAxiosError(error, "视频生成失败"));
    }
}

function resolveVideoModel(config: AiConfig) {
    const model = config.model.trim();
    const videoModel = config.videoModel.trim();
    if (model && model !== config.imageModel && model !== defaultConfig.model) return model;
    return videoModel || model;
}

function normalizeVideoSeconds(value: string) {
    const seconds = Math.floor(Number(value) || 6);
    return String(Math.max(1, Math.min(20, seconds)));
}

function normalizeVideoSize(value: string) {
    if (value === "auto") return null;
    const size = value || "1280x720";
    if (/^\d+x\d+$/.test(size)) return size;
    return ["9:16", "2:3", "3:4"].includes(size) ? "720x1280" : "1280x720";
}

function normalizeVideoResolution(value: string) {
    if (value === "low") return "480p";
    if (value === "auto" || value === "high" || value === "medium") return "720p";
    const resolution = value.replace(/p$/i, "") || "720";
    return `${resolution}p`;
}

function unwrapVideoResponse(payload: ApiVideoResponse): VideoResponse {
    if (!payload) throw new Error("接口没有返回视频任务");
    if ("id" in payload) return payload;
    if ("code" in payload && typeof payload.code === "number") {
        if (payload.code !== 0) throw new Error(payload.msg || "请求失败");
        if (!payload.data) throw new Error("接口没有返回视频任务");
        return payload.data;
    }
    throw new Error("接口没有返回视频任务");
}

async function readAxiosError(error: unknown, fallback: string) {
    if (axios.isAxiosError<{ error?: { message?: string }; msg?: string; code?: number }>(error)) {
        const axiosError = error as { code?: string; name?: string };
        if (axiosError.code === "ERR_CANCELED" || axiosError.name === "CanceledError") return "视频生成已取消";
        const responseData = await parseErrorPayloadAsync(error.response?.data);
        return responseData?.msg || responseData?.error?.message || (error.response?.status ? `${fallback}：${error.response.status}` : fallback);
    }
    return error instanceof Error ? error.message : fallback;
}

async function assertVideoBlob(blob: Blob) {
    const contentType = blob.type.toLowerCase();
    const shouldInspect = contentType.includes("json") || ((contentType === "" || contentType.includes("text") || contentType.includes("octet-stream")) && blob.size <= 64 * 1024);
    if (!shouldInspect) return;
    let payload: { code?: number; msg?: string; error?: { message?: string } };
    try {
        const text = (await blob.text()).trim();
        if (!text.startsWith("{")) return;
        payload = JSON.parse(text) as { code?: number; msg?: string; error?: { message?: string } };
    } catch {
        return;
    }
    if (payload.error?.message) throw new Error(payload.error.message);
    if (payload.msg) throw new Error(payload.msg);
    if (typeof payload.code === "number" && payload.code !== 0) throw new Error(payload.msg || "视频下载失败");
    throw new Error("视频下载失败");
}

function waitForNextPoll(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        if (signal?.aborted) {
            reject(new Error("视频生成已取消"));
            return;
        }
        const timer = setTimeout(() => {
            signal?.removeEventListener("abort", onAbort);
            resolve();
        }, ms);
        const onAbort = () => {
            clearTimeout(timer);
            reject(new Error("视频生成已取消"));
        };
        signal?.addEventListener("abort", onAbort, { once: true });
    });
}

function throwIfAborted(signal?: AbortSignal) {
    if (signal?.aborted) throw new Error("视频生成已取消");
}
