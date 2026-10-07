<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Settings,
  Moon,
  Sun,
  Globe,
  User,
  Key,
  Shield,
  CheckCircle2,
  AlertCircle,
  Save,
  Info,
  Server
} from 'lucide-vue-next'
import { theme, lang, applyTheme, setLang, t, type ThemeMode, type LangType } from '../composables/useSettings'

interface WebUIUserInfo {
  username: string
  role: string
  updatedAt: string
}

const userInfo = ref<WebUIUserInfo>({
  username: 'admin',
  role: 'WebUI 管理员',
  updatedAt: ''
})

const loadingUser = ref(false)
const savingUser = ref(false)
const userMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

// Form fields
const editUsername = ref('')
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')

async function fetchUserInfo() {
  loadingUser.value = true
  try {
    const res = await fetch('/api/settings/user')
    if (res.ok) {
      const data = await res.json()
      userInfo.value = data
      editUsername.value = data.username
    }
  } catch (e) {
    console.error('Failed to fetch user info:', e)
  } finally {
    loadingUser.value = false
  }
}

async function handleSaveUser() {
  userMessage.value = null

  if (!oldPassword.value) {
    userMessage.value = { type: 'error', text: '请输入原登录密码进行安全验证' }
    return
  }

  if (newPassword.value && newPassword.value !== confirmPassword.value) {
    userMessage.value = { type: 'error', text: '两次输入的新密码不一致' }
    return
  }

  if (newPassword.value && newPassword.value.length < 6) {
    userMessage.value = { type: 'error', text: '新密码长度至少需要 6 个字符' }
    return
  }

  savingUser.value = true
  try {
    const res = await fetch('/api/settings/user', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        oldPassword: oldPassword.value,
        newUsername: editUsername.value.trim() || undefined,
        newPassword: newPassword.value || undefined
      })
    })
    const data = await res.json()
    if (res.ok) {
      userMessage.value = { type: 'success', text: data.message || 'WebUI 登录用户信息更新成功' }
      oldPassword.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
      await fetchUserInfo()
    } else {
      userMessage.value = { type: 'error', text: data.error || '更新失败' }
    }
  } catch (e: any) {
    userMessage.value = { type: 'error', text: '网络请求失败: ' + e.message }
  } finally {
    savingUser.value = false
  }
}

function handleThemeChange(newTheme: ThemeMode) {
  applyTheme(newTheme)
}

function handleLangChange(newLang: LangType) {
  setLang(newLang)
}

function formatDate(iso: string) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => {
  fetchUserInfo()
})
</script>

<template>
  <div class="settings-view">
    <!-- Header -->
    <header class="settings-header">
      <div class="header-left">
        <h2>{{ t('settings.title') }}</h2>
        <p class="subtitle">{{ t('settings.subtitle') }}</p>
      </div>
    </header>

    <!-- Main Content Container -->
    <div class="settings-content">
      <!-- Section 1: Appearance & Language -->
      <section class="settings-card">
        <div class="card-title">
          <Globe class="card-icon" :size="18" />
          <h3>界面偏好与语言</h3>
        </div>

        <div class="form-grid">
          <!-- Theme Setting -->
          <div class="setting-item">
            <label class="setting-label">{{ t('settings.theme') }}</label>
            <div class="theme-options">
              <button
                class="theme-btn"
                :class="{ active: theme === 'dark' }"
                @click="handleThemeChange('dark')"
              >
                <Moon :size="16" />
                <span>{{ t('settings.theme.dark') }}</span>
              </button>
              <button
                class="theme-btn"
                :class="{ active: theme === 'light' }"
                @click="handleThemeChange('light')"
              >
                <Sun :size="16" />
                <span>{{ t('settings.theme.light') }}</span>
              </button>
            </div>
            <span class="setting-desc">支持白天浅色与黑夜深色模式实时平滑切换</span>
          </div>

          <!-- Language Setting -->
          <div class="setting-item">
            <label class="setting-label">{{ t('settings.lang') }}</label>
            <div class="lang-selector">
              <select
                :value="lang"
                @change="handleLangChange(($event.target as HTMLSelectElement).value as LangType)"
                class="select-input"
              >
                <option value="zh-CN">简体中文 (Simplified Chinese)</option>
                <option value="en-US">English (US)</option>
              </select>
            </div>
            <span class="setting-desc">切换整个管理面板的显示语言</span>
          </div>
        </div>
      </section>

      <!-- Section 2: WebUI User Account -->
      <section class="settings-card">
        <div class="card-title">
          <User class="card-icon" :size="18" />
          <h3>{{ t('settings.user.title') }}</h3>
        </div>

        <!-- Security Disclaimer Notice -->
        <div class="notice-box">
          <Shield class="notice-icon" :size="18" />
          <div class="notice-text">
            <strong>独立安全认证</strong>：{{ t('settings.user.notice') }}
          </div>
        </div>

        <!-- Current User Info Badges -->
        <div class="user-meta-bar">
          <div class="meta-badge">
            <span class="meta-label">当前账户</span>
            <span class="meta-value bold">{{ userInfo.username }}</span>
          </div>
          <div class="meta-badge">
            <span class="meta-label">角色权限</span>
            <span class="meta-value role">{{ t('settings.user.role.admin') }}</span>
          </div>
          <div class="meta-badge">
            <span class="meta-label">上次修改时间</span>
            <span class="meta-value">{{ formatDate(userInfo.updatedAt) }}</span>
          </div>
        </div>

        <!-- Edit Form -->
        <div class="user-form">
          <div class="form-row">
            <div class="form-field">
              <label>修改 WebUI 用户名</label>
              <input
                v-model="editUsername"
                type="text"
                placeholder="例如: admin"
                class="text-input"
              />
            </div>
            <div class="form-field">
              <label>{{ t('settings.user.old_pwd') }} <span class="required">*</span></label>
              <input
                v-model="oldPassword"
                type="password"
                placeholder="验证原登录密码 (初始: admin123)"
                class="text-input"
              />
            </div>
          </div>

          <div class="form-row">
            <div class="form-field">
              <label>{{ t('settings.user.new_pwd') }} (留空则不修改密码)</label>
              <input
                v-model="newPassword"
                type="password"
                placeholder="新登录密码 (至少 6 位)"
                class="text-input"
              />
            </div>
            <div class="form-field">
              <label>{{ t('settings.user.confirm_pwd') }}</label>
              <input
                v-model="confirmPassword"
                type="password"
                placeholder="再次输入新登录密码"
                class="text-input"
              />
            </div>
          </div>

          <!-- Status Message -->
          <div
            v-if="userMessage"
            class="feedback-msg"
            :class="userMessage.type"
          >
            <CheckCircle2 v-if="userMessage.type === 'success'" :size="15" />
            <AlertCircle v-else :size="15" />
            <span>{{ userMessage.text }}</span>
          </div>

          <!-- Submit Button -->
          <div class="form-actions">
            <button
              class="btn-save"
              :disabled="savingUser"
              @click="handleSaveUser"
            >
              <Save :size="15" />
              <span>{{ savingUser ? '保存中...' : t('settings.user.save') }}</span>
            </button>
          </div>
        </div>
      </section>

      <!-- Section 3: About & Gateway Info -->
      <section class="settings-card">
        <div class="card-title">
          <Info class="card-icon" :size="18" />
          <h3>{{ t('settings.about.title') }}</h3>
        </div>

        <div class="about-grid">
          <div class="about-item">
            <span class="about-label">项目组件</span>
            <span class="about-value">dae-webui (轻量可视化 Web 控制台)</span>
          </div>
          <div class="about-item">
            <span class="about-label">内核底层技术</span>
            <span class="about-value">Linux eBPF / TC Transparent Proxy</span>
          </div>
          <div class="about-item">
            <span class="about-label">生产部署架构</span>
            <span class="about-value">Go 静态单二进制 + 内嵌 Vue3 SPA</span>
          </div>
          <div class="about-item">
            <span class="about-label">配置与语法规则</span>
            <span class="about-value">dae 严格语法规范 (0600 安全权限控制)</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  overflow-y: auto;
}

.settings-header {
  padding: 18px 24px;
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.settings-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
}

.settings-content {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 960px;
}

.settings-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 20px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.card-title h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.card-icon {
  color: #38bdf8;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main, #cbd5e1);
}

.setting-desc {
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.theme-options {
  display: flex;
  gap: 10px;
}

.theme-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.theme-btn:hover {
  background: var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
}

.theme-btn.active {
  background: rgba(14, 165, 233, 0.15);
  border-color: #38bdf8;
  color: #38bdf8;
  box-shadow: 0 0 10px rgba(56, 189, 248, 0.2);
}

.select-input, .text-input {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.select-input:focus, .text-input:focus {
  border-color: #38bdf8;
}

.notice-box {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: rgba(14, 165, 233, 0.08);
  border: 1px solid rgba(14, 165, 233, 0.25);
  border-radius: 6px;
  margin-bottom: 16px;
}

.notice-icon {
  color: #38bdf8;
  flex-shrink: 0;
  margin-top: 1px;
}

.notice-text {
  font-size: 12px;
  color: #bae6fd;
  line-height: 1.5;
}

.notice-text strong {
  color: #38bdf8;
}

.user-meta-bar {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  padding: 12px 16px;
  background: var(--bg-input, #1e293b);
  border-radius: 8px;
  margin-bottom: 20px;
}

.meta-badge {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.meta-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.meta-value {
  font-size: 13px;
  color: var(--text-main, #f8fafc);
}

.meta-value.bold {
  font-weight: 600;
  color: #38bdf8;
}

.meta-value.role {
  color: #34d399;
}

.user-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
}

.required {
  color: #f87171;
}

.feedback-msg {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 12px;
}

.feedback-msg.success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.feedback-msg.error {
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #f87171;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
}

.btn-save {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  background: #2563eb;
  border: none;
  border-radius: 6px;
  color: #ffffff;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-save:hover:not(:disabled) {
  background: #1d4ed8;
}

.btn-save:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.about-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 14px;
}

.about-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--bg-input, #1e293b);
  border-radius: 6px;
}

.about-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.about-value {
  font-size: 12px;
  color: var(--text-main, #e2e8f0);
  font-weight: 500;
}
</style>
