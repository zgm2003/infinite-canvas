import { afterEach, describe, expect, it, vi } from "vitest";
import axios from "axios";

import { requestVideoGeneration } from "./video";
import { defaultConfig } from "@/stores/use-config-store";

vi.mock("axios", () => ({
    default: {
        post: vi.fn(),
        get: vi.fn(),
        isAxiosError: (error: unknown) => Boolean((error as { isAxiosError?: boolean })?.isAxiosError),
    },
}));

describe("requestVideoGeneration", () => {
    afterEach(() => {
        vi.useRealTimers();
        vi.clearAllMocks();
    });

    it("uses videoModel before the generic image model", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "completed" } })
            .mockResolvedValueOnce({ data: new Blob(["video"], { type: "video/mp4" }) });

        await requestVideoGeneration({ ...defaultConfig, channelMode: "remote", model: "gpt-image-2", videoModel: "grok-imagine-video" }, "生成一个视频");

        const body = vi.mocked(axios.post).mock.calls[0]?.[1] as FormData;
        expect(body.get("model")).toBe("grok-imagine-video");
        expect(vi.mocked(axios.get).mock.calls[0]?.[1]?.params).toEqual({ model: "grok-imagine-video" });
        expect(vi.mocked(axios.get).mock.calls[1]?.[1]?.params).toEqual({ model: "grok-imagine-video" });
    });

    it("keeps an explicit video model override instead of forcing the global videoModel", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "completed" } })
            .mockResolvedValueOnce({ data: new Blob(["video"], { type: "video/mp4" }) });

        await requestVideoGeneration({ ...defaultConfig, channelMode: "remote", model: "custom-video-model", imageModel: "gpt-image-2", videoModel: "grok-imagine-video" }, "生成一个视频");

        const body = vi.mocked(axios.post).mock.calls[0]?.[1] as FormData;
        expect(body.get("model")).toBe("custom-video-model");
        expect(vi.mocked(axios.get).mock.calls[0]?.[1]?.params).toEqual({ model: "custom-video-model" });
        expect(vi.mocked(axios.get).mock.calls[1]?.[1]?.params).toEqual({ model: "custom-video-model" });
    });

    it("uses backend JSON msg from non-2xx text responses", async () => {
        vi.mocked(axios.post).mockRejectedValueOnce({
            isAxiosError: true,
            response: {
                status: 402,
                data: `{"code":1,"data":null,"msg":"算力点不足"}`,
            },
        });

        await expect(requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频")).rejects.toThrow("算力点不足");
    });

    it("uses backend JSON msg from non-2xx blob responses", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "completed" } })
            .mockRejectedValueOnce({
                isAxiosError: true,
                response: {
                    status: 402,
                    data: new Blob([`{"code":1,"data":null,"msg":"算力点不足"}`], { type: "application/json" }),
                },
            });

        await expect(requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频")).rejects.toThrow("算力点不足");
    });

    it("rejects JSON error blobs returned as successful video content", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "completed" } })
            .mockResolvedValueOnce({ data: new Blob([`{"error":{"message":"生成失败"}}`], { type: "application/json" }) });

        await expect(requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频")).rejects.toThrow("生成失败");
    });

    it("rejects small JSON error blobs even when the content type is generic", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "completed" } })
            .mockResolvedValueOnce({ data: new Blob([`{"code":1,"msg":"算力点不足"}`], { type: "application/octet-stream" }) });

        await expect(requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频")).rejects.toThrow("算力点不足");
    });

    it("keeps the cancellation message when axios aborts a request", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get).mockRejectedValueOnce({
            isAxiosError: true,
            code: "ERR_CANCELED",
        });

        await expect(requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频")).rejects.toThrow("视频生成已取消");
    });

    it("stops polling after the configured attempt limit", async () => {
        vi.useFakeTimers();
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1" } });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-1", status: "running" } })
            .mockRejectedValueOnce(new Error("unexpected extra poll"));

        const promise = requestVideoGeneration({ ...defaultConfig, channelMode: "remote" }, "生成一个视频", [], { maxPollAttempts: 1, pollIntervalMs: 1 });
        const assertion = expect(promise).rejects.toThrow("视频生成超时");
        await vi.advanceTimersByTimeAsync(2500);

        await assertion;
    });
});
