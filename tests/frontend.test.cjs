"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

const SRC = path.join(__dirname, "..", "frontend", "app.js");
const I18N_SRC = path.join(__dirname, "..", "frontend", "i18n.js");
const HTML_SRC = path.join(__dirname, "..", "frontend", "index.html");
const CSS_SRC = path.join(__dirname, "..", "frontend", "style.css");

function makeEl() {
  return {
    listeners: {},
    children: [],
    dataset: {},
    style: {},
    attrs: {},
    classList: { toggle() {} },
    textContent: "",
    hidden: false,
    value: "",
    checked: false,
    disabled: false,
    addEventListener(t, f) { this.listeners[t] = f; },
    setAttribute(k, v) { this.attrs[k] = v; },
    appendChild(c) { this.children.push(c); },
  };
}

function deferred() {
  let resolve, reject;
  const p = new Promise((res, rej) => { resolve = res; reject = rej; });
  p.resolve = resolve;
  p.reject = reject;
  return p;
}

function i18nStubElements() {
  const html = fs.readFileSync(HTML_SRC, "utf8");
  const groups = { "[data-i18n]": [], "[data-i18n-title]": [], "[data-i18n-aria]": [] };
  for (const m of html.matchAll(/<[^>]+>/g)) {
    const tag = m[0];
    const mi = tag.match(/\bdata-i18n="([^"]+)"/);
    const mt = tag.match(/\bdata-i18n-title="([^"]+)"/);
    const ma = tag.match(/\bdata-i18n-aria="([^"]+)"/);
    if (!mi && !mt && !ma) continue;
    const e = makeEl();
    if (mi) { e.dataset.i18n = mi[1]; groups["[data-i18n]"].push(e); }
    if (mt) { e.dataset.i18nTitle = mt[1]; groups["[data-i18n-title]"].push(e); }
    if (ma) { e.dataset.i18nAria = ma[1]; groups["[data-i18n-aria]"].push(e); }
  }
  return groups;
}

function createWorld() {
  const els = new Map();
  const presets = [30, 50, 75, 100].map((v) => {
    const e = makeEl();
    e.dataset = { preset: String(v) };
    return e;
  });
  const i18nEls = i18nStubElements();
  const el = (id) => {
    if (!els.has(id)) els.set(id, makeEl());
    return els.get(id);
  };
  const timeouts = [];
  const intervals = [];
  const backend = {
    statusQueue: [],
    saves: [],
    nextStatus: deferred(),
    GetStatus() {
      const d = this.statusQueue.shift() || deferred();
      this.lastStatusCall = d;
      return d;
    },
    SaveSettings(s) {
      this.saves.push(s);
      const d = deferred();
      d.payload = s;
      this.lastSave = d;
      return d;
    },
    RefreshDevices() { const d = deferred(); this.lastRefresh = d; return d; },
    HideToTray() { const d = deferred(); this.lastHide = d; return d; },
    ShowWindow() { return Promise.resolve(); },
    Quit() { return Promise.resolve(); },
  };
  const document = {
    readyState: "complete",
    documentElement: { lang: "" },
    hidden: false,
    body: makeEl(),
    getElementById: el,
    querySelectorAll(sel) {
      if (sel === ".btn-preset") return presets;
      return i18nEls[sel] || [];
    },
    createElement() { return makeEl(); },
    addEventListener() {},
  };
  const sandbox = {
    window: { go: { main: { App: backend } } },
    document,
    setTimeout(fn) { timeouts.push(fn); return timeouts.length; },
    clearTimeout() {},
    setInterval(fn) { intervals.push(fn); return intervals.length; },
    console,
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(I18N_SRC, "utf8"), sandbox);
  vm.runInContext(fs.readFileSync(SRC, "utf8"), sandbox);
  return { el, els, presets, timeouts, intervals, backend, sandbox, document, i18nEls };
}

const flush = async () => {
  for (let i = 0; i < 20; i++) await new Promise((r) => setImmediate(r));
};

const baseSettings = {
  deviceId: "", target: 75, locked: false, intervalMs: 250,
  closeToTray: true, startHidden: false, runOnStartup: false, language: "th",
};
const statusOf = (over = {}) => {
  const base = {
    settings: baseSettings,
    devices: [{ id: "m1", name: "Mic", default: true }],
    activeDeviceId: "m1", activeDeviceName: "Mic",
    connected: true, current: 75, muted: false, peak: 0,
    restorations: 0, error: "",
  };
  return { ...base, ...over, settings: { ...baseSettings, ...(over.settings || {}) } };
};

test("controls disabled until first successful status", async () => {
  const w = createWorld();
  await flush();
  assert.strictEqual(w.el("volume-slider").disabled, true);
  assert.strictEqual(w.el("lock-toggle").disabled, true);
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  assert.strictEqual(w.el("volume-slider").disabled, false);
  assert.strictEqual(w.el("lock-toggle").disabled, false);
});

test("rapid patch saves preserve earlier edits in later payloads", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();

  w.el("volume-slider").value = "60";
  w.el("volume-slider").listeners.input();
  w.el("volume-slider").listeners.change();
  assert.strictEqual(w.backend.saves.length, 1);
  assert.strictEqual(w.backend.saves[0].target, 60);

  w.el("opt-close-tray").checked = false;
  w.el("opt-close-tray").listeners.change();

  w.backend.lastSave.resolve(statusOf({ settings: { target: 60 } }));
  await flush();
  w.backend.lastSave.resolve(
    statusOf({ settings: { target: 60, closeToTray: false, startHidden: false } })
  );
  await flush();

  assert.strictEqual(w.backend.saves.length, 2);
  assert.strictEqual(w.backend.saves[1].target, 60);
  assert.strictEqual(w.backend.saves[1].closeToTray, false);
  assert.strictEqual(w.backend.saves[1].startHidden, false);
  assert.strictEqual(w.el("opt-start-hidden").disabled, true);
});

test("in-flight stale poll cannot clobber pending edit", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();

  w.intervals[0]();
  const stalePoll = w.backend.lastStatusCall;
  assert.strictEqual(w.el("hw-status").textContent, "ไม่ได้ล็อค");

  w.el("volume-slider").value = "60";
  w.el("volume-slider").listeners.input();
  w.el("volume-slider").listeners.change();

  stalePoll.resolve(statusOf({ settings: { target: 75 } }));
  await flush();
  assert.strictEqual(w.backend.saves.length, 1);
  assert.strictEqual(w.backend.saves[0].target, 60);
  assert.strictEqual(w.el("slider-value").textContent, "60%");

  w.backend.lastSave.resolve(statusOf({ settings: { target: 60 } }));
  await flush();
  assert.strictEqual(w.el("slider-value").textContent, "60%");
});

test("failed save does not poison the queue", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();

  w.el("interval-select").value = "500";
  w.el("interval-select").listeners.change();
  assert.strictEqual(w.backend.saves.length, 1);
  w.backend.lastSave.reject(new Error("disk full"));
  await flush();
  assert.strictEqual(w.el("toast").hidden, false);
  assert.match(w.el("toast").textContent, /disk full/);

  w.el("interval-select").value = "100";
  w.el("interval-select").listeners.change();
  await flush();
  assert.strictEqual(w.backend.saves.length, 2);
  assert.strictEqual(w.backend.saves[1].intervalMs, 100);
});

test("startHidden cannot stay on without closeToTray", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(
    statusOf({ settings: { startHidden: true } })
  );
  await flush();
  w.el("opt-close-tray").checked = false;
  w.el("opt-close-tray").listeners.change();
  assert.strictEqual(w.backend.saves.length, 1);
  assert.strictEqual(w.backend.saves[0].startHidden, false);
});

test("error status never renders active lock chip", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(
    statusOf({ settings: { locked: true }, error: "audio gone" })
  );
  await flush();
  assert.notStrictEqual(w.el("hw-status").textContent, "กำลังล็อค");
  assert.strictEqual(w.el("hw-status").textContent, "ตรวจสอบข้อผิดพลาด");
  assert.strictEqual(w.el("error-box").hidden, false);
});

test("browser mode shows connect banner and disables controls", async () => {
  const els = new Map();
  const el = (id) => {
    if (!els.has(id)) els.set(id, makeEl());
    return els.get(id);
  };
  const i18nEls = i18nStubElements();
  const document = {
    readyState: "complete",
    documentElement: { lang: "" },
    hidden: false,
    body: makeEl(),
    getElementById: el,
    querySelectorAll(sel) { return i18nEls[sel] || []; },
    createElement() { return makeEl(); },
    addEventListener() {},
  };
  const sandbox = {
    window: {},
    document,
    setTimeout: () => 0,
    clearTimeout() {},
    setInterval: () => 0,
    console,
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(I18N_SRC, "utf8"), sandbox);
  vm.runInContext(fs.readFileSync(SRC, "utf8"), sandbox);
  await flush();
  assert.strictEqual(el("browser-banner").hidden, false);
  assert.strictEqual(el("volume-slider").disabled, true);
});

test("index.html IDs are unique and all app.js lookups exist", () => {
  const html = fs.readFileSync(HTML_SRC, "utf8");
  const js = fs.readFileSync(SRC, "utf8");
  const ids = [...html.matchAll(/id="([^"]+)"/g)].map((m) => m[1]);
  assert.strictEqual(new Set(ids).size, ids.length, "duplicate id found");
  const used = [...js.matchAll(/\$\("([^"]+)"\)/g)].map((m) => m[1]);
  for (const id of used) {
    assert.ok(ids.includes(id), `app.js references missing #${id}`);
  }
});

test("ring gauge keeps r=84 contract for app.js math", () => {
  const html = fs.readFileSync(HTML_SRC, "utf8");
  assert.match(html, /<circle id="ring-fg"[^>]*r="84"/);
});

test("presets and control inputs unchanged", () => {
  const html = fs.readFileSync(HTML_SRC, "utf8");
  for (const v of ["30", "50", "75", "100"]) {
    assert.match(html, new RegExp(`class="btn btn-preset"[^>]*data-preset="${v}"`));
  }
  for (const id of [
    "device-select", "volume-slider", "lock-toggle", "interval-select",
    "opt-close-tray", "opt-start-hidden", "opt-startup", "language-select",
  ]) {
    assert.ok(html.includes(`id="${id}"`), `missing #${id}`);
  }
  for (const v of ["100", "250", "500", "1000"]) {
    assert.ok(html.includes(`value="${v}"`), `missing interval option ${v}`);
  }
});

test("stylesheet uses blue glass theme with safety fallbacks", () => {
  const css = fs.readFileSync(CSS_SRC, "utf8");
  assert.match(css, /backdrop-filter:\s*blur\(22px\)/i);
  assert.match(css, /@supports\s*\(backdrop-filter/i);
  assert.match(css, /@keyframes\s+aurora-drift/);
  assert.match(css, /--accent:\s*#60d5ff/i);
  assert.match(css, /--bg:\s*#050a16/i);
  assert.doesNotMatch(css, /#c3f45c|#0b0d0c|#141715/i);
  assert.match(css, /prefers-reduced-motion:\s*reduce/);
  assert.match(css, /prefers-reduced-transparency:\s*reduce/);
  assert.match(css, /\[hidden\]/);
});

test("i18n catalogs have identical nonempty keys", () => {
  const { messages } = requireVM();
  const enKeys = Object.keys(messages.en).sort();
  const thKeys = Object.keys(messages.th).sort();
  assert.deepStrictEqual(enKeys, thKeys);
  for (const k of enKeys) {
    assert.ok(messages.en[k].length > 0, `en.${k} empty`);
    assert.ok(messages.th[k].length > 0, `th.${k} empty`);
  }
});

function requireVM() {
  const document = {
    documentElement: { lang: "" },
    querySelectorAll() { return []; },
  };
  const sandbox = { window: {}, document };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(I18N_SRC, "utf8"), sandbox);
  return sandbox.window.MicLockerI18n;
}

test("every data-i18n key in HTML exists in both catalogs", () => {
  const { messages } = requireVM();
  const html = fs.readFileSync(HTML_SRC, "utf8");
  const keys = new Set();
  for (const m of html.matchAll(/data-i18n(?:-title|-aria)?="([^"]+)"/g)) keys.add(m[1]);
  assert.ok(keys.size > 30, "expected broad coverage");
  for (const k of keys) {
    assert.ok(k in messages.en, `missing en.${k}`);
    assert.ok(k in messages.th, `missing th.${k}`);
  }
});

test("saved English status renders English and sets document lang", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "en");
  const heading = w.i18nEls["[data-i18n]"].find((e) => e.dataset.i18n === "heading");
  assert.strictEqual(heading.textContent, "Your voice.");
  assert.strictEqual(w.el("hw-status").textContent, "Unlocked");
  const auto = w.el("device-select").children[0];
  assert.strictEqual(auto.textContent, "Automatic (Windows default microphone)");
  const def = w.el("device-select").children.find((o) => o.value === "m1");
  assert.strictEqual(def.textContent, "Mic (default)");
});

test("Thai saved status renders Thai", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "th");
  const heading = w.i18nEls["[data-i18n]"].find((e) => e.dataset.i18n === "heading");
  assert.strictEqual(heading.textContent, "เสียงของคุณ");
});

test("language switch applies English immediately before save resolves", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  w.el("language-select").value = "en";
  w.el("language-select").listeners.change();
  await flush();
  assert.strictEqual(w.backend.saves.length, 1);
  assert.deepStrictEqual(
    Object.keys(w.backend.saves[0]).filter((k) => w.backend.saves[0][k] !== baseSettings[k]),
    ["language"]
  );
  assert.strictEqual(w.backend.saves[0].language, "en");
  assert.strictEqual(w.document.documentElement.lang, "en");
  const heading = w.i18nEls["[data-i18n]"].find((e) => e.dataset.i18n === "heading");
  assert.strictEqual(heading.textContent, "Your voice.");
  w.backend.lastSave.resolve(statusOf({ settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "en");
});

test("failed language save rolls back DOM and toast works", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  w.el("language-select").value = "en";
  w.el("language-select").listeners.change();
  await flush();
  w.backend.lastSave.reject(new Error("unsupported"));
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "th");
  assert.strictEqual(w.el("language-select").value, "th");
  assert.strictEqual(w.el("toast").hidden, false);
  const heading = w.i18nEls["[data-i18n]"].find((e) => e.dataset.i18n === "heading");
  assert.strictEqual(heading.textContent, "เสียงของคุณ");
  w.el("interval-select").value = "500";
  w.el("interval-select").listeners.change();
  await flush();
  assert.strictEqual(w.backend.saves.length, 2);
  assert.strictEqual(w.backend.saves[1].intervalMs, 500);
});

test("language change during pending target preserves both", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  w.el("volume-slider").value = "60";
  w.el("volume-slider").listeners.input();
  w.el("volume-slider").listeners.change();
  w.el("language-select").value = "en";
  w.el("language-select").listeners.change();
  w.backend.lastSave.resolve(statusOf({ settings: { target: 60 } }));
  await flush();
  assert.strictEqual(w.backend.saves.length, 2);
  assert.strictEqual(w.backend.saves[1].target, 60);
  assert.strictEqual(w.backend.saves[1].language, "en");
  w.backend.lastSave.resolve(statusOf({ settings: { target: 60, language: "en" } }));
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "en");
});

test("stale poll cannot reset pending language change", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  w.el("language-select").value = "en";
  w.el("language-select").listeners.change();
  w.intervals[0] && w.intervals[0]();
  if (w.backend.lastStatusCall && w.backend.lastStatusCall.resolve) {
    w.backend.lastStatusCall.resolve(statusOf());
  }
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "en");
  w.backend.lastSave.resolve(statusOf({ settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.document.documentElement.lang, "en");
});

test("restoration counter singular and plural in both languages", async () => {
  let w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ restorations: 1 }));
  await flush();
  assert.strictEqual(w.el("restorations").textContent, "1 ครั้ง");
  w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ restorations: 12 }));
  await flush();
  assert.strictEqual(w.el("restorations").textContent, "12 ครั้ง");
  w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ restorations: 1, settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.el("restorations").textContent, "1 time");
  w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ restorations: 12, settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.el("restorations").textContent, "12 times");
});

test("dynamic lock, mute and disconnect labels in both languages", async () => {
  let w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ settings: { locked: true }, muted: true }));
  await flush();
  assert.strictEqual(w.el("hw-status").textContent, "กำลังล็อค");
  assert.strictEqual(w.el("mute-label").textContent, "ปิดเสียงอยู่");
  assert.strictEqual(w.el("lock-toggle-label").textContent, "เปิดการล็อค");
  assert.strictEqual(w.el("conn-label").textContent, "เชื่อมต่อแล้ว");
  w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf({ connected: false, settings: { language: "en" } }));
  await flush();
  assert.strictEqual(w.el("conn-label").textContent, "Disconnected");
  assert.strictEqual(w.el("hw-status").textContent, "No microphone");
  assert.strictEqual(w.el("lock-toggle-label").textContent, "Lock disabled");
  assert.strictEqual(w.el("mute-label").textContent, "--");
});

test("aria-label and title attributes update on language switch", async () => {
  const w = createWorld();
  w.backend.lastStatusCall.resolve(statusOf());
  await flush();
  w.el("language-select").value = "en";
  w.el("language-select").listeners.change();
  await flush();
  const aria = w.i18nEls["[data-i18n-aria]"].find((e) => e.dataset.i18nAria === "refresh");
  const title = w.i18nEls["[data-i18n-title]"].find((e) => e.dataset.i18nTitle === "hideTitle");
  assert.strictEqual(aria.attrs["aria-label"], "Refresh devices");
  assert.strictEqual(title.attrs["title"], "Hide the window and keep running in System tray");
  assert.ok(w.el("device-select").children.length > 0);
});

test("native device name is set via textContent without markup injection", async () => {
  const w = createWorld();
  const s = statusOf({ devices: [{ id: "m1", name: "My <Mic>", default: true }], activeDeviceName: "My <Mic>" });
  w.backend.lastStatusCall.resolve(s);
  await flush();
  const opt = w.el("device-select").children.find((o) => o.value === "m1");
  assert.strictEqual(opt.textContent, "My <Mic> (ค่าเริ่มต้น)");
  assert.strictEqual(w.el("active-device").textContent, "My <Mic>");
});

test("unknown language falls back to Thai", () => {
  const i18n = requireVM();
  assert.strictEqual(i18n.normalizeLanguage("de"), "th");
  assert.strictEqual(i18n.text("de", "locked"), "กำลังล็อค");
  assert.strictEqual(i18n.text("en", "no-such-key"), "no-such-key");
  assert.strictEqual(i18n.text("en", "timesMany", { count: 7 }), "7 times");
});
