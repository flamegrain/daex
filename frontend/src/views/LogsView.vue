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
      alert(t('logs.backup_success', { name: data.name }))
    } else {
      alert(t('logs.backup_failed', { error: data.error || '' }))
    }
  } catch (err: any) {
    alert(t('logs.backup_req_err', { error: err.message }))
  } finally {
    creatingBackup.value = false
  }
}

function downloadBackupItem(name: string) {
  window.open(`/api/logs/backups/${encodeURIComponent(name)}/download`, '_blank')
}

async function deleteBackupItem(name: string) {
  if (!confirm(t('logs.confirm_delete_backup', { name }))) return
  try {
    const res = await fetch(`/api/logs/backups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    if (res.ok) {
      await fetchBackups()
    }
  } catch (e: any) {
    alert(t('logs.delete_backup_failed', { error: e.message }))
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
            <h2>{{ t('logs.title') }}</h2>
            <span class="subtext">{{ t('logs.subtitle') }}</span>
          </div>
        </div>

        <div class="source-tag">
          <span class="source-dot"></span>
          <span>{{ t('logs.source', { source: logSource }) }}</span>
          <span class="time-hint">{{ t('logs.updated_at', { time: lastUpdateTime }) }}</span>
        </div>
      </div>

      <div class="toolbar-actions">
        <!-- Auto Refresh Toggle -->
        <button
          class="btn btn-secondary btn-sm"
          :class="{ active: autoRefresh }"
          @click="autoRefresh = !autoRefresh"
          :title="autoRefresh ? t('logs.auto_pull_on') : t('logs.auto_pull_off')"
        >
          <component :is="autoRefresh ? Pause : Play" :size="13" />
          <span>{{ autoRefresh ? t('logs.auto_pull_on') : t('logs.auto_pull_off') }}</span>
        </button>

        <!-- Auto Scroll Toggle -->
        <button
          class="btn btn-secondary btn-sm"
          :class="{ active: autoScroll }"
          @click="autoScroll = !autoScroll"
          :title="autoScroll ? t('logs.auto_scroll_on') : t('logs.auto_scroll_off')"
        >
          <ArrowDownCircle :size="13" />
          <span>{{ autoScroll ? t('logs.auto_scroll_on') : t('logs.auto_scroll_off') }}</span>
        </button>

        <!-- Manual Refresh -->
        <button
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="fetchLogs(true)"
          :title="t('logs.btn_refresh')"
        >
          <RefreshCw :size="13" :class="{ spin: loading }" />
          <span>{{ t('logs.btn_refresh') }}</span>
        </button>

        <div class="divider"></div>

        <!-- Download Logs Button -->
        <button
          class="btn btn-secondary btn-sm"
          @click="downloadCurrentLogs"
          :title="t('logs.btn_download')"
        >
          <Download :size="13" />
          <span>{{ t('logs.btn_download') }}</span>
        </button>

        <!-- Backup Logs Button -->
        <button
          class="btn btn-primary btn-sm"
          @click="openBackupManager"
          :title="t('logs.btn_backup')"
        >
          <Archive :size="13" />
          <span>{{ t('logs.btn_backup') }}</span>
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
          :placeholder="t('logs.search_ph')"
          class="search-input"
        />
        <button
          v-if="searchQuery"
          class="btn-clear"
          @click="searchQuery = ''"
          :title="t('common.cancel')"
        >
          &times;
        </button>
      </div>

      <!-- Level Pills Filter -->
      <div class="filter-pill-group">
        <span class="pill-label">{{ t('logs.level_filter') }}</span>
        <button
          class="level-pill"
          :class="{ active: selectedLevel === 'ALL' }"
          @click="selectedLevel = 'ALL'"
        >
          {{ t('logs.level_all') }} ({{ levelCounts.ALL }})
        </button>
        <button
          class="level-pill error"
          :class="{ active: selectedLevel === 'ERROR' }"
          @click="selectedLevel = 'ERROR'"
        >
          <ShieldAlert :size="12" /> {{ t('logs.level_error') }} ({{ levelCounts.ERROR }})
        </button>
        <button
          class="level-pill warn"
          :class="{ active: selectedLevel === 'WARN' }"
          @click="selectedLevel = 'WARN'"
        >
          <AlertTriangle :size="12" /> {{ t('logs.level_warn') }} ({{ levelCounts.WARN }})
        </button>
        <button
          class="level-pill info"
          :class="{ active: selectedLevel === 'INFO' }"
          @click="selectedLevel = 'INFO'"
        >
          <Info :size="12" /> {{ t('logs.level_info') }} ({{ levelCounts.INFO }})
        </button>
        <button
          class="level-pill debug"
          :class="{ active: selectedLevel === 'DEBUG' }"
          @click="selectedLevel = 'DEBUG'"
        >
          <Bug :size="12" /> {{ t('logs.level_debug') }} ({{ levelCounts.DEBUG }})
        </button>
      </div>

      <!-- Module Tag Filter -->
      <div class="filter-pill-group tag-group">
        <span class="pill-label">{{ t('logs.module_filter') }}</span>
        <select v-model="selectedTag" class="tag-select">
          <option value="all">{{ t('logs.module_all') }}</option>
          <option value="routing">routing</option>
          <option value="dns">dns</option>
          <option value="ebpf">ebpf</option>
          <option value="proxy">proxy</option>
          <option value="system">system</option>
        </select>
      </div>

      <!-- Count and Clear Screen -->
      <div class="filter-right-tools">
        <span class="matched-count">
          <strong>{{ filteredLogs.length }}</strong> / {{ logs.length }}
        </span>
        <button class="btn-text-sm" @click="clearConsole" :title="t('logs.btn_clear')">
          {{ t('logs.btn_clear') }}
        </button>
      </div>
    </div>

    <!-- Main Terminal Stream Viewer -->
    <main class="console-wrapper">
      <div ref="consoleRef" class="console-body mono">
        <div v-if="filteredLogs.length === 0" class="console-empty">
          <Terminal :size="36" class="empty-icon" />
          <p v-if="searchQuery || selectedLevel !== 'ALL' || selectedTag !== 'all'">
            {{ t('logs.no_matching_logs', { query: searchQuery }) }}
          </p>
          <p v-else>{{ t('logs.waiting_for_events') }}</p>
        </div>

        <div
          v-for="(log, idx) in filteredLogs"
          :key="idx"
          class="log-row"
          :class="log.level.toLowerCase()"
          @dblclick="copyLogLine(log.raw)"
          :title="t('logs.copy_row_dblclick')"
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
            :title="t('logs.copy_line')"
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
            <h3>{{ t('logs.modal_backup_title') }}</h3>
          </div>
          <button class="close-btn" @click="showBackupModal = false">&times;</button>
        </div>

        <div class="modal-body">
          <!-- Create Backup Bar -->
          <div class="create-backup-section">
            <label class="section-lbl">{{ t('logs.modal_backup_title') }}</label>
            <div class="create-input-row">
              <input
                type="text"
                v-model="backupLabel"
                :placeholder="t('logs.modal_backup_ph')"
                class="form-input"
                @keydown.enter="createLogBackup"
              />
              <button
                class="btn btn-primary"
                :disabled="creatingBackup"
                @click="createLogBackup"
              >
                <Archive :size="13" />
                <span>{{ creatingBackup ? t('logs.modal_backup_creating') : t('logs.modal_backup_create') }}</span>
              </button>
            </div>
          </div>

          <!-- Existing Backups List -->
          <div class="backups-list-section">
            <label class="section-lbl">{{ t('logs.modal_backup_title') }} ({{ backupsList.length }})</label>

            <div v-if="backupsList.length === 0" class="backups-empty">
              <FileText :size="32" class="text-muted" />
              <p>{{ t('logs.modal_backup_empty') }}</p>
            </div>

            <div v-else class="backups-table-wrap">
              <table class="backups-table">
                <thead>
                  <tr>
                    <th>{{ t('common.name') }}</th>
                    <th style="width: 100px">{{ t('common.size') }}</th>
                    <th style="width: 150px">{{ t('common.time') }}</th>
                    <th style="width: 120px; text-align: right">{{ t('common.action') }}</th>
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
                        :title="t('common.download')"
                      >
                        <Download :size="13" />
                      </button>
                      <button
                        class="btn-icon delete"
                        @click="deleteBackupItem(b.name)"
                        :title="t('common.delete')"
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
          <button class="btn btn-secondary" @click="showBackupModal = false">{{ t('common.close') }}</button>
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
