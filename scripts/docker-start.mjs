import { spawn, spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

const apiPort = process.env.API_PORT || "8080";
const webPort = process.env.WEB_PORT || "3000";
const webHostname = process.env.WEB_HOSTNAME || "0.0.0.0";

function runMigration() {
    const result = spawnSync("/app/server", ["migrate"], { stdio: "inherit", env: process.env });
    if (result.error) {
        console.error("failed to run database migration", result.error);
        process.exit(1);
    }
    if (result.status !== 0) {
        process.exit(result.status || 1);
    }
}

function startChild(name, command, args, options = {}) {
    return spawn(command, args, { stdio: "inherit", ...options });
}

function exitCode(code, signal) {
    if (typeof code === "number") return code;
    return signal ? 1 : 0;
}

export function superviseChildren(children, { exit = process.exit, signalSource = process, logError = console.error } = {}) {
    let exiting = false;

    const stopOthers = (current) => {
        for (const item of children) {
            if (item.child === current) continue;
            item.child.kill("SIGTERM");
        }
    };

    for (const item of children) {
        item.child.on("exit", (code, signal) => {
            if (exiting) return;
            exiting = true;
            stopOthers(item.child);
            exit(exitCode(code, signal));
        });
        item.child.on("error", (error) => {
            logError(`${item.name} process error`, error);
            if (exiting) return;
            exiting = true;
            stopOthers(item.child);
            exit(1);
        });
    }

    for (const signal of ["SIGINT", "SIGTERM"]) {
        signalSource.on(signal, () => {
            if (exiting) return;
            exiting = true;
            for (const item of children) {
                item.child.kill("SIGTERM");
            }
            exit(signal === "SIGTERM" ? 143 : 130);
        });
    }
}

export function startRuntime() {
    runMigration();
    const api = startChild("api", "/app/server", [], { env: { ...process.env, PORT: apiPort } });
    const web = startChild("web", "node", ["node_modules/next/dist/bin/next", "start"], {
        cwd: "/app/web",
        env: { ...process.env, HOSTNAME: webHostname, PORT: webPort },
    });
    superviseChildren([
        { name: "api", child: api },
        { name: "web", child: web },
    ]);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
    startRuntime();
}
