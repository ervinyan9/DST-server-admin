function dstAdminApp(options) {
  return {
    appId: options.appId,
    authenticated: false,
    loginUsername: "",
    loginPassword: "",
    authError: "",
    activeTab: "dashboard",
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
    clusterToken: "",
    mods: [],
    status: { status: "unknown", services: [], logs: "", log_files: [], token: { present: false, file_present: false, last_status: "missing" } },
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
      const key = localStorage.getItem("dstAdminKey") || "";
      if (key) await this.verifySession(true);
    },

    async login(silent = false) {
      this.authError = "";
      const username = this.loginUsername.trim();
      if (!username || !this.loginPassword) {
        this.authError = "请输入用户名和密码";
        return;
      }
      try {
        await this.withBusy("auth", async () => {
          const data = await this.api("/api/auth/login", {
            method: "POST",
            body: JSON.stringify({ username, password: this.loginPassword }),
            skipAuth: true,
          });
          localStorage.setItem("dstAdminKey", data.admin_key || "");
          this.authenticated = true;
          await this.refreshAll(true);
          if (!this.search.results.length) this.loadPopular(true);
        });
      } catch (error) {
        localStorage.removeItem("dstAdminKey");
        this.authenticated = false;
        this.authError = "用户名或密码错误，请检查后重试。";
        if (!silent) this.toast(error.message, "error");
      }
    },

    async verifySession(silent = false) {
      this.authError = "";
      try {
        await this.withBusy("auth", async () => {
          await this.api("/api/auth/verify");
          this.authenticated = true;
          await this.refreshAll(true);
          if (!this.search.results.length) this.loadPopular(true);
        });
      } catch (error) {
        localStorage.removeItem("dstAdminKey");
        this.authenticated = false;
        if (!silent) {
          this.authError = "登录已失效，请重新登录。";
          this.toast(error.message, "error");
        }
      }
    },

    logout() {
      localStorage.removeItem("dstAdminKey");
      this.loginPassword = "";
      this.authenticated = false;
      this.authError = "";
      this.stopPolling();
    },

    async api(path, options = {}) {
      const { skipAuth = false, ...fetchOptions } = options;
      const headers = { "content-type": "application/json" };
      const key = skipAuth ? "" : (localStorage.getItem("dstAdminKey") || "");
      if (key) headers["X-Admin-Key"] = key;
      const response = await fetch(path, { headers, ...fetchOptions });
      if (response.status === 401) {
        this.authenticated = false;
        throw new Error("登录凭据无效或已过期");
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
        this.scrollLogsToBottom();
        if (!silent) this.toast(this.status.message || "服务器状态已刷新", "success");
      } catch (error) {
        this.status = { status: "error", message: error.message, services: [], logs: error.message, log_files: [] };
        this.scrollLogsToBottom();
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
          if (data.state && data.state.settings) {
            this.settings = { ...this.settings, ...data.state.settings };
          }
          const written = (data.applied && data.applied.written) || [];
          this.toast(`配置已保存到 /data，写入 ${written.length} 个文件`, "success");
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async saveClusterToken() {
      const token = this.clusterToken.trim();
      if (!token) {
        this.toast("请输入 Klei cluster token", "warning");
        return;
      }
      try {
        await this.withBusy("token", async () => {
          const data = await this.api("/api/cluster-token", {
            method: "POST",
            body: JSON.stringify({ token }),
          });
          if (data.token) this.status = { ...this.status, token: data.token };
        });
        this.clusterToken = "";
        this.toast("Klei cluster token 已写入，可点击重启启动 DST", "success");
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async restartServer() {
      if (!confirm("确定要重启饥荒服务器吗？在线玩家会断开连接。")) return;
      try {
        await this.withBusy("restart", async () => {
          await this.api("/api/restart", { method: "POST", body: "{}" });
          this.toast("已发出重启指令，正在轮询启动状态", "success");
          await this.loadServerStatus(true);
          this.startPolling();
        });
      } catch (error) {
        this.toast(error.message, "error");
      }
    },

    async syncEnabledMods() {
      if (!confirm("确定要同步已启用 MOD 并重载服务器吗？在线玩家会断开连接。")) return;
      try {
        await this.withBusy("modSync", async () => {
          const data = await this.api("/api/mods/sync", { method: "POST", body: "{}" });
          if (Array.isArray(data.diagnostics)) {
            const next = {};
            for (const item of data.diagnostics) next[item.id] = item;
            this.diagnostics = next;
          }
          this.toast(data.message || "MOD 已同步", data.status === "ok" ? "success" : "warning");
          await this.loadState();
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
        this.toast(`已加入 MOD ${id}，配置已写入 /data`, "success");
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
          await this.loadState();
          this.toast(data.message || `MOD ${id} 下载完成`, data.status === "ok" ? "success" : "warning");
        });
      } catch (error) {
        await this.loadState();
        await this.loadDiagnostics();
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
        this.toast("请输入正确的 Klei ID，例如 KU_xxxxxxxx", "warning");
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

    tokenStatusLabel() {
      const token = this.status.token || {};
      if (!token.present) return "未保存";
      if (token.last_status === "ok") return "校验正常";
      if (token.last_status === "error") return "校验失败";
      if (token.last_status === "missing") return "文件缺失";
      return "已保存";
    },

    tokenStatusMessage() {
      const token = this.status.token || {};
      if (token.last_message) return token.last_message;
      if (token.present) return "Token 已保存在管理端 SQLite，等待重启后校验。";
      return "尚未保存 Klei cluster token。";
    },

    tokenSavedAt() {
      const token = this.status.token || {};
      return token.saved_at ? new Date(token.saved_at).toLocaleString() : "-";
    },

    tokenBadgeClass() {
      const token = this.status.token || {};
      if (token.last_status === "ok") return this.statusBadgeClass("running");
      if (token.last_status === "error" || token.last_status === "missing") return this.statusBadgeClass("error");
      return this.statusBadgeClass("starting");
    },

    navClass(name) {
      // 侧栏导航按钮的 utility class 串。基础部分始终成立，再根据 active 切换深浅主题。
      const base =
        "flex items-center gap-2.5 min-h-8 rounded-md px-2.5 py-1.5 " +
        "text-[13px] font-semibold w-full text-left border-0 " +
        "cursor-pointer transition-colors flex-none " +
        "max-[1080px]:w-auto max-[1080px]:whitespace-nowrap";
      return this.activeTab === name
        ? base + " bg-panel text-ink"
        : base + " bg-transparent text-cream/75 hover:bg-cream/10 hover:text-cream";
    },

    navIconClass(name) {
      // 选中时图标用红色，未选中用琥珀色。
      const base = "text-[10px] w-3 text-center";
      return this.activeTab === name ? base + " text-red" : base + " text-amber";
    },

    currentTabEyebrow() {
      return {
        dashboard: "概览",
        settings: "配置",
        mods: "MOD",
        players: "权限",
      }[this.activeTab] || "管理端";
    },

    currentTabTitle() {
      return {
        dashboard: "控制台",
        settings: "服务器设置",
        mods: "MOD 管理",
        players: "玩家权限",
      }[this.activeTab] || "饥荒联机版管理端";
    },

    statusBadgeClass(status) {
      // 顶部 / 服务卡片右侧的圆角徽章，根据状态字符串返回不同色调。
      const base =
        "inline-flex items-center min-h-6 rounded-full border " +
        "px-[9px] py-[3px] text-xs font-extrabold whitespace-nowrap";
      const value = String(status || "unknown").toLowerCase();
      if (value.includes("running") || value === "healthy") {
        return base + " bg-green/[0.13] border-green/[0.44] text-[#1f5d35]";
      }
      if (value.includes("start") || value === "unknown") {
        return base + " bg-amber/[0.17] border-amber/[0.45] text-[#704313]";
      }
      return base + " bg-red/[0.13] border-red/[0.42] text-red-dark";
    },

    diagClass(value) {
      // MOD 诊断里的"配置/下载/启用/日志加载"小药丸，绿表正常红表缺失。
      const base =
        "inline-flex items-center min-h-6 rounded-full border " +
        "px-[9px] py-[3px] text-xs font-extrabold whitespace-nowrap";
      return value
        ? base + " bg-green/[0.13] border-green/[0.44] text-[#1f5d35]"
        : base + " bg-red/[0.13] border-red/[0.42] text-red-dark";
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

    formatBytes(value) {
      const size = Number(value || 0);
      if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`;
      if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`;
      return `${size} B`;
    },

    scrollLogsToBottom() {
      setTimeout(() => {
        document.querySelectorAll("[data-log-box]").forEach((node) => {
          node.scrollTop = node.scrollHeight;
        });
      }, 0);
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
