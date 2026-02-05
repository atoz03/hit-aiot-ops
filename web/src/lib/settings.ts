export type Settings = {
  baseUrl: string;
  defaultUsername: string;
};

const KEY = "gpuops.settings.v1";

const defaults: Settings = {
  baseUrl: "",
  defaultUsername: "",
};

export function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return { ...defaults };
    const parsed = JSON.parse(raw) as Partial<Settings>;
    return {
      baseUrl: (parsed.baseUrl ?? defaults.baseUrl).toString(),
      defaultUsername: (parsed.defaultUsername ?? defaults.defaultUsername).toString(),
    };
  } catch {
    return { ...defaults };
  }
}

export function saveSettings(next: Settings): void {
  localStorage.setItem(KEY, JSON.stringify(next));
}
