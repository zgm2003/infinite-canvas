import { EventEmitter } from "node:events";
import assert from "node:assert/strict";
import test from "node:test";

import { superviseChildren } from "./docker-start.mjs";

class FakeChild extends EventEmitter {
    constructor(name) {
        super();
        this.name = name;
        this.killedWith = "";
    }

    kill(signal) {
        this.killedWith = signal;
    }
}

test("supervisor exits and kills web when api exits", () => {
    const api = new FakeChild("api");
    const web = new FakeChild("web");
    const exits = [];

    superviseChildren(
        [
            { name: "api", child: api },
            { name: "web", child: web },
        ],
        { exit: (code) => exits.push(code), signalSource: new EventEmitter(), logError: () => {} },
    );

    api.emit("exit", 7, null);

    assert.equal(web.killedWith, "SIGTERM");
    assert.deepEqual(exits, [7]);
});

test("supervisor exits and kills api when web exits", () => {
    const api = new FakeChild("api");
    const web = new FakeChild("web");
    const exits = [];

    superviseChildren(
        [
            { name: "api", child: api },
            { name: "web", child: web },
        ],
        { exit: (code) => exits.push(code), signalSource: new EventEmitter(), logError: () => {} },
    );

    web.emit("exit", 0, null);

    assert.equal(api.killedWith, "SIGTERM");
    assert.deepEqual(exits, [0]);
});

test("supervisor forwards termination signals to both children", () => {
    const api = new FakeChild("api");
    const web = new FakeChild("web");
    const signals = new EventEmitter();
    const exits = [];

    superviseChildren(
        [
            { name: "api", child: api },
            { name: "web", child: web },
        ],
        { exit: (code) => exits.push(code), signalSource: signals },
    );

    signals.emit("SIGTERM");

    assert.equal(api.killedWith, "SIGTERM");
    assert.equal(web.killedWith, "SIGTERM");
    assert.deepEqual(exits, [143]);
});

test("supervisor treats child spawn errors as fatal", () => {
    const api = new FakeChild("api");
    const web = new FakeChild("web");
    const exits = [];

    superviseChildren(
        [
            { name: "api", child: api },
            { name: "web", child: web },
        ],
        { exit: (code) => exits.push(code), signalSource: new EventEmitter(), logError: () => {} },
    );

    api.emit("error", new Error("spawn failed"));

    assert.equal(web.killedWith, "SIGTERM");
    assert.deepEqual(exits, [1]);
});
