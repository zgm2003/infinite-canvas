import { describe, expect, it } from "vitest";

import { clearFinishedRunningNodeId, runWithRunningNodeId } from "./running-node-state";

describe("clearFinishedRunningNodeId", () => {
    it("clears the running node when the finished task still owns it", () => {
        expect(toIds(clearFinishedRunningNodeId(new Set(["node-a"]), "node-a"))).toEqual([]);
    });

    it("keeps other running nodes when one of multiple tasks finishes", () => {
        expect(toIds(clearFinishedRunningNodeId(new Set(["node-a", "node-b"]), "node-b"))).toEqual(["node-a"]);
    });

    it("keeps an empty set when nothing is running", () => {
        expect(toIds(clearFinishedRunningNodeId(new Set(), "node-a"))).toEqual([]);
    });
});

describe("runWithRunningNodeId", () => {
    it("clears the running node even when preparation fails", async () => {
        let current = new Set<string>();
        const setRunningNodeId = (value: Set<string> | ((current: Set<string>) => Set<string>)) => {
            current = typeof value === "function" ? value(current) : value;
        };

        await expect(
            runWithRunningNodeId("node-a", setRunningNodeId, async () => {
                throw new Error("hydrate failed");
            }),
        ).rejects.toThrow("hydrate failed");

        expect(toIds(current)).toEqual([]);
    });

    it("does not clear other running nodes after one task finishes", async () => {
        let current = new Set<string>();
        const setRunningNodeId = (value: Set<string> | ((current: Set<string>) => Set<string>)) => {
            current = typeof value === "function" ? value(current) : value;
        };

        await runWithRunningNodeId("node-a", setRunningNodeId, async () => {
            current = new Set([...current, "node-b"]);
        });

        expect(toIds(current)).toEqual(["node-b"]);
    });
});

function toIds(ids: Set<string>) {
    return Array.from(ids).sort();
}
