export type ApiErrorPayload = { error?: { message?: string }; msg?: string; code?: number };

export function parseErrorPayload(data: unknown): ApiErrorPayload | null {
    if (!data) return null;
    if (typeof data === "string") {
        try {
            return JSON.parse(data) as ApiErrorPayload;
        } catch {
            return null;
        }
    }
    return typeof data === "object" ? (data as ApiErrorPayload) : null;
}

export async function parseErrorPayloadAsync(data: unknown): Promise<ApiErrorPayload | null> {
    if (typeof Blob !== "undefined" && data instanceof Blob) {
        return parseErrorPayload(await data.text());
    }
    return parseErrorPayload(data);
}
