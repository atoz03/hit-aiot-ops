import { reactive } from "vue";
import { loadSettings, saveSettings, type Settings } from "./settings";

// 全局设置（持久化到 localStorage）
export const settingsState = reactive<Settings>(loadSettings());

export function persistSettings(): void {
  saveSettings({ ...settingsState });
}

export function setDefaultUsername(username: string): void {
  settingsState.defaultUsername = username;
  persistSettings();
}

