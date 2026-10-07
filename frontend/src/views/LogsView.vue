<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
import {
  Terminal,
  RefreshCw,
  Download,
  Archive,
  Search,
  Filter,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  ShieldAlert,
  Info,
  Bug,
  Play,
  Pause,
  ArrowDownCircle,
  Copy,
  Clock,
  HardDrive,
  FileText
} from 'lucide-vue-next'
import { t } from '../composables/useSettings'

interface LogEntry {
  timestamp: string
  level: 'INFO' | 'WARN' | 'ERROR' | 'DEBUG' | string
  tag: string
  message: string
  raw: string
}

interface LogBackupItem {
  name: string
  size: number
  modTime: string
}

const logs = ref<LogEntry[]>([])
const loading = ref(false)
const autoRefresh = ref(true)
const autoScroll = ref(true)
const logSource = ref('dae runtime')
const lastUpdateTime = ref('-')
const consoleRef = ref<HTMLDivElement | null>(null)

// Filters
const searchQuery = ref('')
const selectedLevel = ref('ALL')
const selectedTag = ref('all')

// Backups Modal
const showBackupModal = ref(false)
const backupLabel = ref('')
const creatingBackup = ref(false)
const backupsList = ref<LogBackupItem[]>([])

let timer: any = null

const levelCounts = computed(() => {
  const counts = { ALL: logs.value.length, ERROR: 0, WARN: 0, INFO: 0, DEBUG: 0 }
  for (const e of logs.value) {
    if (e.level === 'ERROR') counts.ERROR++
    else if (e.level === 'WARN') counts.WARN++
    else if (e.level === 'INFO') counts.INFO++
    else if (e.level === 'DEBUG') counts.DEBUG++
  }
  return counts
})

const filteredLogs = computed(() => {
  let list = logs.value
  if (selectedLevel.value !== 'ALL') {
    list = list.filter(e => e.level === selectedLevel.value)
  }
  if (selectedTag.value !== 'all') {
    list = list.filter(e => e.tag === selectedTag.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    list = list.filter(e =>
      e.raw.toLowerCase().includes(q) ||
      e.message.toLowerCase().includes(q) ||
      e.tag.toLowerCase().includes(q)
    )
  }
  return list
})

async function fetchLogs(showSpinner = false) {
  if (showSpinner) loading.value = true
  try {
    const res = await fetch('/api/logs')
    if (res.ok) {
      const data = await res.json()
      logs.value = data.entries || []
      logSource.value = data.source || 'dae runtime'
      lastUpdateTime.value = data.time || new Date().toLocaleTimeString()

      if (autoScroll.value) {
        await nextTick()
        scrollToBottom()
      }
    }
  } catch (err) {
    console.error('Fetch logs error:', err)
  } finally {
    if (showSpinner) loading.value = false
  }
}

function scrollToBottom() {
  if (consoleRef.value) {
    consoleRef.value.scrollTop = consoleRef.value.scrollHeight
  }
}

function clearConsole() {
  logs.value = []
}

function downloadCurrentLogs() {
  window.open('/api/logs/download', '_blank')
}

// Backups
async function fetchBackups() {
  try {
    const res = await fetch('/api/logs/backups')
    if (res.ok) {
      const data = await res.json()
      backupsList.value = data.backups || []
    }
  } catch (err) {
    console.error('Fetch log backups error:', err)
  }
}

async function createLogBackup() {
  creatingBackup.value = true
  try {
    const res = await fetch('/api/logs/backup', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label: backupLabel.value.trim() || 'snapshot' })
    })
    const data = await res.json()
    if (res.ok && data.ok) {
      backupLabel.value = ''
      await fetchBackups()
      alert(`日志快照备份成功！\n文件: ${data.name}`)
    } else {
      alert('备份失败: ' + (data.error || ''))
    }
  } catch (err: any) {
    alert('备份请求异常: ' + err.message)
  } finally {
    creatingBackup.value = false
  }
}

function downloadBackupItem(name: string) {
  window.open(`/api/logs/backups/${encodeURIComponent(name)}/download`, '_blank')
}

async function deleteBackupItem(name: string) {
  if (!confirm(`确认删除日志快照 [${name}]？`)) return
  try {
    const res = await fetch(`/api/logs/backups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    if (res.ok) {
      await fetchBackups()
    }
  } catch (e: any) {
    alert('删除备份失败: ' + e.message)
  }
}

function openBackupManager() {
  fetchBackups()
  showBackupModal.value = true
}

function copyLogLine(raw: string) {
  navigator.clipboard.writeText(raw)
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// Highlight matching words in message
function highlightText(text: string, query: string) {
  if (!query.trim()) return text
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return text.replace(new RegExp(`(${escaped})`, 'gi'), '<mark class="log-mark">$1</mark>')
}

onMounted(() => {
  fetchLogs(true)
  timer = setInterval(() => {
    if (autoRefresh.value) {
      fetchLogs(false)
    }
  }, 3000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="logs-page">
    <!-- Top Toolbar Header -->
    <header class="logs-toolbar">
      <div class="toolbar-left">
        <div class="page-title-badge">
          <Terminal class="title-icon" :size="20" />
          <div>
            <h2>日志管理与实时调试</h2>
            <span class="subtext">支持系统分流日志实时捕获、级别检索、快照备份与日志文件下载导出</span>
          </div>
        </div>

        <div class="source-tag">
          <span class="source-dot"></span>
          <span>数据来源: {{ logSource }}</span>
          <span class="time-hint">(更新于 {{ lastUpdateTime }})</span>
        </div>
      </div>

      <div class="toolbar-actions">
        <!-- Auto Refresh Toggle -->
        <button
          class="btn btn-secondary btn-sm"
          :class="{ active: autoRefresh }"
          @click="autoRefresh = !autoRefresh"
          title="切换是否自动拉取最新日志"
        >
          <component :is="autoRefresh ? Pause : Play" :size="13" />
          <span>{{ autoRefresh ? '自动拉取 (开)' : '自动拉取 (已暂停)' }}</span>
        </button>

        <!-- Auto Scroll Toggle -->
        <button
          class="btn btn-secondary btn-sm"
          :class="{ active: autoScroll }"
          @click="autoScroll = !autoScroll"
          title="新日志出现时自动平滑滚动到最新行"
        >
          <ArrowDownCircle :size="13" />
          <span>{{ autoScroll ? '滚动跟随 (开)' : '滚动跟随 (关)' }}</span>
        </button>

        <!-- Manual Refresh -->
        <button
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="fetchLogs(true)"
          title="立即手动拉取最新日志"
        >
          <RefreshCw :size="13" :class="{ spin: loading }" />
          <span>刷新</span>
        </button>

        <div class="divider"></div>

        <!-- Download Logs Button -->
        <button
          class="btn btn-secondary btn-sm"
          @click="downloadCurrentLogs"
          title="下载全部原始日志为 .log 文件"
        >
          <Download :size="13" />
          <span>日志下载</span>
        </button>

        <!-- Backup Logs Button -->
        <button
          class="btn btn-primary btn-sm"
          @click="openBackupManager"
          title="创建日志快照备份或管理历史快照"
        >
          <Archive :size="13" />
          <span>日志备份</span>
        </button>
      </div>
    </header>

    <!-- Filter Control Bar -->
    <div class="logs-filter-bar">
      <!-- Search Input -->
      <div class="search-box">
        <Search :size="14" class="search-icon text-muted" />
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索日志关键字、IP地址、域名、规则或错误信息..."
          class="search-input"
        />
        <button
          v-if="searchQuery"
          class="btn-clear"
          @click="searchQuery = ''"
          title="清除搜索"
        >
          &times;
        </button>
      </div>

      <!-- Level Pills Filter -->
      <div class="filter-pill-group">
        <span class="pill-label">级别筛选:</span>
        <button
          class="level-pill"
          :class="{ active: selectedLevel === 'ALL' }"
          @click="selectedLevel = 'ALL'"
        >
          全部 ({{ levelCounts.ALL }})
        </button>
        <button
          class="level-pill error"
          :class="{ active: selectedLevel === 'ERROR' }"
          @click="selectedLevel = 'ERROR'"
        >
          <ShieldAlert :size="12" /> 错误 ({{ levelCounts.ERROR }})
        </button>
        <button
          class="level-pill warn"
          :class="{ active: selectedLevel === 'WARN' }"
          @click="selectedLevel = 'WARN'"
        >
          <AlertTriangle :size="12" /> 警告 ({{ levelCounts.WARN }})
        </button>
        <button
          class="level-pill info"
          :class="{ active: selectedLevel === 'INFO' }"
          @click="selectedLevel = 'INFO'"
        >
          <Info :size="12" /> 信息 ({{ levelCounts.INFO }})
        </button>
        <button
          class="level-pill debug"
          :class="{ active: selectedLevel === 'DEBUG' }"
          @click="selectedLevel = 'DEBUG'"
        >
          <Bug :size="12" /> 调试 ({{ levelCounts.DEBUG }})
        </button>
      </div>

      <!-- Module Tag Filter -->
      <div class="filter-pill-group tag-group">
        <span class="pill-label">模块分类:</span>
        <select v-model="selectedTag" class="tag-select">
          <option value="all">全部模块</option>
          <option value="routing">分流规则</option>
          <option value="dns">域名解析</option>
          <option value="ebpf">内核探针</option>
          <option value="proxy">代理策略</option>
          <option value="system">系统核心</option>
        </select>
      </div>

      <!-- Count and Clear Screen -->
      <div class="filter-right-tools">
        <span class="matched-count">
          匹配 <strong>{{ filteredLogs.length }}</strong> / {{ logs.length }} 行
        </span>
        <button class="btn-text-sm" @click="clearConsole" title="临时清除屏幕显示的日志">
          清屏
        </button>
      </div>
    </div>

    <!-- Main Terminal Stream Viewer -->
    <main class="console-wrapper">
      <div ref="consoleRef" class="console-body mono">
        <div v-if="filteredLogs.length === 0" class="console-empty">
          <Terminal :size="36" class="empty-icon" />
          <p v-if="searchQuery || selectedLevel !== 'ALL' || selectedTag !== 'all'">
            未找到符合筛选条件的日志（搜索词: "{{ searchQuery }}"）
          </p>
          <p v-else>暂无实时日志记录，等待服务事件写入...</p>
        </div>

        <div
          v-for="(log, idx) in filteredLogs"
          :key="idx"
          class="log-row"
          :class="log.level.toLowerCase()"
          @dblclick="copyLogLine(log.raw)"
          title="双击复制此行原始日志"
        >
          <!-- Line number -->
          <span class="log-num">{{ idx + 1 }}</span>

          <!-- Timestamp -->
          <span class="log-ts">{{ log.timestamp }}</span>

          <!-- Level Badge -->
          <span class="log-level-badge" :class="log.level.toLowerCase()">
            {{ log.level }}
          </span>

          <!-- Module Tag -->
          <span class="log-tag-badge" :class="log.tag">
            [{{ log.tag }}]
          </span>

          <!-- Message Body with Keyword Highlight -->
          <span
            class="log-msg"
            v-html="highlightText(log.message, searchQuery)"
          ></span>

          <!-- Copy Button -->
          <button
            class="btn-copy-line"
            @click.stop="copyLogLine(log.raw)"
            title="复制单行"
          >
            <Copy :size="11" />
          </button>
        </div>
      </div>
    </main>

    <!-- Backup Manager Modal -->
    <div v-if="showBackupModal" class="modal-backdrop" @click.self="showBackupModal = false">
      <div class="modal-card modal-backup">
        <div class="modal-header">
          <div class="modal-title-row">
            <Archive :size="18" class="text-accent" />
            <h3>日志快照备份管理</h3>
          </div>
          <button class="close-btn" @click="showBackupModal = false">&times;</button>
        </div>

        <div class="modal-body">
          <!-- Create Backup Bar -->
          <div class="create-backup-section">
            <label class="section-lbl">创建当前日志快照备份</label>
            <div class="create-input-row">
              <input
                type="text"
                v-model="backupLabel"
                placeholder="填写备份备注（如: 分流异常排查前, 压力测试快照）"
                class="form-input"
                @keydown.enter="createLogBackup"
              />
              <button
                class="btn btn-primary"
                :disabled="creatingBackup"
                @click="createLogBackup"
              >
                <Archive :size="13" />
                <span>{{ creatingBackup ? '正在备份...' : '立即创建备份' }}</span>
              </button>
            </div>
          </div>

          <!-- Existing Backups List -->
          <div class="backups-list-section">
            <label class="section-lbl">历史日志快照文件 (共 {{ backupsList.length }} 份)</label>

            <div v-if="backupsList.length === 0" class="backups-empty">
              <FileText :size="32" class="text-muted" />
              <p>暂无历史日志备份，你可以点击上方按钮创建快照。</p>
            </div>

            <div v-else class="backups-table-wrap">
              <table class="backups-table">
                <thead>
                  <tr>
                    <th>备份文件名</th>
                    <th style="width: 100px">文件大小</th>
                    <th style="width: 150px">创建时间</th>
                    <th style="width: 120px; text-align: right">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="b in backupsList" :key="b.name">
                    <td class="mono b-name">{{ b.name }}</td>
                    <td class="mono">{{ formatBytes(b.size) }}</td>
                    <td class="text-muted">{{ b.modTime }}</td>
                    <td style="text-align: right">
                      <button
                        class="btn-icon"
                        @click="downloadBackupItem(b.name)"
                        title="下载此备份日志"
                      >
                        <Download :size="13" />
                      </button>
                      <button
                        class="btn-icon delete"
                        @click="deleteBackupItem(b.name)"
                        title="删除此备份"
                      >
                        <Trash2 :size="13" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showBackupModal = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  overflow: hidden;
}

/* Toolbar Header */
.logs-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
  gap: 16px;
  flex-wrap: wrap;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.page-title-badge {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-title-badge h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.page-title-badge .subtext {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.title-icon {
  color: #0284c7;
}

.source-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.source-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
}

.time-hint {
  font-size: 10px;
  opacity: 0.8;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.divider {
  width: 1px;
  height: 18px;
  background: var(--border-color, #334155);
  margin: 0 4px;
}

/* Filter Bar */
.logs-filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
  flex-wrap: wrap;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  padding: 4px 10px;
  border-radius: 6px;
  flex: 1;
  max-width: 360px;
  position: relative;
}

.search-input {
  background: transparent;
  border: none;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  outline: none;
  width: 100%;
}

.btn-clear {
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  font-size: 14px;
  cursor: pointer;
}

.filter-pill-group {
  display: flex;
  align-items: center;
  gap: 5px;
}

.pill-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
}

.level-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.level-pill:hover {
  color: var(--text-main, #f8fafc);
}

.level-pill.active {
  background: #0284c7;
  color: #fff;
  border-color: #0284c7;
  font-weight: 600;
}

.level-pill.error.active {
  background: #ef4444;
  border-color: #ef4444;
}

.level-pill.warn.active {
  background: #f59e0b;
  border-color: #f59e0b;
}

.level-pill.info.active {
  background: #0ea5e9;
  border-color: #0ea5e9;
}

.level-pill.debug.active {
  background: #8b5cf6;
  border-color: #8b5cf6;
}

.tag-select {
  padding: 4px 8px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 4px;
  color: var(--text-main, #f8fafc);
  font-size: 11px;
  outline: none;
}

.filter-right-tools {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}

.matched-count {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.matched-count strong {
  color: #0284c7;
}

.btn-text-sm {
  background: transparent;
  border: none;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  padding: 0;
}

.btn-text-sm:hover {
  color: #ef4444;
}

/* Console Wrapper */
.console-wrapper {
  flex: 1;
  background: var(--bg-input, #050811);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.console-body {
  flex: 1;
  overflow-y: auto;
  padding: 10px 14px;
  font-size: 12px;
  line-height: 1.6;
}

.console-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60%;
  color: var(--text-muted, #64748b);
  gap: 10px;
}

.empty-icon {
  opacity: 0.4;
}

.log-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 2px 6px;
  border-radius: 3px;
  position: relative;
  transition: background 0.1s;
}

.log-row:hover {
  background: rgba(255, 255, 255, 0.04);
}

.log-row:hover .btn-copy-line {
  opacity: 1;
}

.log-num {
  font-size: 10px;
  color: var(--text-muted, #475569);
  width: 34px;
  text-align: right;
  user-select: none;
  flex-shrink: 0;
}

.log-ts {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  flex-shrink: 0;
  user-select: none;
}

.log-level-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 3px;
  text-transform: uppercase;
  flex-shrink: 0;
}

.log-level-badge.info {
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
}

.log-level-badge.warn {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
}

.log-level-badge.error {
  background: rgba(239, 68, 68, 0.18);
  color: #f87171;
}

.log-level-badge.debug {
  background: rgba(139, 92, 246, 0.15);
  color: #c084fc;
}

.log-tag-badge {
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.log-tag-badge.routing {
  color: #34d399;
}

.log-tag-badge.dns {
  color: #60a5fa;
}

.log-tag-badge.ebpf {
  color: #f472b6;
}

.log-tag-badge.proxy {
  color: #fb923c;
}

.log-tag-badge.system {
  color: #94a3b8;
}

.log-msg {
  flex: 1;
  word-break: break-all;
  color: var(--text-main, #e2e8f0);
}

.log-row.error .log-msg {
  color: #fca5a5;
}

.log-row.warn .log-msg {
  color: #fde68a;
}

.log-row.debug .log-msg {
  color: #94a3b8;
}

.log-mark {
  background: rgba(234, 179, 8, 0.35);
  color: #fef08a;
  padding: 0 2px;
  border-radius: 2px;
}

.btn-copy-line {
  background: transparent;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
  padding: 2px 4px;
}

.btn-copy-line:hover {
  color: #0284c7;
}

/* Backups Modal */
.modal-card.modal-backup {
  width: 680px;
  max-width: 95vw;
}

.modal-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.modal-title-row h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.create-backup-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 20px;
}

.section-lbl {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.create-input-row {
  display: flex;
  gap: 10px;
}

.create-input-row .form-input {
  flex: 1;
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  outline: none;
}

.backups-empty {
  padding: 30px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--text-muted, #64748b);
  font-size: 12px;
}

.backups-table-wrap {
  max-height: 260px;
  overflow-y: auto;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
}

.backups-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.backups-table th {
  padding: 8px 12px;
  text-align: left;
  background: var(--bg-input, #090d16);
  color: var(--text-muted, #94a3b8);
  font-weight: 600;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.backups-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #f8fafc);
}

.b-name {
  color: #0284c7;
}

/* Helpers */
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s;
}

.btn.active {
  background: rgba(2, 132, 199, 0.2);
  color: #0284c7;
  border-color: rgba(2, 132, 199, 0.4);
}

.btn-sm {
  padding: 4px 10px;
  font-size: 11px;
}

.btn-primary {
  background: #0284c7;
  color: #ffffff;
}

.btn-primary:hover {
  background: #0369a1;
}

.btn-secondary {
  background: var(--bg-input, #1e293b);
  border-color: var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
}

.btn-secondary:hover {
  background: var(--border-color, #334155);
}

.btn-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
}

.btn-icon:hover {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #f8fafc);
}

.btn-icon.delete:hover {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
