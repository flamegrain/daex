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
  Server,
  Archive,
  Download,
  Trash2,
  RotateCcw,
  Clock,
  Calendar,
  FolderArchive,
  Plus
} from 'lucide-vue-next'
import { theme, lang, applyTheme, setLang, t, type ThemeMode, type LangType } from '../composables/useSettings'

interface WebUIUserInfo {
  username: string
  role: string
  updatedAt: string
}

interface AutoBackupSettings {
  enabled: boolean
  frequency: 'daily' | 'weekly' | 'monthly'
  time: string
  dayOfWeek: number
  dayOfMonth: number
  keepCount: number
  updatedAt?: string
}

interface SystemBackupItem {
  name: string
  size: number
  modTime: string
  type: string
}

const userInfo = ref<WebUIUserInfo>({
  username: 'admin',
  role: 'WebUI 管理员',
  updatedAt: ''
})

const loadingUser = ref(false)
const savingUser = ref(false)
const userMessage = ref<{ type: 'success' | 'error'; text: string } | null>(null)

// User Form fields
const editUsername = ref('')
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')

// Auto Backup Settings State
const autoBackup = ref<AutoBackupSettings>({
  enabled: true,
  frequency: 'daily',
  time: '03:00',
  dayOfWeek: 1,
  dayOfMonth: 1,
  keepCount: 30
})
const loadingBackupSettings = ref(false)
const savingBackupSettings = ref(false)
const backupSettingsMsg = ref<{ type: 'success' | 'error'; text: string } | null>(null)

// System Backups Management State
const systemBackups = ref<SystemBackupItem[]>([])
const loadingBackups = ref(false)
const creatingBackup = ref(false)
const newBackupLabel = ref('')
const backupActionMsg = ref<{ type: 'success' | 'error'; text: string } | null>(null)

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

// Auto Backup Settings Methods
async function fetchBackupSettings() {
  loadingBackupSettings.value = true
  try {
    const res = await fetch('/api/settings/backup')
    if (res.ok) {
      autoBackup.value = await res.json()
    }
  } catch (e) {
    console.error('Fetch backup settings failed:', e)
  } finally {
    loadingBackupSettings.value = false
  }
}

async function handleSaveBackupSettings() {
  savingBackupSettings.value = true
  backupSettingsMsg.value = null
  try {
    const res = await fetch('/api/settings/backup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(autoBackup.value)
    })
    const data = await res.json()
    if (res.ok) {
      backupSettingsMsg.value = { type: 'success', text: data.message || '配置文件自动备份策略已保存' }
    } else {
      backupSettingsMsg.value = { type: 'error', text: data.error || '保存失败' }
    }
  } catch (e: any) {
    backupSettingsMsg.value = { type: 'error', text: '请求失败: ' + e.message }
  } finally {
    savingBackupSettings.value = false
  }
}

// System Backups Management Methods
async function fetchSystemBackups() {
  loadingBackups.value = true
  try {
    const res = await fetch('/api/backups')
    if (res.ok) {
      const data = await res.json()
      systemBackups.value = data.backups || []
    }
  } catch (e) {
    console.error('Fetch backups failed:', e)
  } finally {
    loadingBackups.value = false
  }
}

async function handleCreateSystemBackup() {
  creatingBackup.value = true
  backupActionMsg.value = null
  try {
    const res = await fetch('/api/backups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label: newBackupLabel.value.trim() || 'manual' })
    })
    const data = await res.json()
    if (res.ok) {
      backupActionMsg.value = { type: 'success', text: `系统备份创建成功：${data.file}` }
      newBackupLabel.value = ''
      await fetchSystemBackups()
    } else {
      backupActionMsg.value = { type: 'error', text: data.error || '备份创建失败' }
    }
  } catch (e: any) {
    backupActionMsg.value = { type: 'error', text: '创建备份失败: ' + e.message }
  } finally {
    creatingBackup.value = false
  }
}

async function handleRestoreBackup(name: string) {
  if (!confirm(`确认要将当前系统配置恢复为备份 [${name}] 吗？\n当前未保存的配置改动将被覆盖。`)) {
    return
  }
  try {
    const res = await fetch('/api/backups/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    })
    const data = await res.json()
    if (res.ok) {
      alert(`恢复成功！\n${data.message || ''}`)
      await fetchSystemBackups()
    } else {
      alert(`恢复失败: ${data.error || ''}\n${data.details || ''}`)
    }
  } catch (e: any) {
    alert('恢复网络请求失败: ' + e.message)
  }
}

async function handleDeleteBackup(name: string) {
  if (!confirm(`确认删除备份文件 [${name}] 吗？此操作无法撤销。`)) return
  try {
    const res = await fetch(`/api/backups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    if (res.ok) {
      await fetchSystemBackups()
    } else {
      const data = await res.json()
      alert('删除失败: ' + (data.error || ''))
    }
  } catch (e: any) {
    alert('删除请求失败: ' + e.message)
  }
}

function handleDownloadBackup(name: string) {
  window.open(`/api/backups/${encodeURIComponent(name)}/download`, '_blank')
}

function formatBytes(bytes: number) {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
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
  fetchBackupSettings()
  fetchSystemBackups()
})
</script>

<template>
  <div class="settings-view">
    <!-- Header -->
    <header class="settings-header">
      <div class="header-left">
        <h2>{{ t('settings.title') }}</h2>
        <p class="subtitle">管理界面偏好、语言模式、配置文件自动备份与系统快照恢复</p>
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
                <option value="zh-CN">简体中文</option>
                <option value="en-US">English</option>
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
                placeholder="验证原登录密码 (初始密码: admin123)"
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

      <!-- Section 3: Configuration Auto Backup Strategy -->
      <section class="settings-card">
        <div class="card-title">
          <Calendar class="card-icon" :size="18" />
          <h3>配置文件自动备份策略</h3>
        </div>

        <div class="auto-backup-form">
          <!-- Switch Item -->
          <div class="backup-switch-row">
            <div class="switch-info">
              <span class="switch-title">定时自动备份配置文件</span>
              <span class="switch-desc">系统将按设定的周期自动生成当前生效分流配置与核心参数快照</span>
            </div>
            <label class="toggle-switch">
              <input type="checkbox" v-model="autoBackup.enabled" />
              <span class="slider"></span>
            </label>
          </div>

          <div v-if="autoBackup.enabled" class="backup-controls-grid">
            <div class="setting-item">
              <label class="setting-label">备份执行周期</label>
              <select v-model="autoBackup.frequency" class="select-input">
                <option value="daily">每天</option>
                <option value="weekly">每周</option>
                <option value="monthly">每月</option>
              </select>
            </div>

            <!-- If weekly -->
            <div v-if="autoBackup.frequency === 'weekly'" class="setting-item">
              <label class="setting-label">指定每周几执行</label>
              <select v-model="autoBackup.dayOfWeek" class="select-input">
                <option :value="1">周一</option>
                <option :value="2">周二</option>
                <option :value="3">周三</option>
                <option :value="4">周四</option>
                <option :value="5">周五</option>
                <option :value="6">周六</option>
                <option :value="7">周日</option>
              </select>
            </div>

            <!-- If monthly -->
            <div v-if="autoBackup.frequency === 'monthly'" class="setting-item">
              <label class="setting-label">指定每月几号执行</label>
              <select v-model="autoBackup.dayOfMonth" class="select-input">
                <option v-for="d in 28" :key="d" :value="d">{{ d }} 号</option>
              </select>
            </div>

            <div class="setting-item">
              <label class="setting-label">固定执行时间</label>
              <input
                v-model="autoBackup.time"
                type="time"
                class="text-input"
              />
            </div>

            <div class="setting-item">
              <label class="setting-label">历史备份保留份数</label>
              <select v-model="autoBackup.keepCount" class="select-input">
                <option :value="7">保留最近 7 份</option>
                <option :value="15">保留最近 15 份</option>
                <option :value="30">保留最近 30 份</option>
                <option :value="60">保留最近 60 份</option>
              </select>
            </div>
          </div>

          <!-- Status Message -->
          <div
            v-if="backupSettingsMsg"
            class="feedback-msg"
            :class="backupSettingsMsg.type"
          >
            <CheckCircle2 v-if="backupSettingsMsg.type === 'success'" :size="15" />
            <AlertCircle v-else :size="15" />
            <span>{{ backupSettingsMsg.text }}</span>
          </div>

          <!-- Submit Button -->
          <div class="form-actions">
            <button
              class="btn-save"
              :disabled="savingBackupSettings"
              @click="handleSaveBackupSettings"
            >
              <Save :size="15" />
              <span>{{ savingBackupSettings ? '保存中...' : '保存自动备份策略' }}</span>
            </button>
          </div>
        </div>
      </section>

      <!-- Section 4: dae System Backups Management -->
      <section class="settings-card">
        <div class="card-title justify-between">
          <div class="title-left">
            <FolderArchive class="card-icon" :size="18" />
            <h3>dae 系统备份与恢复</h3>
          </div>
          <span class="count-badge">历史备份共 {{ systemBackups.length }} 份</span>
        </div>

        <!-- Quick Instant Backup Bar -->
        <div class="create-backup-bar">
          <input
            v-model="newBackupLabel"
            type="text"
            placeholder="输入备份备注 (例如: 升级前全量快照、调整分流备份)"
            class="text-input flex-1"
          />
          <button
            class="btn-action primary"
            :disabled="creatingBackup"
            @click="handleCreateSystemBackup"
          >
            <Plus :size="14" />
            <span>{{ creatingBackup ? '正在备份...' : '立即创建系统备份' }}</span>
          </button>
        </div>

        <div
          v-if="backupActionMsg"
          class="feedback-msg"
          :class="backupActionMsg.type"
        >
          <CheckCircle2 v-if="backupActionMsg.type === 'success'" :size="15" />
          <AlertCircle v-else :size="15" />
          <span>{{ backupActionMsg.text }}</span>
        </div>

        <!-- Backup List Table -->
        <div class="backup-table-wrapper">
          <div v-if="systemBackups.length === 0" class="empty-backups">
            <Archive class="empty-icon" :size="36" />
            <p>暂无系统备份文件，可点击上方「立即创建系统备份」快速生成。</p>
          </div>

          <table v-else class="backup-table">
            <thead>
              <tr>
                <th>备份文件名称</th>
                <th style="width: 110px">文件大小</th>
                <th style="width: 170px">备份生成时间</th>
                <th style="width: 170px; text-align: right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in systemBackups" :key="b.name">
                <td class="name-cell">
                  <div class="name-box">
                    <span class="file-name">{{ b.name }}</span>
                    <span v-if="b.type" class="type-pill">{{ b.type }}</span>
                  </div>
                </td>
                <td class="mono-text">{{ formatBytes(b.size) }}</td>
                <td class="time-cell">{{ formatDate(b.modTime) }}</td>
                <td class="actions-cell">
                  <button
                    class="btn-cell-action restore"
                    @click="handleRestoreBackup(b.name)"
                    title="从该快照恢复系统配置"
                  >
                    <RotateCcw :size="13" /> 恢复
                  </button>
                  <button
                    class="btn-cell-action download"
                    @click="handleDownloadBackup(b.name)"
                    title="下载备份到本地"
                  >
                    <Download :size="13" />
                  </button>
                  <button
                    class="btn-cell-action delete"
                    @click="handleDeleteBackup(b.name)"
                    title="删除该备份文件"
                  >
                    <Trash2 :size="13" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- Section 5: About & Gateway Info -->
      <section class="settings-card">
        <div class="card-title">
          <Info class="card-icon" :size="18" />
          <h3>关于与系统信息</h3>
        </div>

        <div class="about-grid">
          <div class="about-item">
            <span class="about-label">项目组件</span>
            <span class="about-value">dae 控制台可视化网关面板</span>
          </div>
          <div class="about-item">
            <span class="about-label">内核底层技术</span>
            <span class="about-value">Linux eBPF 透明网络路由核心</span>
          </div>
          <div class="about-item">
            <span class="about-label">系统架构</span>
            <span class="about-value">Go 静态单二进制 + 内嵌 Vue3 单页系统</span>
          </div>
          <div class="about-item">
            <span class="about-label">配置与语法规则</span>
            <span class="about-value">dae 规范语法与 0600 安全权限控制</span>
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

.card-title.justify-between {
  justify-content: space-between;
}

.title-left {
  display: flex;
  align-items: center;
  gap: 8px;
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

.count-badge {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  background: var(--bg-input, #1e293b);
  padding: 3px 8px;
  border-radius: 12px;
  border: 1px solid var(--border-color, #334155);
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
  transition: border-color 0.15s, box-shadow 0.15s;
  box-sizing: border-box;
}

.select-input:focus, .text-input:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

.notice-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(56, 189, 248, 0.08);
  border: 1px solid rgba(56, 189, 248, 0.2);
  border-radius: 6px;
  margin-bottom: 20px;
}

.notice-icon {
  color: #38bdf8;
  margin-top: 2px;
  flex-shrink: 0;
}

.notice-text {
  font-size: 13px;
  color: var(--text-main, #e2e8f0);
  line-height: 1.5;
}

.user-meta-bar {
  display: flex;
  gap: 16px;
  padding: 12px 16px;
  background: var(--bg-input, #1e293b);
  border-radius: 6px;
  border: 1px solid var(--border-color, #334155);
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.meta-badge {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meta-label {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  text-transform: uppercase;
}

.meta-value {
  font-size: 13px;
  color: var(--text-main, #f8fafc);
}

.meta-value.bold {
  font-weight: 600;
}

.meta-value.role {
  color: #38bdf8;
  font-weight: 500;
}

.user-form, .auto-backup-form {
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
  gap: 8px;
}

.form-field label {
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
}

.required {
  color: #ef4444;
}

.feedback-msg {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 13px;
}

.feedback-msg.success {
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #10b981;
}

.feedback-msg.error {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #ef4444;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

.btn-save {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 20px;
  background: #0284c7;
  border: none;
  border-radius: 6px;
  color: #ffffff;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-save:hover:not(:disabled) {
  background: #0369a1;
}

.btn-save:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Auto Backup Styles */
.backup-switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 8px;
}

.switch-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.switch-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.switch-desc {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background-color: var(--border-color, #334155);
  transition: .3s;
  border-radius: 24px;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: .3s;
  border-radius: 50%;
}

input:checked + .slider {
  background-color: #10b981;
}

input:checked + .slider:before {
  transform: translateX(20px);
}

.backup-controls-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  padding: 16px;
  background: rgba(2, 132, 199, 0.04);
  border: 1px dashed rgba(2, 132, 199, 0.25);
  border-radius: 8px;
}

/* System Backups Styles */
.create-backup-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.flex-1 {
  flex: 1;
}

.btn-action.primary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: #0284c7;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s;
}

.btn-action.primary:hover:not(:disabled) {
  background: #0369a1;
}

.backup-table-wrapper {
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-card, #0f172a);
}

.empty-backups {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 36px 20px;
  color: var(--text-muted, #94a3b8);
  gap: 10px;
}

.empty-icon {
  color: var(--text-muted, #64748b);
  opacity: 0.6;
}

.backup-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.backup-table th {
  background: var(--bg-input, #090d16);
  color: var(--text-muted, #94a3b8);
  padding: 10px 14px;
  font-weight: 500;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.backup-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #f8fafc);
}

.name-cell .name-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-name {
  font-family: monospace;
  font-size: 12px;
}

.type-pill {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.mono-text {
  font-family: monospace;
  color: var(--text-muted, #94a3b8);
  font-size: 12px;
}

.time-cell {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.actions-cell {
  text-align: right;
  white-space: nowrap;
}

.btn-cell-action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  border: 1px solid transparent;
  font-size: 11px;
  cursor: pointer;
  margin-left: 6px;
  transition: all 0.15s;
}

.btn-cell-action.restore {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.25);
}

.btn-cell-action.restore:hover {
  background: #10b981;
  color: #fff;
}

.btn-cell-action.download {
  background: var(--bg-input, #1e293b);
  color: var(--text-muted, #94a3b8);
  border-color: var(--border-color, #334155);
}

.btn-cell-action.download:hover {
  color: var(--text-main, #fff);
  border-color: #38bdf8;
}

.btn-cell-action.delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.2);
}

.btn-cell-action.delete:hover {
  background: #ef4444;
  color: #fff;
}

.about-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.about-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: var(--bg-input, #1e293b);
  border-radius: 6px;
  border: 1px solid var(--border-color, #334155);
}

.about-label {
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.about-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main, #e2e8f0);
}
</style>
