import { describe, expect, it, vi } from "vitest";
import axios from "axios";

import { requestImageQuestion } from "./image";
import { defaultConfig } from "@/stores/use-config-store";

vi.mock("axios", () => ({
    default: {
        post: vi.fn(),
        get: vi.fn(),
        isAxiosError: (error: unknown) => Boolean((error as { isAxiosError?: boolean })?.isAxiosError),
    },
}));

describe("requestImageQuestion", () => {
    it("uses backend JSON msg from non-2xx text responses", async () => {
        vi.mocked(axios.post).mockRejectedValueOnce({
            isAxiosError: true,
            response: {
                status: 401,
                data: `{"code":1,"data":null,"msg":"未登录或权限不足"}`,
            },
        });

        await expect(requestImageQuestion({ ...defaultConfig, channelMode: "remote" }, [{ role: "user", content: "hello" }], vi.fn())).rejects.toThrow("未登录或权限不足");
    });
});
