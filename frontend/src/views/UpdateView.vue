<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  ArrowUpCircle,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  DownloadCloud,
  FileCode2,
  Database,
  Terminal,
  Cpu,
  Clock,
  Layers,
  ChevronDown,
  ChevronUp,
  ShieldCheck,
  HardDrive
} from 'lucide-vue-next'
import { t } from '../composables/useSettings'

interface DaeInfo {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  releaseName: string
  publishedAt: string
  changelog: string
  downloadUrl: string
  arch: string
}

interface RuleItem {
  name: string
  path: string
  size: string
  sizeBytes: number
  modTime: string
  latestTag: string
  publishedAt: string
  downloadUrl: string
  hasUpdate: boolean
}

interface UpdateOverview {
  dae: DaeInfo
  rules: Record<string, RuleItem>
  checkedAt: string
}

const loading = ref(true)
const updatingDae = ref(false)
const updatingRules = ref(false)
const updatingSingleRule = ref<string | null>(null)
const showChangelog = ref(true)

const overview = ref<UpdateOverview>({
  dae: {
    currentVersion: 'v2.1.1',
    latestVersion: 'v2.1.1',
    hasUpdate: false,
    releaseName: '',
    publishedAt: '',
    changelog: '',
    downloadUrl: '',
    arch: 'amd64'
  },
  rules: {
    geoip: {
      name: 'geoip.dat',
      path: '/usr/local/share/dae/geoip.dat',
      size: '15.8 MB',
      sizeBytes: 0,
      modTime: '-',
      latestTag: '-',
      publishedAt: '-',
      downloadUrl: '',
      hasUpdate: false
    },
    geosite: {
      name: 'geosite.dat',
      path: '/usr/local/share/dae/geosite.dat',
      size: '10.5 MB',
      sizeBytes: 0,
      modTime: '-',
      latestTag: '-',
      publishedAt: '-',
      downloadUrl: '',
      hasUpdate: false
    }
  },
  checkedAt: '-'
})

const executionLogs = ref<string[]>([])

async function fetchUpdateCheck() {
  loading.value = true
  try {
    const res = await fetch('/api/update/check')
    if (res.ok) {
      overview.value = await res.json()
    }
  } catch (e) {
    console.error('Fetch update check failed:', e)
  } finally {
    loading.value = false
  }
}

async function triggerDaeUpdate() {
  if (updatingDae.value) return
  updatingDae.value = true
  executionLogs.value = [
    `[${new Date().toLocaleTimeString()}] 🚀 准备升级 dae 核心主程序 (${overview.value.dae.latestVersion})...`
  ]

  try {
    const res = await fetch('/api/update/dae', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        downloadUrl: overview.value.dae.downloadUrl
      })
    })
    if (res.ok) {
      const result = await res.json()
      if (result.logs && Array.isArray(result.logs)) {
        executionLogs.value = result.logs
      }
      await fetchUpdateCheck()
    }
  } catch (e: any) {
    executionLogs.value.push(`❌ 请求失败: ${e?.message || e}`)
  } finally {
    updatingDae.value = false
  }
}

async function triggerRulesUpdate(types: string[] = []) {
  if (updatingRules.value) return
  updatingRules.value = true
  executionLogs.value = [
    `[${new Date().toLocaleTimeString()}] 🚀 准备更新规则库: ${types.length ? types.join(', ') : 'geoip, geosite'}...`
  ]

  try {
    const res = await fetch('/api/update/rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ types })
    })
    if (res.ok) {
      const result = await res.json()
      if (result.logs && Array.isArray(result.logs)) {
        executionLogs.value = result.logs
      }
      await fetchUpdateCheck()
    }
  } catch (e: any) {
    executionLogs.value.push(`❌ 更新失败: ${e?.message || e}`)
  } finally {
    updatingRules.value = false
    updatingSingleRule.value = null
  }
}

function updateSingle(name: string) {
  updatingSingleRule.value = name
  triggerRulesUpdate([name])
}

onMounted(() => {
  fetchUpdateCheck()
})
</script>

<template>
  <div class="update-view">
    <!-- Top Header -->
    <header class="update-header">
      <div class="header-left">
        <div class="title-row">
          <ArrowUpCircle :size="22" class="header-icon" />
          <h2>{{ t('update.title') }}</h2>
        </div>
        <p class="subtitle">{{ t('update.subtitle') }}</p>
      </div>

      <div class="header-actions">
        <button
          class="action-btn primary"
          :disabled="loading"
          @click="fetchUpdateCheck"
        >
          <RefreshCw :size="14" :class="{ spin: loading }" />
          <span>{{ loading ? t('update.checking') : t('update.check') }}</span>
        </button>
      </div>
    </header>

    <main class="update-content">
      <!-- Section 1: dae Core Binary -->
      <section class="update-card">
        <div class="card-top">
          <div class="card-title">
            <Cpu :size="18" class="card-icon" />
            <h3>{{ t('update.dae.card_title') }}</h3>
          </div>

          <div class="status-badge" :class="{ update: overview.dae.hasUpdate, latest: !overview.dae.hasUpdate }">
            <component :is="overview.dae.hasUpdate ? AlertCircle : CheckCircle2" :size="14" />
            <span>{{ overview.dae.hasUpdate ? t('update.dae.has_update') : t('update.dae.is_latest') }}</span>
          </div>
        </div>

        <!-- Metric Grid -->
        <div class="meta-grid">
          <div class="meta-item">
            <span class="meta-lbl">{{ t('update.dae.curr_ver') }}</span>
            <span class="meta-val mono bold">{{ overview.dae.currentVersion }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-lbl">{{ t('update.dae.latest_ver') }}</span>
            <span class="meta-val mono bold" :class="{ highlight: overview.dae.hasUpdate }">
              {{ overview.dae.latestVersion }}
            </span>
          </div>

          <div class="meta-item">
            <span class="meta-lbl">{{ t('update.dae.arch') }}</span>
            <span class="meta-val mono">{{ overview.dae.arch }}</span>
          </div>

          <div class="meta-item">
            <span class="meta-lbl">{{ t('update.dae.pub_date') }}</span>
            <span class="meta-val">{{ overview.dae.publishedAt || '-' }}</span>
          </div>
        </div>

        <!-- Action bar -->
        <div class="card-action-bar">
          <button
            class="btn-upgrade"
            :disabled="updatingDae"
            @click="triggerDaeUpdate"
          >
            <DownloadCloud :size="14" :class="{ spin: updatingDae }" />
            <span>{{ updatingDae ? t('update.dae.upgrading') : t('update.dae.upgrade_btn') }}</span>
          </button>
        </div>

        <!-- Changelog Accordion -->
        <div class="changelog-box">
          <div class="changelog-header" @click="showChangelog = !showChangelog">
            <div class="cl-title">
              <FileCode2 :size="15" />
              <span>{{ t('update.dae.changelog') }}</span>
            </div>
            <button class="toggle-btn">
              <component :is="showChangelog ? ChevronUp : ChevronDown" :size="14" />
            </button>
          </div>

          <div v-if="showChangelog" class="changelog-body mono">
            <pre v-if="overview.dae.changelog">{{ overview.dae.changelog }}</pre>
            <div v-else class="empty-cl">{{ t('update.dae.no_changelog') }}</div>
          </div>
        </div>
      </section>

      <!-- Section 2: geosite and geoip dat databases -->
      <section class="update-card">
        <div class="card-top">
          <div class="card-title">
            <Database :size="18" class="card-icon rules-icon" />
            <div>
              <h3>{{ t('update.rules.card_title') }}</h3>
              <p class="card-sub">{{ t('update.rules.subtitle') }}</p>
            </div>
          </div>

          <button
            class="action-btn secondary"
            :disabled="updatingRules"
            @click="triggerRulesUpdate()"
          >
            <DownloadCloud :size="14" :class="{ spin: updatingRules }" />
            <span>{{ updatingRules ? t('update.rules.updating') : t('update.rules.update_all') }}</span>
          </button>
        </div>

        <!-- Rules Table -->
        <div class="table-wrap">
          <table class="rules-table">
            <thead>
              <tr>
                <th style="width: 140px">{{ t('update.rules.file_name') }}</th>
                <th style="width: 25%">{{ t('update.rules.path') }}</th>
                <th style="width: 100px">{{ t('update.rules.size') }}</th>
                <th style="width: 160px">{{ t('update.rules.mod_time') }}</th>
                <th style="width: 130px">{{ t('update.rules.latest_tag') }}</th>
                <th style="width: 110px">{{ t('update.rules.status') }}</th>
                <th style="width: 90px; text-align: right">{{ t('update.rules.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(rule, key) in overview.rules" :key="key">
                <td>
                  <div class="rule-name-box">
                    <Database :size="14" class="rule-icon" />
                    <strong class="mono">{{ rule.name }}</strong>
                  </div>
                </td>

                <td class="path-cell mono truncate" :title="rule.path">
                  {{ rule.path }}
                </td>

                <td class="mono">{{ rule.size }}</td>

                <td>{{ rule.modTime }}</td>

                <td class="mono">{{ rule.latestTag || '-' }}</td>

                <td>
                  <span v-if="rule.hasUpdate" class="pill update">
                    <AlertCircle :size="12" /> {{ t('update.rules.has_update') }}
                  </span>
                  <span v-else class="pill latest">
                    <CheckCircle2 :size="12" /> {{ t('update.rules.up_to_date') }}
                  </span>
                </td>

                <td style="text-align: right">
                  <button
                    class="btn-sm"
                    :disabled="updatingSingleRule === key || updatingRules"
                    @click="updateSingle(String(key))"
                  >
                    <RefreshCw :size="12" :class="{ spin: updatingSingleRule === key }" />
                    <span>{{ t('update.rules.update_single') }}</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- Section 3: Execution Live Log Console -->
      <section class="update-card console-card">
        <div class="card-top">
          <div class="card-title">
            <Terminal :size="18" class="card-icon log-icon" />
            <h3>{{ t('update.log.title') }}</h3>
          </div>
        </div>

        <div class="console-box mono">
          <div v-if="executionLogs.length" class="log-stream">
            <div
              v-for="(log, idx) in executionLogs"
              :key="idx"
              class="log-line"
              :class="{
                success: log.includes('✅') || log.includes('✓'),
                warn: log.includes('⚠️'),
                err: log.includes('❌'),
                start: log.includes('🚀')
              }"
            >
              {{ log }}
            </div>
          </div>
          <div v-else class="console-empty">
            {{ t('update.log.empty') }}
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.update-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  overflow-y: auto;
  padding: 24px;
  gap: 20px;
}

/* Header */
.update-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-icon {
  color: #0284c7;
}

.title-row h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-muted, #64748b);
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.action-btn.primary {
  background: #0284c7;
  color: #fff;
}
.action-btn.primary:hover:not(:disabled) {
  background: #0369a1;
}

.action-btn.secondary {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #cbd5e1);
}
.action-btn.secondary:hover:not(:disabled) {
  background: var(--border-color, #334155);
  color: #0284c7;
}

.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  100% { transform: rotate(360deg); }
}

/* Content */
.update-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 1080px;
}

.update-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: background-color 0.2s, border-color 0.2s;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-title h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.card-sub {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.card-icon {
  color: #0284c7;
}
.card-icon.rules-icon {
  color: #10b981;
}
.card-icon.log-icon {
  color: #a855f7;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}
.status-badge.latest {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}
.status-badge.update {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

/* Meta Grid */
.meta-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  background: var(--bg-input, #0b1120);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px 16px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meta-lbl {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.meta-val {
  font-size: 15px;
  color: var(--text-main, #f8fafc);
}
.meta-val.bold {
  font-weight: 600;
}
.meta-val.highlight {
  color: #f59e0b;
}

.card-action-bar {
  display: flex;
  align-items: center;
}

.btn-upgrade {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #0284c7;
  color: #fff;
  border: none;
  padding: 8px 18px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-upgrade:hover:not(:disabled) {
  background: #0369a1;
}

/* Changelog Accordion */
.changelog-box {
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  overflow: hidden;
}

.changelog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--bg-input, #0b1120);
  cursor: pointer;
  user-select: none;
}

.cl-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
}

.toggle-btn {
  background: none;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
}

.changelog-body {
  padding: 14px;
  background: #020617;
  max-height: 280px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.6;
  color: #cbd5e1;
}

.changelog-body pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.empty-cl {
  color: var(--text-muted, #64748b);
  font-style: italic;
}

/* Rules Table */
.table-wrap {
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  overflow: hidden;
}

.rules-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.rules-table th {
  text-align: left;
  padding: 10px 14px;
  background: var(--bg-input, #0b1120);
  color: var(--text-muted, #64748b);
  border-bottom: 1px solid var(--border-color, #1e293b);
  font-weight: 500;
  white-space: nowrap;
}

.rules-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #1e293b55);
  color: var(--text-main, #cbd5e1);
  vertical-align: middle;
}

.rule-name-box {
  display: flex;
  align-items: center;
  gap: 6px;
}
.rule-icon {
  color: #10b981;
}

.path-cell {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}

.pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
}
.pill.latest {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}
.pill.update {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.btn-sm {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: #0284c7;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
}
.btn-sm:hover:not(:disabled) {
  background: var(--border-color, #334155);
  color: #38bdf8;
}

/* Console Log */
.console-card {
  padding-bottom: 16px;
}

.console-box {
  background: #020617;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px;
  min-height: 120px;
  max-height: 260px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.6;
}

.log-stream {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-line {
  color: #94a3b8;
}
.log-line.success {
  color: #34d399;
}
.log-line.warn {
  color: #fbbf24;
}
.log-line.err {
  color: #f87171;
}
.log-line.start {
  color: #38bdf8;
  font-weight: 600;
}

.console-empty {
  color: var(--text-muted, #64748b);
  font-size: 12px;
  font-style: italic;
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
