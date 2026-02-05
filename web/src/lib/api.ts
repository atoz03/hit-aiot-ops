export type ApiError = {
  message: string;
  status?: number;
  body?: string;
};

export type AuthMeResp = {
  authenticated: boolean;
  username?: string;
  role?: string;
  expires_at?: string;
  csrf_token?: string;
};

export type BalanceResp = {
  username: string;
  balance: number;
  status: "normal" | "warning" | "limited" | "blocked" | string;
};

export type UsageRecord = {
  node_id: string;
  username: string;
  timestamp: string;
  cpu_percent: number;
  memory_mb: number;
  gpu_usage: string;
  cost: number;
};

export type NodeStatus = {
  node_id: string;
  last_seen_at: string;
  last_report_id: string;
  last_report_ts: string;
  interval_seconds: number;
  gpu_process_count: number;
  cpu_process_count: number;
  usage_records_count: number;
  cost_total: number;
  updated_at: string;
};

export type PriceRow = { Model?: string; Price?: number; model?: string; price?: number };

function trimSlashRight(v: string): string {
  return v.replace(/\/+$/, "");
}

export class ApiClient {
  constructor(
    private readonly baseUrl: string,
    private readonly adminToken: string,
  ) {}

  private url(path: string): string {
    const base = this.baseUrl?.trim() ? trimSlashRight(this.baseUrl.trim()) : window.location.origin;
    return base + path;
  }

  private adminHeaders(): Record<string, string> {
    if (!this.adminToken?.trim()) return {};
    return { Authorization: `Bearer ${this.adminToken.trim()}` };
  }

  private async readText(res: Response): Promise<string> {
    try {
      return await res.text();
    } catch {
      return "";
    }
  }

  private async getJson<T>(path: string, headers: Record<string, string> = {}): Promise<T> {
    const res = await fetch(this.url(path), { headers, credentials: "include" });
    if (!res.ok) {
      const text = await this.readText(res);
      throw { message: `请求失败：${res.status}`, status: res.status, body: text } satisfies ApiError;
    }
    return (await res.json()) as T;
  }

  private async postJson<T>(path: string, body: unknown, headers: Record<string, string> = {}): Promise<T> {
    const res = await fetch(this.url(path), {
      method: "POST",
      headers: { "Content-Type": "application/json", ...headers },
      body: JSON.stringify(body),
      credentials: "include",
    });
    if (!res.ok) {
      const text = await this.readText(res);
      throw { message: `请求失败：${res.status}`, status: res.status, body: text } satisfies ApiError;
    }
    return (await res.json()) as T;
  }

  async healthz(): Promise<{ ok: boolean }> {
    return await this.getJson("/healthz");
  }

  async metricsText(): Promise<string> {
    const res = await fetch(this.url("/metrics"), { credentials: "include" });
    if (!res.ok) {
      const text = await this.readText(res);
      throw { message: `请求失败：${res.status}`, status: res.status, body: text } satisfies ApiError;
    }
    return await res.text();
  }

  async authMe(): Promise<AuthMeResp> {
    return await this.getJson("/api/auth/me");
  }

  async authLogin(username: string, password: string): Promise<{ ok: boolean }> {
    return await this.postJson("/api/auth/login", { username, password });
  }

  async authLogout(): Promise<{ ok: boolean }> {
    return await this.postJson("/api/auth/logout", {});
  }

  async userBalance(username: string): Promise<BalanceResp> {
    return await this.getJson(`/api/users/${encodeURIComponent(username)}/balance`);
  }

  async userUsage(username: string, limit: number): Promise<{ records: UsageRecord[] }> {
    return await this.getJson(`/api/users/${encodeURIComponent(username)}/usage?limit=${limit}`);
  }

  async adminUsers(): Promise<{ users: Array<{ Username?: string; Balance?: number; Status?: string; username?: string; balance?: number; status?: string }> }> {
    return await this.getJson("/api/admin/users", this.adminHeaders());
  }

  async adminPrices(): Promise<{ prices: Array<{ Model?: string; Price?: number; model?: string; price?: number }> }> {
    return await this.getJson("/api/admin/prices", this.adminHeaders());
  }

  async adminSetPrice(model: string, pricePerMinute: number): Promise<{ ok: boolean }> {
    return await this.postJson(
      "/api/admin/prices",
      { gpu_model: model, price_per_minute: pricePerMinute },
      this.adminHeaders(),
    );
  }

  async adminRecharge(username: string, amount: number, method: string): Promise<BalanceResp> {
    return await this.postJson(
      `/api/users/${encodeURIComponent(username)}/recharge`,
      { amount, method },
      this.adminHeaders(),
    );
  }

  async adminUsage(username: string, limit: number): Promise<{ records: UsageRecord[] }> {
    const q = new URLSearchParams();
    if (username.trim()) q.set("username", username.trim());
    q.set("limit", String(limit));
    return await this.getJson(`/api/admin/usage?${q.toString()}`, this.adminHeaders());
  }

  async adminNodes(limit: number): Promise<{ nodes: NodeStatus[] }> {
    return await this.getJson(`/api/admin/nodes?limit=${limit}`, this.adminHeaders());
  }

  async adminQueue(): Promise<{ queue: Array<{ username: string; gpu_type: string; count: number; timestamp: string }> }> {
    return await this.getJson("/api/admin/gpu/queue", this.adminHeaders());
  }

  async adminExportUsageCSV(params: { username?: string; from?: string; to?: string; limit?: number }): Promise<Blob> {
    const q = new URLSearchParams();
    if (params.username?.trim()) q.set("username", params.username.trim());
    if (params.from?.trim()) q.set("from", params.from.trim());
    if (params.to?.trim()) q.set("to", params.to.trim());
    q.set("limit", String(params.limit ?? 20000));
    const res = await fetch(this.url(`/api/admin/usage/export.csv?${q.toString()}`), {
      headers: this.adminHeaders(),
    });
    if (!res.ok) {
      const text = await this.readText(res);
      throw { message: `请求失败：${res.status}`, status: res.status, body: text } satisfies ApiError;
    }
    return await res.blob();
  }
}
