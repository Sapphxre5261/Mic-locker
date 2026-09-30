(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);

  const el = {
    hwStatus: $("hw-status"),
    banner: $("browser-banner"),
    toast: $("toast"),
    ringFg: $("ring-fg"),
    volumeFigure: $("volume-figure"),
    targetFigure: $("target-figure"),
    peakFill: $("peak-fill"),
    muteLabel: $("mute-label"),
    restorations: $("restorations"),
    deviceSelect: $("device-select"),
    btnRefresh: $("btn-refresh"),
    btnHide: $("btn-hide"),
    slider: $("volume-slider"),
    sliderValue: $("slider-value"),
    lockToggle: $("lock-toggle"),
    lockToggleLabel: $("lock-toggle-label"),
    intervalSelect: $("interval-select"),
    optCloseTray: $("opt-close-tray"),
    optStartHidden: $("opt-start-hidden"),
    optStartup: $("opt-startup"),
    activeDevice: $("active-device"),
    connLabel: $("conn-label"),
    errorBox: $("error-box"),
    languageSelect: $("language-select"),
  };

  const i18n = window.MicLockerI18n;
  let currentLanguage = "th";
  const t = (key, params) => i18n.text(currentLanguage, key, params);

  function applyLanguage(lang) {
    const next = i18n.normalizeLanguage(lang);
    if (next === currentLanguage) {
      if (el.languageSelect && el.languageSelect.value !== next) {
        el.languageSelect.value = next;
      }
      return;
    }
    currentLanguage = next;
    i18n.apply(next);
    if (el.languageSelect) el.languageSelect.value = next;
  }

  const RING_LEN = 2 * Math.PI * 84;

  const backend = () =>
    typeof window !== "undefined" &&
    window.go && window.go.main && window.go.main.App
      ? window.go.main.App
      : null;

  let settings = null;
  let lastStatus = null;
  let connectedOnce = false;
  let backendLost = false;

  let saving = false;
  const patches = [];
  let revision = 0;

  let sliderInteracting = false;
  let sliderDebounce = null;
  let pendingTarget = null;
  let toastTimer = null;

  function showToast(msg) {
    el.toast.textContent = msg;
    el.toast.hidden = false;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      el.toast.hidden = true;
    }, 4000);
  }

  function desiredSettings() {
    const base = settings || {};
    return patches.reduce((s, p) => ({ ...s, ...p }), { ...base });
  }

  async function save(patch) {
    patches.push(patch);
    revision++;
    if (lastStatus) render(lastStatus);
    if (saving) return;
    saving = true;
    try {
      while (patches.length) {
        const head = patches[0];
        try {
          const status = await backend().SaveSettings({ ...settings, ...head });
          settings = status.settings;
          lastStatus = status;
        } catch (e) {
          showToast(String((e && e.message) || e));
        }
        patches.shift();
        revision++;
        if (lastStatus) render(lastStatus);
      }
    } finally {
      saving = false;
    }
  }

  function hasPending(field) {
    return patches.some((p) => field in p);
  }

  function setControlsEnabled(on) {
    const controls = [
      el.deviceSelect, el.btnRefresh, el.btnHide, el.slider,
      el.lockToggle, el.intervalSelect, el.optCloseTray,
      el.optStartHidden, el.optStartup, el.languageSelect,
      ...document.querySelectorAll(".btn-preset"),
    ];
    for (const c of controls) {
      if ("disabled" in c) c.disabled = !on;
    }
    document.body.classList.toggle("no-backend", !on);
  }

  function render(status) {
    if (!status) return;
    setControlsEnabled(true);
    lastStatus = status;
    settings = status.settings;
    const s = desiredSettings();
    applyLanguage(s.language);

    let chipText;
    let chipClass;
    if (status.error) {
      chipText = status.connected ? t("checkError") : t("noMicrophone");
      chipClass = "chip-dim";
    } else if (!status.connected) {
      chipText = t("noMicrophone");
      chipClass = "chip-dim";
    } else if (status.settings.locked) {
      chipText = t("locked");
      chipClass = "chip-locked";
    } else {
      chipText = t("unlocked");
      chipClass = "chip-unlocked";
    }
    el.hwStatus.textContent = chipText;
    el.hwStatus.className = "chip " + chipClass;
    document.body.classList.toggle(
      "app-active",
      status.connected && status.settings.locked && !status.error,
    );

    const pct = status.connected ? status.current : null;
    el.volumeFigure.textContent = pct === null ? "--" : String(pct);
    const frac = pct === null ? 0 : Math.min(100, Math.max(0, pct)) / 100;
    el.ringFg.style.strokeDashoffset = String(RING_LEN * (1 - frac));
    el.peakFill.style.width =
      Math.min(100, Math.max(0, status.peak * 100)).toFixed(1) + "%";
    el.muteLabel.textContent = status.connected
      ? status.muted ? t("muted") : t("unmuted")
      : "--";
    el.restorations.textContent = t(
      status.restorations === 1 ? "timesOne" : "timesMany",
      { count: status.restorations },
    );
    el.targetFigure.textContent = s.target + "%";
    el.activeDevice.textContent = status.activeDeviceName || "--";
    el.connLabel.textContent = status.connected
      ? t("connected")
      : t("disconnected");

    if (status.error) {
      el.errorBox.textContent = status.error;
      el.errorBox.hidden = false;
    } else {
      el.errorBox.hidden = true;
    }

    const locked = !!s.locked;
    el.lockToggle.setAttribute("aria-checked", locked ? "true" : "false");
    el.lockToggleLabel.textContent = locked ? t("lockOn") : t("lockOff");
    el.intervalSelect.value = String(s.intervalMs);
    el.optCloseTray.checked = !!s.closeToTray;
    el.optStartHidden.checked = !!s.startHidden;
    el.optStartHidden.disabled = !s.closeToTray;
    el.optStartup.checked = !!s.runOnStartup;

    const sliderBusy = sliderInteracting || pendingTarget !== null || hasPending("target");
    if (!sliderBusy) {
      el.slider.value = String(s.target);
      el.sliderValue.textContent = s.target + "%";
    }

    renderDevices(status, s);
  }

  function renderDevices(status, desired) {
    const sel = el.deviceSelect;
    const wanted = desired.deviceId || "";
    const devices = status.devices || [];

    const known = new Set(devices.map((d) => d.id));
    const needsExtra = wanted !== "" && !known.has(wanted);
    const signature =
      JSON.stringify(devices) + "|" + wanted + "|" + needsExtra + "|" + currentLanguage;
    if (sel.dataset.sig === signature) {
      sel.value = wanted;
      return;
    }
    sel.dataset.sig = signature;

    sel.textContent = "";
    const auto = document.createElement("option");
    auto.value = "";
    auto.textContent = t("autoDevice");
    sel.appendChild(auto);
    for (const d of devices) {
      const o = document.createElement("option");
      o.value = d.id;
      o.textContent = d.default ? d.name + " (" + t("defaultDevice") + ")" : d.name;
      sel.appendChild(o);
    }
    if (needsExtra) {
      const o = document.createElement("option");
      o.value = wanted;
      o.textContent = t("missingDevice");
      sel.appendChild(o);
    }
    sel.value = wanted;
  }

  function bindEvents() {
    el.deviceSelect.addEventListener("change", () => {
      save({ deviceId: el.deviceSelect.value });
    });

    el.btnRefresh.addEventListener("click", async () => {
      const app = backend();
      if (!app) return;
      const rev = ++revision;
      try {
        const status = await app.RefreshDevices();
        if (rev === revision && !saving && !patches.length) {
          render(status);
        }
      } catch (e) {
        showToast(String((e && e.message) || e));
      }
    });

    el.btnHide.addEventListener("click", async () => {
      const app = backend();
      if (!app) return;
      try {
        await app.HideToTray();
      } catch (e) {
        showToast(String((e && e.message) || e));
      }
    });

    const commitSlider = () => {
      const target = pendingTarget;
      pendingTarget = null;
      if (target === null) return;
      save({ target });
    };

    el.slider.addEventListener("input", () => {
      sliderInteracting = true;
      const v = parseInt(el.slider.value, 10);
      pendingTarget = v;
      el.sliderValue.textContent = v + "%";
      if (sliderDebounce) clearTimeout(sliderDebounce);
      sliderDebounce = setTimeout(() => {
        sliderInteracting = false;
        sliderDebounce = null;
        commitSlider();
      }, 350);
    });
    el.slider.addEventListener("change", () => {
      sliderInteracting = false;
      if (sliderDebounce) {
        clearTimeout(sliderDebounce);
        sliderDebounce = null;
      }
      commitSlider();
    });

    document.querySelectorAll(".btn-preset").forEach((b) => {
      b.addEventListener("click", () => {
        if (sliderDebounce) {
          clearTimeout(sliderDebounce);
          sliderDebounce = null;
        }
        sliderInteracting = false;
        pendingTarget = null;
        const v = parseInt(b.dataset.preset, 10);
        el.slider.value = String(v);
        el.sliderValue.textContent = v + "%";
        save({ target: v });
      });
    });

    el.lockToggle.addEventListener("click", () => {
      save({ locked: !desiredSettings().locked });
    });

    el.intervalSelect.addEventListener("change", () => {
      save({ intervalMs: parseInt(el.intervalSelect.value, 10) });
    });
    el.optCloseTray.addEventListener("change", () => {
      if (el.optCloseTray.checked) {
        save({ closeToTray: true });
      } else {
        save({ closeToTray: false, startHidden: false });
      }
    });
    el.optStartHidden.addEventListener("change", () => {
      save({ startHidden: el.optStartHidden.checked });
    });
    el.optStartup.addEventListener("change", () => {
      save({ runOnStartup: el.optStartup.checked });
    });
    el.languageSelect.addEventListener("change", () => {
      save({ language: el.languageSelect.value });
    });
    document.addEventListener("visibilitychange", () => {
      document.body.classList.toggle("page-hidden", document.hidden);
    });
  }

  let pollInFlight = false;
  async function poll() {
    const app = backend();
    if (!app || pollInFlight || saving || patches.length) return;
    const rev = revision;
    pollInFlight = true;
    try {
      const status = await app.GetStatus();
      connectedOnce = true;
      if (backendLost) {
        backendLost = false;
        showToast(t("reconnected"));
      }
      if (rev === revision && !saving && !patches.length) {
        render(status);
      }
    } catch (e) {
      if (!backendLost) {
        backendLost = true;
        showToast(t("connectionLost"));
      }
      if (rev === revision && lastStatus) {
        render({ ...lastStatus, connected: false, error: t("connectionError") });
      } else if (!connectedOnce) {
        el.hwStatus.textContent = t("noMicrophone");
        el.hwStatus.className = "chip chip-dim";
      }
    } finally {
      pollInFlight = false;
    }
  }

  function boot() {
    i18n.apply("th");
    setControlsEnabled(false);
    bindEvents();
    if (!backend()) {
      el.banner.hidden = false;
      return;
    }
    poll();
    setInterval(poll, 250);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
