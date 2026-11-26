<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/store/auth";
import { errorMessage } from "@/api/client";

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();
const username = ref("");
const password = ref("");
const busy = ref(false);
const message = ref(route.query.expired ? "登录已过期，请重新登录" : "");

async function submit() {
  if (!username.value || !password.value) {
    message.value = "请输入账号和密码";
    return;
  }
  busy.value = true;
  message.value = "";
  try {
    await auth.login(username.value.trim(), password.value);
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/users";
    await router.replace(redirect);
  } catch (error) {
    message.value = errorMessage(error);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-glow login-glow-left" />
    <div class="login-glow login-glow-right" />

    <main class="login-card">
      <header class="mb-8">
        <div class="mb-7 flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div class="grid h-10 w-10 place-items-center rounded-xl bg-blue-600 text-lg font-black text-white shadow-lg shadow-blue-600/20">S</div>
            <div>
              <div class="text-sm font-bold tracking-wide text-slate-900">SVN PANEL</div>
              <div class="text-[11px] text-slate-400">Access Control</div>
            </div>
          </div>
          <span class="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[11px] font-semibold text-emerald-700">svnserve · local</span>
        </div>
        <h1 class="text-[28px] font-bold tracking-tight text-slate-900">管理员登录</h1>
        <p class="mt-2 text-sm leading-6 text-slate-500">使用服务器配置中的管理员凭据进入控制台。</p>
      </header>

      <form class="space-y-5" @submit.prevent="submit">
        <label class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">账号</span>
          <input
            v-model="username"
            class="login-input"
            placeholder="请输入管理员账号"
            autocomplete="username"
            autofocus
          />
        </label>
        <label class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">密码</span>
          <input
            v-model="password"
            class="login-input"
            type="password"
            placeholder="请输入管理员密码"
            autocomplete="current-password"
          />
        </label>

        <div v-if="message" class="rounded-lg border border-rose-200 bg-rose-50 px-3.5 py-2.5 text-sm text-rose-700">
          {{ message }}
        </div>

        <button class="login-submit" :disabled="busy">
          <span v-if="busy" class="login-spinner" />
          {{ busy ? "正在验证" : "进入管理面板" }}
        </button>
      </form>

      <footer class="mt-8 flex items-center justify-between border-t border-slate-100 pt-5 text-[11px] text-slate-400">
        <span>仅限授权管理员访问</span>
        <span>SVN Access Control</span>
      </footer>
    </main>
  </div>
</template>

<style scoped>
.login-page {
  position: relative;
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 2rem;
  background:
    radial-gradient(circle at 18% 12%, rgb(219 234 254 / 75%), transparent 28rem),
    radial-gradient(circle at 88% 86%, rgb(207 250 254 / 60%), transparent 30rem),
    linear-gradient(145deg, #f8fafc 0%, #f3f7fd 48%, #eef6ff 100%);
}

.login-page::before {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgb(148 163 184 / 8%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(148 163 184 / 8%) 1px, transparent 1px);
  background-size: 32px 32px;
  content: "";
  mask-image: linear-gradient(to bottom, black, transparent 75%);
}

.login-glow {
  position: absolute;
  border-radius: 999px;
  filter: blur(2px);
  pointer-events: none;
}

.login-glow-left {
  top: 18%;
  left: 12%;
  width: 9rem;
  height: 9rem;
  background: rgb(59 130 246 / 8%);
}

.login-glow-right {
  right: 14%;
  bottom: 16%;
  width: 12rem;
  height: 12rem;
  background: rgb(6 182 212 / 7%);
}

.login-card {
  position: relative;
  z-index: 1;
  width: min(28rem, 100%);
  border: 1px solid rgb(226 232 240 / 90%);
  border-radius: 20px;
  background: rgb(255 255 255 / 96%);
  padding: 2rem;
  box-shadow:
    0 24px 70px rgb(15 23 42 / 10%),
    0 2px 8px rgb(15 23 42 / 4%);
}

.login-input {
  width: 100%;
  height: 44px;
  border: 1px solid #dbe3ee;
  border-radius: 9px;
  background: #fff;
  padding: 0 13px;
  color: #172033;
  outline: none;
  transition: border-color 140ms, box-shadow 140ms;
}

.login-input::placeholder {
  color: #a3adba;
}

.login-input:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 4px rgb(59 130 246 / 10%);
}

.login-submit {
  display: flex;
  width: 100%;
  height: 44px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 0;
  border-radius: 9px;
  background: #2563eb;
  color: #fff;
  font-size: 14px;
  font-weight: 650;
  box-shadow: 0 8px 22px rgb(37 99 235 / 22%);
  transition: background-color 140ms, transform 100ms, box-shadow 140ms;
}

.login-submit:hover:not(:disabled) {
  background: #1d4ed8;
  box-shadow: 0 10px 26px rgb(37 99 235 / 28%);
}

.login-submit:active:not(:disabled) {
  transform: translateY(1px);
}

.login-submit:disabled {
  cursor: wait;
  opacity: 0.72;
}

.login-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgb(255 255 255 / 45%);
  border-top-color: #fff;
  border-radius: 50%;
  animation: login-spin 700ms linear infinite;
}

@keyframes login-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .login-page {
    padding: 1rem;
  }

  .login-card {
    padding: 1.5rem;
  }
}
</style>
