import { defineStore } from "pinia";
import { login as apiLogin, logout as apiLogout, sessionInfo, setCSRFToken } from "@/api/client";

let restoreTask: Promise<void> | undefined;

export const useAuthStore = defineStore("auth", {
  state: () => ({
    loaded: false,
    username: "",
  }),
  actions: {
    async restore() {
      if (this.loaded) return;
      if (!restoreTask) {
        restoreTask = (async () => {
          try {
            const result = await sessionInfo();
            if (result.authenticated && result.username && result.csrf) {
              this.username = result.username;
              setCSRFToken(result.csrf);
            } else {
              this.username = "";
            }
          } catch {
            this.username = "";
          } finally {
            this.loaded = true;
          }
        })();
      }
      await restoreTask;
    },
    async login(username: string, password: string) {
      const result = await apiLogin(username, password);
      setCSRFToken(result.csrf);
      this.username = result.username;
      this.loaded = true;
    },
    async logout() {
      try {
        await apiLogout();
      } finally {
        this.clearSession();
      }
    },
    clearSession() {
      this.username = "";
      this.loaded = true;
      setCSRFToken("");
    },
  },
});
