(() => {
  "use strict";

  const messages = {
    en: {
      brandCaption: "PERSONAL AUDIO CONTROL",
      hide: "Hide to tray",
      hideTitle: "Hide the window and keep running in System tray",
      browserBanner: "Open Mic Locker to connect to your microphone",
      eyebrow: "AUDIO / CONTROL CENTER",
      heading: "Your voice.",
      headingTail: "Your control.",
      consoleTitle: "Microphone control",
      inputLevel: "INPUT LEVEL",
      chooseDevice: "Select microphone",
      refresh: "Refresh devices",
      currentLevel: "CURRENT LEVEL",
      currentCaption: "Microphone volume",
      target: "TARGET",
      signal: "Input signal",
      peak: "PEAK",
      audioState: "Audio state",
      restored: "Restored",
      targetVolume: "Target volume",
      presets: "Volume presets",
      quickSet: "QUICK SET",
      lockVolume: "Lock microphone level",
      preferencesTitle: "Behavior",
      preferences: "PREFERENCES",
      background: "Background mode",
      backgroundHelp: "Close the window to System tray",
      startHidden: "Start quietly",
      startHiddenHelp: "Hide the window when the app starts",
      startup: "Launch with Windows",
      startupHelp: "Run on startup",
      interval: "Check interval",
      interval100: "Every 100 ms",
      interval250: "Every 250 ms",
      interval500: "Every 500 ms",
      interval1000: "Every second",
      deviceTitle: "Connected device",
      privacy: "Processed on your device",
      privacyDetail: "No recording. No data leaves your PC.",
      lockHelp: "When locked, volume changes from other apps are restored.",
      agcHelp: "In-app automatic gain (AGC) may need to be disabled separately.",
      footerPrivacy: "LOCAL FIRST / NO RECORDING",
      footerAudio: "WINDOWS AUDIO",
      language: "Language",
      noMicrophone: "No microphone",
      checkError: "Check connection",
      locked: "Level locked",
      unlocked: "Unlocked",
      muted: "Muted",
      unmuted: "Unmuted",
      timesOne: "{count} time",
      timesMany: "{count} times",
      connected: "Connected",
      disconnected: "Disconnected",
      lockOn: "Lock enabled",
      lockOff: "Lock disabled",
      autoDevice: "Automatic (Windows default microphone)",
      defaultDevice: "default",
      missingDevice: "Saved device (disconnected)",
      reconnected: "Reconnected to the app",
      connectionLost: "Temporarily unable to communicate with the app",
      connectionError: "Unable to communicate with the app",
    },
    th: {
      brandCaption: "ควบคุมเสียงในแบบของคุณ",
      hide: "ซ่อนลง Tray",
      hideTitle: "ซ่อนหน้าต่างและทำงานต่อใน System tray",
      browserBanner: "เปิดผ่านแอป Mic Locker เพื่อเชื่อมต่อไมโครโฟน",
      eyebrow: "เสียง / ศูนย์ควบคุม",
      heading: "เสียงของคุณ",
      headingTail: "คุณควบคุมเอง",
      consoleTitle: "ควบคุมไมโครโฟน",
      inputLevel: "ระดับเสียงเข้า",
      chooseDevice: "เลือกไมโครโฟน",
      refresh: "รีเฟรชอุปกรณ์",
      currentLevel: "ระดับปัจจุบัน",
      currentCaption: "ระดับเสียงไมโครโฟน",
      target: "เป้าหมาย",
      signal: "สัญญาณเข้า",
      peak: "พีก",
      audioState: "สถานะเสียง",
      restored: "ปรับกลับแล้ว",
      targetVolume: "ระดับเสียงเป้าหมาย",
      presets: "ค่าระดับเสียงล่วงหน้า",
      quickSet: "ตั้งค่าด่วน",
      lockVolume: "ล็อคระดับเสียง",
      preferencesTitle: "การทำงาน",
      preferences: "ตั้งค่า",
      background: "ทำงานเบื้องหลัง",
      backgroundHelp: "ปิดหน้าต่างแล้วย่อลง System tray",
      startHidden: "เริ่มแบบเงียบ",
      startHiddenHelp: "ซ่อนหน้าต่างเมื่อเปิดโปรแกรม",
      startup: "เปิดพร้อม Windows",
      startupHelp: "เริ่มอัตโนมัติเมื่อเข้า Windows",
      interval: "ช่วงเวลาตรวจสอบ",
      interval100: "ทุก 100 มิลลิวินาที",
      interval250: "ทุก 250 มิลลิวินาที",
      interval500: "ทุก 500 มิลลิวินาที",
      interval1000: "ทุก 1 วินาที",
      deviceTitle: "อุปกรณ์ที่เชื่อมต่อ",
      privacy: "ประมวลผลบนเครื่องเท่านั้น",
      privacyDetail: "ไม่บันทึกเสียง ไม่ส่งข้อมูลออก",
      lockHelp: "เมื่อล็อค แอปจะปรับระดับกลับหากโปรแกรมอื่นเปลี่ยนค่า",
      agcHelp: "การปรับเสียงอัตโนมัติ (AGC) ภายในบางแอปต้องปิดแยก",
      footerPrivacy: "ทำงานบนเครื่อง / ไม่บันทึกเสียง",
      footerAudio: "ระบบเสียง WINDOWS",
      language: "ภาษา",
      noMicrophone: "ไม่มีไมโครโฟน",
      checkError: "ตรวจสอบข้อผิดพลาด",
      locked: "กำลังล็อค",
      unlocked: "ไม่ได้ล็อค",
      muted: "ปิดเสียงอยู่",
      unmuted: "เปิดเสียงอยู่",
      timesOne: "{count} ครั้ง",
      timesMany: "{count} ครั้ง",
      connected: "เชื่อมต่อแล้ว",
      disconnected: "ไม่ได้เชื่อมต่อ",
      lockOn: "เปิดการล็อค",
      lockOff: "ปิดการล็อค",
      autoDevice: "อัตโนมัติ (ไมโครโฟนเริ่มต้นของ Windows)",
      defaultDevice: "ค่าเริ่มต้น",
      missingDevice: "อุปกรณ์ที่บันทึกไว้ (ไม่ได้เชื่อมต่อ)",
      reconnected: "เชื่อมต่อแอปกลับมาแล้ว",
      connectionLost: "สื่อสารกับแอปไม่ได้ชั่วคราว",
      connectionError: "สื่อสารกับแอปไม่ได้",
    },
  };

  function normalizeLanguage(lang) {
    return lang === "en" ? "en" : "th";
  }

  function text(lang, key, params = {}) {
    const template = messages[normalizeLanguage(lang)][key] ?? messages.th[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? `{${name}}`));
  }

  function apply(lang) {
    const locale = normalizeLanguage(lang);
    document.documentElement.lang = locale;
    document.querySelectorAll("[data-i18n]").forEach((el) => {
      el.textContent = text(locale, el.dataset.i18n);
    });
    for (const [dataName, attr] of [["i18nTitle", "title"], ["i18nAria", "aria-label"]]) {
      const selector = dataName === "i18nTitle" ? "[data-i18n-title]" : "[data-i18n-aria]";
      document.querySelectorAll(selector).forEach((el) => el.setAttribute(attr, text(locale, el.dataset[dataName])));
    }
  }

  window.MicLockerI18n = { messages, normalizeLanguage, text, apply };
})();
