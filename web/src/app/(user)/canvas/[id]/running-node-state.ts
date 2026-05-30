export function clearFinishedRunningNodeId(current: Set<string>, finishedNodeId: string) {
    if (!current.has(finishedNodeId)) return current;
    const next = new Set(current);
    next.delete(finishedNodeId);
    return next;
}

export function addRunningNodeId(current: Set<string>, nodeId: string) {
    if (current.has(nodeId)) return current;
    return new Set([...current, nodeId]);
}

export type RunningNodeSetter = (value: Set<string> | ((current: Set<string>) => Set<string>)) => void;

export async function runWithRunningNodeId<T>(nodeId: string, setRunningNodeId: RunningNodeSetter, task: () => Promise<T>) {
    setRunningNodeId((current) => addRunningNodeId(current, nodeId));
    try {
        return await task();
    } finally {
        setRunningNodeId((current) => clearFinishedRunningNodeId(current, nodeId));
    }
}
