function dstAdminApp(options) {
  return {
    appId: options.appId,
    authenticated: false,
    loginKey: "",
    authError: "",
    activeTab: "server",
    busy: {},
    toasts: [],
    settings: {
      server_name: "",
      server_password: "",
      game_mode: "survival",
      max_players: 6,
      pvp: false,
      pause_when_empty: true,
      enable_caves: true,
    },
    mods: [],
    status: { status: "unknown", services: [], logs: "" },
    players: [],
    diagnostics: {},
    manualId: "",
    adminKuid: "",
    search: {
      query: "",
      page: 1,
      pageSize: 18,
      sort: "timeupdated",
      hasMore: false,
      results: [],
    },
    searchStatus: "默认按更新时间优先展示，避免装到长期不维护的 MOD。",
    pollTimer: null,

    async init() {
      this.loginKey = localStorage.getItem("dstAdminKey") || "";
      if (!this.loginKey) return;
      await this.login(true);
    },

    async login(silent = false) {
      this.authError = "";
      if (!this.loginKey.trim()) {
        this.authError = "请输入管理密钥";
        return;
      }
      try {
        await this.withBusy("auth", async () => {
          localStorage.setItem("dstAdminKey", this.loginKey.trim());
          await this.api("/api/auth/verify");
          this.authenticated = true;
          await this.refreshAll(true);
          if (!this.search.results.length) this.loadPopular(true);
        });
      } catch (error) {
        localStorage.removeItem("dstAdminKey");
        this.authenticated = false;
        this.authError = "密钥验证失败，请检查后重试。";
        if (!silent) this.toast(error.message, "error");
      }
    },

    logout() {
      localStorage.removeItem("dstAdminKey");
      this.loginKey = "";
      this.authenticated = false;
      this.authError = "";
      this.stopPolling();
    },

    async api(path, options = {}) {
      const headers = { "content-type": "application/json" };
      const key = localStorage.getItem("dstAdminKey") || "";
      if (key) headers["X-Admin-Key"] = key;
      const response = await fetch(path, { headers, ...options });
      if (response.status === 401) {
        this.authenticated = false;
        throw new Error("管理密钥无效或已过期");
      }
      if (!response.ok) {
        const text = await response.text();
        throw new Error((text || response.statusText).trim());
      }
      return response.json();
    },

    async withBusy(key, fn) {
      this.busy[key] = true;
      try {
        return await fn();
      } finally {
        this.busy[key] = false;
      }
    },

    async refreshAll(silent = false) {
      await this.withBusy("refresh", async () => {
        await Promise.allSettled([
          this.loadState(),
          this.loadServerStatus(true),
          this.loadPlayers(true),
        ]);
      });
      if (!silent) this.toast("已刷新管理端数据", "success");
    },

    async loadState() {
      const data = await this.api("/api/state");
      this.mods = data.mods || [];
      this.settings = { ...this.settings, ...(data.settings || {}) };
    },

    async loadServerStatus(silent = false) {
      try {
        await this.withBusy("status", async () => {
          this.status = await this.api("/api/server/status");
        });
        if (!silent) this.toast(this.status.message || "服务器状态已刷新", "success");
      } catch (error) {
        this.status = { status: "error", message: error.message, services: [], logs: error.message };
        if (!silent) this.toast(error.message, "error");
      }
    },

    async loadPlayers(silent = false) {
      try {
        await this.withBusy("players", async () => {
          const data = await this.api("/api/players");
          this.players = data.players || [];
        });
        if (!silent) this.toast("玩家列表已刷新", "success");
      } catch (error) {
        if (!silent) this.toast(error.message, "error");
      }
    },

    async saveSettings() {
      try {
        await this.withBusy("settings", async () => {
          const data = await this.api("/api/settings", {
            method: "POST",
            body: JSON.stringify(this.settings),
          });
          this.settings = { ...this.settings, ...(data.settings || {}) };
        });
        this.toast("基础设置已保存到管理端草稿", "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async generateConfig() {
      try {
        await this.withBusy("generate", async () => {
          await this.saveSettings();
          const data = await this.api("/api/generate", { method: "POST", body: "{}" });
          this.toast(`已生成本地配置，启用 ${data.enabled_count || 0} 个 MOD`, "success");
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async saveToServer() {
      try {
        await this.withBusy("save", async () => {
          await this.api("/api/settings", { method: "POST", body: JSON.stringify(this.settings) });
          const data = await this.api("/api/save", { method: "POST", body: "{}" });
          await this.loadDiagnostics();
          this.toast(`已保存到服务器，写入 ${data.written ? data.written.length : 0} 个文件`, "success");
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async restartServer() {
      if (!confirm("确定要重启饥荒服务器吗？在线玩家会断开连接。")) return;
      try {
        await this.withBusy("restart", async () => {
          await this.api("/api/restart", { method: "POST", body: "{}" });
          this.toast("重启命令已发送，正在轮询启动状态", "success");
          await this.loadServerStatus(true);
          this.startPolling();
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    startPolling() {
      this.stopPolling();
      let left = 24;
      this.pollTimer = setInterval(async () => {
        left -= 1;
        await this.loadServerStatus(true);
        if (["running", "error", "stopped"].includes(this.status.status) || left <= 0) this.stopPolling();
      }, 5000);
    },

    stopPolling() {
      if (this.pollTimer) clearInterval(this.pollTimer);
      this.pollTimer = null;
    },

    async runSearch(resetPage = false) {
      if (!this.search.query.trim()) {
        this.toast("请输入 MOD 名称或 Workshop ID", "warning");
        return;
      }
      if (resetPage) this.search.page = 1;
      try {
        await this.withBusy("search", async () => {
          const params = new URLSearchParams({
            q: this.search.query.trim(),
            page: String(this.search.page),
            page_size: String(this.search.pageSize),
            sort: this.search.sort,
          });
          const data = await this.api(`/api/search?${params.toString()}`);
          this.search.results = data.results || [];
          this.search.page = data.page || this.search.page;
          this.search.hasMore = !!data.has_more;
          this.searchStatus = `找到 ${this.search.results.length} 个结果，排序：${this.sortLabel(this.search.sort)}`;
        });
      } catch (error) {
        this.searchStatus = "搜索失败";
        this.toast(error.message, "error");
      }
    },

    async loadPopular(silent = false) {
      try {
        await this.withBusy("popular", async () => {
          const data = await this.api("/api/popular?limit=50");
          this.search.results = data.results || [];
          this.search.page = 1;
          this.search.hasMore = false;
          this.searchStatus = `热门推荐已加载 ${this.search.results.length} 个，已按更新时间和热度去重。`;
        });
        if (!silent) this.toast("热门 MOD 已刷新", "success");
      } catch (error) {
        if (!silent) this.toast(error.message, "error");
      }
    },

    previousPage() {
      if (this.search.page <= 1) return;
      this.search.page -= 1;
      this.runSearch(false);
    },

    nextPage() {
      if (!this.search.hasMore) return;
      this.search.page += 1;
      this.runSearch(false);
    },

    async addManualMod() {
      const id = this.manualId.trim();
      if (!/^\d+$/.test(id)) {
        this.toast("请输入正确的 Workshop ID", "warning");
        return;
      }
      await this.addMod(id);
      this.manualId = "";
    },

    async addMod(id) {
      try {
        await this.withBusy(`add:${id}`, async () => {
          const data = await this.api("/api/mods", {
            method: "POST",
            body: JSON.stringify({ id }),
          });
          this.mods = data.mods || [];
        });
        this.toast(`已加入右侧已安装列表：${id}。保存并重启后生效`, "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async toggleMod(id, enabled) {
      try {
        const data = await this.api("/api/mods/toggle", {
          method: "POST",
          body: JSON.stringify({ id, enabled }),
        });
        this.mods = data.mods || [];
        this.toast(`${enabled ? "已启用" : "已停用"} MOD ${id}`, "success");
      } catch (error) {
        this.toast(error.message, "error");
        await this.loadState();
      }
    },

    async removeMod(id) {
      if (!confirm(`确定移除 MOD ${id} 吗？`)) return;
      try {
        await this.withBusy(`remove:${id}`, async () => {
          const data = await this.api("/api/mods/remove", {
            method: "POST",
            body: JSON.stringify({ id }),
          });
          this.mods = data.mods || [];
          delete this.diagnostics[id];
        });
        this.toast(`已移除 MOD ${id}`, "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async downloadMod(id) {
      try {
        await this.withBusy(`download:${id}`, async () => {
          const data = await this.api("/api/mods/download", {
            method: "POST",
            body: JSON.stringify({ id }),
          });
          if (data.diagnostic) this.diagnostics[id] = data.diagnostic;
          this.toast(data.message || `MOD ${id} 下载完成`, data.status === "ok" ? "success" : "warning");
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async loadDiagnostics() {
      try {
        await this.withBusy("diagnostics", async () => {
          const data = await this.api("/api/mods/diagnostics");
          const next = {};
          for (const item of data.diagnostics || []) next[item.id] = item;
          this.diagnostics = next;
        });
        this.toast("MOD 诊断已刷新", "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async setAdmin(kuid, enabled) {
      try {
        const path = enabled ? "/api/players/admin/add" : "/api/players/admin/remove";
        const data = await this.api(path, {
          method: "POST",
          body: JSON.stringify({ kuid }),
        });
        this.players = data.players || [];
        this.toast(data.message || "管理员列表已保存，重启后生效", "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async addAdminByKuid() {
      const kuid = this.adminKuid.trim();
      if (!/^KU_[A-Za-z0-9]+$/.test(kuid)) {
        this.toast("请输入正确的 Klei ID，例如 KU_AvFFQ7Ox", "warning");
        return;
      }
      await this.setAdmin(kuid, true);
      this.adminKuid = "";
    },

    enabledMods() {
      return this.mods.filter((item) => item.enabled);
    },

    isInstalled(id) {
      return this.mods.some((item) => item.id === id);
    },

    serverStatusLabel() {
      return {
        running: "服务器正常运行",
        starting: "服务器启动中",
        stopped: "服务器已停止",
        error: "服务器异常",
        unknown: "等待状态刷新",
      }[this.status.status] || "等待状态刷新";
    },

    tabClass(name) {
      return this.activeTab === name ? "tab-button active" : "tab-button";
    },

    statusBadgeClass(status) {
      const value = String(status || "unknown").toLowerCase();
      if (value.includes("running") || value === "healthy") return "status-badge ok";
      if (value.includes("start") || value === "unknown") return "status-badge warn";
      return "status-badge danger";
    },

    diagClass(value) {
      return value ? "diag-pill ok" : "diag-pill danger";
    },

    sortLabel(value) {
      return {
        timeupdated: "优先最新",
        trend: "近期趋势",
        totaluniquesubscribers: "订阅最多",
        toprated: "评分最高",
      }[value] || "优先最新";
    },

    formatNumber(value) {
      return Number(value || 0).toLocaleString("zh-CN");
    },

    formatDate(value) {
      if (!value) return "未知";
      return new Date(Number(value) * 1000).toLocaleDateString("zh-CN");
    },

    toast(message, type = "success") {
      const id = Date.now() + Math.random();
      this.toasts.push({ id, message, type });
      setTimeout(() => {
        this.toasts = this.toasts.filter((item) => item.id !== id);
      }, 4200);
    },
  };
}
