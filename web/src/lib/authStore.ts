import { reactive } from "vue";
import { ApiClient } from "./api";
import { settingsState } from "./settingsStore";

export type AuthState = {
  checked: boolean;
  authenticated: boolean;
  username: string;
  role: string;
  csrfToken: string;
  expiresAt: string;
};

export const authState = reactive<AuthState>({
  checked: false,
  authenticated: false,
  username: "",
  role: "",
  csrfToken: "",
  expiresAt: "",
});

export async function refreshAuth(): Promise<void> {
  const client = new ApiClient(settingsState.baseUrl);
  const me = await client.authMe();
  authState.checked = true;
  authState.authenticated = !!me.authenticated;
  authState.username = me.username ?? "";
  authState.role = me.role ?? "";
  authState.csrfToken = me.csrf_token ?? "";
  authState.expiresAt = me.expires_at ?? "";
}

export async function login(username: string, password: string): Promise<void> {
  const client = new ApiClient(settingsState.baseUrl);
  await client.authLogin(username, password);
  await refreshAuth();
}

export async function logout(): Promise<void> {
  const client = new ApiClient(settingsState.baseUrl);
  await client.authLogout();
  await refreshAuth();
}
