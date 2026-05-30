export function resolveMissingStorageFallback(fallback = "") {
    return fallback.startsWith("blob:") ? "" : fallback;
}
