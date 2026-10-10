<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  Activity,
  Play,
  Square,
  RotateCw,
  RefreshCw,
  Power,
  Shield,
  Server,
  Terminal,
  Copy,
  CheckCircle2,
  AlertTriangle,
  Clock,
  Cpu,
  Layers,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Pause,
  Stethoscope,
  X
} from 'lucide-vue-next'
import { t } from '../composables/useSettings'

interface ServiceDetail {
  name: string
  displayName: string
  description: string
  activeState: string
  subState: string
  unitFileState: string
  mainPid: number
  activeEnterTimestamp: string
  uptime: string
  uptimeSeconds: number
  memoryBytes: number
  memoryFormatted: string
  cpuTimeSeconds: number
  version: string
  binPath: string
  configPath: string
  listenAddr?: string
  extra?: Record<string, any>
}

interface ServicesData {
  dae: ServiceDetail
  webui: ServiceDetail
  system: Record<string, any>
}

const servicesData = ref<ServicesData | null>(null)
const loading = ref(false)
const autoPoll = ref(true)
let pollTimer: any = null

// Action loading states
const actionLoading = ref<Record<string, boolean>>({})

// Collapsible logs
const showDaeLogs = ref(true)
const showWebuiLogs = ref(false)
const daeLogs = ref<string[]>([])
const webuiLogs = ref<string[]>([])
const loadingDaeLogs = ref(false)
const loadingWebuiLogs = ref(false)
const logsCopied = ref<string | null>(null)

// Toast message
const toast = ref<{ type: 'success' | 'error'; message: string } | null>(null)
let toastTimer: any = null

function showToast(type: 'success' | 'error', message: string) {
  if (toastTimer) clearTimeout(toastTimer)
  toast.value = { type, message }
  toastTimer = setTimeout(() => {
    toast.value = null
  }, 4000)
}

// Reconnecting overlay state for WebUI restart
const restartingWebui = ref(false)
const reconnectElapsed = ref(0)
let reconnectTimer: any = null

// Doctor Self-Check State
interface DoctorCheckItem {
  id: string
  name: string
  status: 'pass' | 'warn' | 'fail'
  message: string
  detail?: string
}

interface DoctorReport {
  timestamp: string
  status: 'healthy' | 'warning' | 'error'
  summary: {
    total: number
    passed: number
    warning: number
    failed: number
  }
  checks: DoctorCheckItem[]
}

const showDoctorModal = ref(false)
const runningDoctor = ref(false)
const doctorReport = ref<DoctorReport | null>(null)
const doctorReportCopied = ref(false)

async function runSystemDoctor() {
  runningDoctor.value = true
  showDoctorModal.value = true
  try {
    const res = await fetch('/api/system/doctor')
    if (res.ok) {
      doctorReport.value = await res.json()
    }
  } catch (err) {
    console.error('Run system doctor error:', err)
  } finally {
    runningDoctor.value = false
  }
}

function copyDoctorReport() {
  if (!doctorReport.value) return
  let text = `=== dae-webui 系统运行环境与健康自检报告 ===\n时间: ${doctorReport.value.timestamp}\n总体状态: ${doctorReport.value.status}\n检查统计: 共 ${doctorReport.value.summary.total} 项 (通过: ${doctorReport.value.summary.passed}, 警告: ${doctorReport.value.summary.warning}, 失败: ${doctorReport.value.summary.failed})\n\n详细检查项:\n`
  for (const c of doctorReport.value.checks) {
    text += `[${c.status.toUpperCase()}] ${c.name}: ${c.message} ${c.detail ? '(' + c.detail + ')' : ''}\n`
  }
  navigator.clipboard.writeText(text).then(() => {
    doctorReportCopied.value = true
    setTimeout(() => {
      doctorReportCopied.value = false
    }, 2000)
  })
}

async function fetchServiceStatus(showSpinner = false) {
  if (showSpinner) loading.value = true
  try {
    const res = await fetch('/api/service/status')
    if (res.ok) {
      servicesData.value = await res.json()
    }
  } catch (err) {
    console.error('Fetch service status error:', err)
  } finally {
    if (showSpinner) loading.value = false
  }
}

async function fetchLogs(target: 'dae' | 'webui') {
  if (target === 'dae') loadingDaeLogs.value = true
  else loadingWebuiLogs.value = true

  try {
    const res = await fetch(`/api/service/logs?target=${target}&lines=40`)
    if (res.ok) {
      const data = await res.json()
      if (target === 'dae') {
        daeLogs.value = data.logs || []
      } else {
        webuiLogs.value = data.logs || []
      }
    }
  } catch (err) {
    console.error(`Fetch ${target} logs error:`, err)
  } finally {
    if (target === 'dae') loadingDaeLogs.value = false
    else loadingWebuiLogs.value = false
  }
}

async function executeAction(target: 'dae' | 'webui', action: string) {
  const opKey = `${target}:${action}`

  // Confirmations for risky actions
  if (target === 'dae') {
    if (action === 'stop' && !confirm(t('service.confirm_stop_dae'))) return
    if (action === 'restart' && !confirm(t('service.confirm_restart_dae'))) return
    if (action === 'reload' && !confirm(t('service.confirm_reload_dae'))) return
  } else if (target === 'webui' && action === 'restart') {
    if (!confirm(t('service.confirm_restart_webui'))) return
  }

  actionLoading.value[opKey] = true

  if (target === 'webui' && action === 'restart') {
    // Special handling for restarting WebUI
    try {
      await fetch('/api/service/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target, action })
      })
    } catch (_) {
      // Ignored since server will drop connection on restart
    }
    actionLoading.value[opKey] = false
    startReconnectFlow()
    return
  }

  try {
    const res = await fetch('/api/service/action', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target, action })
    })
    const data = await res.json()
    if (res.ok && data.ok) {
      showToast('success', data.message || `${target} ${action} 成功`)
      await fetchServiceStatus(false)
      if (target === 'dae' && showDaeLogs.value) fetchLogs('dae')
      if (target === 'webui' && showWebuiLogs.value) fetchLogs('webui')
    } else {
      showToast('error', data.error || (data.details ? `${data.error}: ${data.details}` : '操作失败'))
    }
  } catch (err: any) {
    showToast('error', err.message || '网络请求异常')
  } finally {
    actionLoading.value[opKey] = false
  }
}

function startReconnectFlow() {
  restartingWebui.value = true
  reconnectElapsed.value = 0

  if (reconnectTimer) clearInterval(reconnectTimer)

  reconnectTimer = setInterval(async () => {
    reconnectElapsed.value++
    // Probe health endpoint after 1.5s
    if (reconnectElapsed.value >= 2) {
      try {
        const res = await fetch('/api/health')
        if (res.ok) {
          clearInterval(reconnectTimer)
          restartingWebui.value = false
          showToast('success', 'WebUI 控制台重连成功！')
          await fetchServiceStatus(true)
          if (showWebuiLogs.value) fetchLogs('webui')
        }
      } catch (_) {
        // Still down, keep probing
      }
    }
    // Safety max timeout: 30s
    if (reconnectElapsed.value > 30) {
      clearInterval(reconnectTimer)
      restartingWebui.value = false
      showToast('error', '重连超时，请检查服务状态或手动刷新网页。')
    }
  }, 1000)
}

function copyLogs(target: 'dae' | 'webui') {
  const content = (target === 'dae' ? daeLogs.value : webuiLogs.value).join('\n')
  navigator.clipboard.writeText(content).then(() => {
    logsCopied.value = target
    setTimeout(() => {
      logsCopied.value = null
    }, 2000)
  })
}

function getStatusBadge(state: string) {
  if (state === 'active') return { class: 'status-active', text: t('service.status_active') }
  if (state === 'inactive') return { class: 'status-inactive', text: t('service.status_inactive') }
  if (state === 'failed') return { class: 'status-failed', text: t('service.status_failed') }
  return { class: 'status-unknown', text: state || t('service.status_unknown') }
}

onMounted(() => {
  fetchServiceStatus(true)
  fetchLogs('dae')
  pollTimer = setInterval(() => {
    if (autoPoll.value && !restartingWebui.value) {
      fetchServiceStatus(false)
    }
  }, 3000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  if (reconnectTimer) clearInterval(reconnectTimer)
})
</script>

<template>
  <div class="service-page">
    <!-- Top Toolbar Header -->
    <header class="service-header">
      <div class="header-left">
        <div class="page-title-badge">
          <Activity class="title-icon" :size="20" />
          <div>
            <h2>{{ t('service.title') }}</h2>
            <span class="subtext">{{ t('service.subtitle') }}</span>
          </div>
        </div>
      </div>

      <div class="header-actions">
        <!-- Auto Poll Toggle -->
        <button
          class="btn btn-secondary btn-sm"
          :class="{ active: autoPoll }"
          @click="autoPoll = !autoPoll"
          :title="autoPoll ? '自动轮询 (开启)' : '自动轮询 (暂停)'"
        >
          <component :is="autoPoll ? Pause : Play" :size="13" />
          <span>{{ t('service.auto_refresh') }} ({{ autoPoll ? '开' : '关' }})</span>
        </button>

        <!-- Doctor Self-Check Button -->
        <button
          class="btn btn-primary btn-sm btn-doctor"
          :disabled="runningDoctor"
          @click="runSystemDoctor"
          :title="t('service.doctor_btn')"
        >
          <Stethoscope :size="13" :class="{ spin: runningDoctor }" />
          <span>{{ runningDoctor ? t('service.doctor_running') : t('service.doctor_btn') }}</span>
        </button>

        <!-- Manual Refresh Button -->
        <button
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="fetchServiceStatus(true)"
          :title="t('service.refresh')"
        >
          <RefreshCw :size="13" :class="{ spin: loading }" />
          <span>{{ loading ? t('service.refreshing') : t('service.refresh') }}</span>
        </button>
      </div>
    </header>

    <!-- Global Toast Banner -->
    <div v-if="toast" class="toast-banner" :class="toast.type">
      <component :is="toast.type === 'success' ? CheckCircle2 : AlertTriangle" :size="15" />
      <span>{{ toast.message }}</span>
    </div>

    <!-- Main Content Area: Cards Stack -->
    <main class="service-content">
      <!-- 1. dae Core Service Card -->
      <section class="service-card dae-card">
        <div class="card-header">
          <div class="card-identity">
            <div class="icon-avatar dae-avatar">
              <Shield :size="22" />
            </div>
            <div>
              <div class="title-row">
                <h3>{{ t('service.dae_title') }}</h3>
                <!-- Status Badge -->
                <span
                  class="badge-state"
                  :class="getStatusBadge(servicesData?.dae?.activeState || '').class"
                >
                  <span class="pulse-dot"></span>
                  {{ getStatusBadge(servicesData?.dae?.activeState || '').text }}
                </span>
                <!-- Autostart Badge -->
                <span
                  class="badge-autostart"
                  :class="servicesData?.dae?.unitFileState === 'enabled' ? 'enabled' : 'disabled'"
                >
                  {{ servicesData?.dae?.unitFileState === 'enabled' ? t('service.autostart_enabled') : t('service.autostart_disabled') }}
                </span>
              </div>
              <p class="desc">{{ t('service.dae_desc') }}</p>
            </div>
          </div>

          <!-- Actions Toolbar -->
          <div class="card-toolbar">
            <!-- Start -->
            <button
              class="btn btn-action btn-start"
              :disabled="servicesData?.dae?.activeState === 'active' || actionLoading['dae:start']"
              @click="executeAction('dae', 'start')"
              :title="t('service.btn_start')"
            >
              <Play :size="13" />
              <span>{{ t('service.btn_start') }}</span>
            </button>

            <!-- Stop -->
            <button
              class="btn btn-action btn-stop"
              :disabled="servicesData?.dae?.activeState !== 'active' || actionLoading['dae:stop']"
              @click="executeAction('dae', 'stop')"
              :title="t('service.btn_stop')"
            >
              <Square :size="13" />
              <span>{{ t('service.btn_stop') }}</span>
            </button>

            <!-- Restart -->
            <button
              class="btn btn-action btn-restart"
              :disabled="actionLoading['dae:restart']"
              @click="executeAction('dae', 'restart')"
              :title="t('service.btn_restart')"
            >
              <RotateCw :size="13" :class="{ spin: actionLoading['dae:restart'] }" />
              <span>{{ t('service.btn_restart') }}</span>
            </button>

            <!-- Reload -->
            <button
              class="btn btn-action btn-reload"
              :disabled="servicesData?.dae?.activeState !== 'active' || actionLoading['dae:reload']"
              @click="executeAction('dae', 'reload')"
              :title="t('service.btn_reload')"
            >
              <RefreshCw :size="13" :class="{ spin: actionLoading['dae:reload'] }" />
              <span>{{ t('service.btn_reload') }}</span>
            </button>

            <div class="v-divider"></div>

            <!-- Enable / Disable Autostart Toggle -->
            <button
              v-if="servicesData?.dae?.unitFileState === 'enabled'"
              class="btn btn-secondary btn-sm"
              :disabled="actionLoading['dae:disable']"
              @click="executeAction('dae', 'disable')"
              :title="t('service.btn_autostart_disable')"
            >
              <Power :size="13" />
              <span>{{ t('service.btn_autostart_disable') }}</span>
            </button>
            <button
              v-else
              class="btn btn-secondary btn-sm"
              :disabled="actionLoading['dae:enable']"
              @click="executeAction('dae', 'enable')"
              :title="t('service.btn_autostart_enable')"
            >
              <Power :size="13" />
              <span>{{ t('service.btn_autostart_enable') }}</span>
            </button>
          </div>
        </div>

        <!-- Metrics Grid -->
        <div class="metrics-grid">
          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_pid') }}</span>
            <span class="m-value mono highlight">{{ servicesData?.dae?.mainPid || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_uptime') }}</span>
            <span class="m-value">{{ servicesData?.dae?.uptime || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_memory') }}</span>
            <span class="m-value mono">{{ servicesData?.dae?.memoryFormatted || '0 B' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_cpu_time') }}</span>
            <span class="m-value mono">{{ (servicesData?.dae?.cpuTimeSeconds || 0).toFixed(2) }} s</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_version') }}</span>
            <span class="m-value mono version">{{ servicesData?.dae?.version || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_config') }}</span>
            <span class="m-value mono path" :title="servicesData?.dae?.configPath">{{ servicesData?.dae?.configPath || '-' }}</span>
          </div>
        </div>

        <!-- Collapsible Journal Logs Section -->
        <div class="logs-section">
          <div class="logs-header" @click="showDaeLogs = !showDaeLogs">
            <div class="logs-title-left">
              <Terminal :size="14" class="text-accent" />
              <span>{{ t('service.logs_title') }}</span>
              <span class="logs-tag">journalctl -u {{ servicesData?.dae?.name || 'dae' }}</span>
            </div>
            <div class="logs-header-actions" @click.stop>
              <button
                class="btn-icon-xs"
                @click="copyLogs('dae')"
                :title="t('service.logs_copy')"
              >
                <CheckCircle2 v-if="logsCopied === 'dae'" :size="13" class="text-success" />
                <Copy v-else :size="13" />
              </button>
              <button
                class="btn-icon-xs"
                :disabled="loadingDaeLogs"
                @click="fetchLogs('dae')"
                :title="t('service.logs_refresh')"
              >
                <RefreshCw :size="13" :class="{ spin: loadingDaeLogs }" />
              </button>
              <button class="btn-icon-xs" @click="showDaeLogs = !showDaeLogs">
                <component :is="showDaeLogs ? ChevronUp : ChevronDown" :size="14" />
              </button>
            </div>
          </div>

          <div v-if="showDaeLogs" class="logs-body">
            <div v-if="daeLogs.length === 0" class="logs-empty">
              <span>{{ t('service.logs_empty') }}</span>
            </div>
            <div v-else class="logs-console">
              <div v-for="(line, idx) in daeLogs" :key="idx" class="console-line">
                <span class="line-num">{{ idx + 1 }}</span>
                <span class="line-text">{{ line }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. WebUI Service Management Card -->
      <section class="service-card webui-card">
        <div class="card-header">
          <div class="card-identity">
            <div class="icon-avatar webui-avatar">
              <Server :size="22" />
            </div>
            <div>
              <div class="title-row">
                <h3>{{ t('service.webui_title') }} ({{ servicesData?.webui?.name || 'dae-webui.service' }})</h3>
                <!-- Status Badge -->
                <span
                  class="badge-state"
                  :class="getStatusBadge(servicesData?.webui?.activeState || 'active').class"
                >
                  <span class="pulse-dot"></span>
                  {{ getStatusBadge(servicesData?.webui?.activeState || 'active').text }}
                </span>
                <!-- Autostart Badge -->
                <span
                  class="badge-autostart"
                  :class="servicesData?.webui?.unitFileState === 'enabled' ? 'enabled' : 'disabled'"
                >
                  {{ servicesData?.webui?.unitFileState === 'enabled' ? t('service.autostart_enabled') : t('service.autostart_disabled') }}
                </span>
              </div>
              <p class="desc">{{ t('service.webui_desc') }}</p>
            </div>
          </div>

          <!-- Actions Toolbar -->
          <div class="card-toolbar">
            <!-- Restart WebUI Button -->
            <button
              class="btn btn-action btn-restart"
              :disabled="actionLoading['webui:restart']"
              @click="executeAction('webui', 'restart')"
              :title="t('service.btn_restart_webui')"
            >
              <RotateCw :size="13" :class="{ spin: actionLoading['webui:restart'] }" />
              <span>{{ t('service.btn_restart_webui') }}</span>
            </button>

            <!-- Autostart Toggle -->
            <button
              v-if="servicesData?.webui?.unitFileState === 'enabled'"
              class="btn btn-secondary btn-sm"
              :disabled="actionLoading['webui:disable']"
              @click="executeAction('webui', 'disable')"
              :title="t('service.btn_autostart_disable')"
            >
              <Power :size="13" />
              <span>{{ t('service.btn_autostart_disable') }}</span>
            </button>
            <button
              v-else
              class="btn btn-secondary btn-sm"
              :disabled="actionLoading['webui:enable']"
              @click="executeAction('webui', 'enable')"
              :title="t('service.btn_autostart_enable')"
            >
              <Power :size="13" />
              <span>{{ t('service.btn_autostart_enable') }}</span>
            </button>
          </div>
        </div>

        <!-- Metrics Grid -->
        <div class="metrics-grid">
          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_pid') }}</span>
            <span class="m-value mono highlight">{{ servicesData?.webui?.mainPid || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_listen') }}</span>
            <span class="m-value mono highlight">{{ servicesData?.webui?.listenAddr || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_uptime') }}</span>
            <span class="m-value">{{ servicesData?.webui?.uptime || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_memory') }}</span>
            <span class="m-value mono">{{ servicesData?.webui?.memoryFormatted || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_version') }}</span>
            <span class="m-value mono version">{{ servicesData?.webui?.version || '-' }}</span>
          </div>

          <div class="metric-item">
            <span class="m-label">{{ t('service.metric_arch') }}</span>
            <span class="m-value mono">
              {{ servicesData?.webui?.extra?.arch }}/{{ servicesData?.webui?.extra?.os }}
              <span class="tag-soft">({{ servicesData?.webui?.extra?.goVersion }})</span>
            </span>
          </div>
        </div>

        <!-- Collapsible WebUI Journal Logs Section -->
        <div class="logs-section">
          <div class="logs-header" @click="showWebuiLogs = !showWebuiLogs">
            <div class="logs-title-left">
              <Terminal :size="14" class="text-accent" />
              <span>{{ t('service.logs_title') }}</span>
              <span class="logs-tag">journalctl -u {{ servicesData?.webui?.name || 'dae-webui' }}</span>
            </div>
            <div class="logs-header-actions" @click.stop>
              <button
                class="btn-icon-xs"
                @click="copyLogs('webui')"
                :title="t('service.logs_copy')"
              >
                <CheckCircle2 v-if="logsCopied === 'webui'" :size="13" class="text-success" />
                <Copy v-else :size="13" />
              </button>
              <button
                class="btn-icon-xs"
                :disabled="loadingWebuiLogs"
                @click="fetchLogs('webui')"
                :title="t('service.logs_refresh')"
              >
                <RefreshCw :size="13" :class="{ spin: loadingWebuiLogs }" />
              </button>
              <button class="btn-icon-xs" @click="showWebuiLogs = !showWebuiLogs">
                <component :is="showWebuiLogs ? ChevronUp : ChevronDown" :size="14" />
              </button>
            </div>
          </div>

          <div v-if="showWebuiLogs" class="logs-body">
            <div v-if="webuiLogs.length === 0" class="logs-empty">
              <span>{{ t('service.logs_empty') }}</span>
            </div>
            <div v-else class="logs-console">
              <div v-for="(line, idx) in webuiLogs" :key="idx" class="console-line">
                <span class="line-num">{{ idx + 1 }}</span>
                <span class="line-text">{{ line }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- Fullscreen Reconnect Overlay when Restarting WebUI -->
    <div v-if="restartingWebui" class="reconnect-overlay">
      <div class="reconnect-modal">
        <RefreshCw :size="36" class="spin reconnect-spinner" />
        <h3>{{ t('service.restarting_webui_title') }}</h3>
        <p>{{ t('service.restarting_webui_desc', { seconds: reconnectElapsed }) }}</p>
        <div class="reconnect-bar">
          <div class="bar-progress"></div>
        </div>
      </div>
    </div>

    <!-- Doctor Modal -->
    <div v-if="showDoctorModal" class="doctor-modal-backdrop" @click.self="showDoctorModal = false">
      <div class="doctor-modal-card">
        <div class="doctor-modal-header">
          <div class="modal-title-wrap">
            <Stethoscope :size="20" class="doctor-modal-icon" />
            <h3>{{ t('service.doctor_title') }}</h3>
          </div>
          <button class="btn-close" @click="showDoctorModal = false" :title="t('service.doctor_close')">
            <X :size="18" />
          </button>
        </div>

        <div class="doctor-modal-body">
          <div v-if="runningDoctor" class="doctor-loading">
            <RefreshCw :size="32" class="spin text-primary" />
            <p>{{ t('service.doctor_running') }}</p>
          </div>

          <div v-else-if="doctorReport" class="doctor-report-content">
            <!-- Summary Banner -->
            <div
              class="doctor-summary-banner"
              :class="`status-${doctorReport.status}`"
            >
              <div class="summary-left">
                <CheckCircle2 v-if="doctorReport.status === 'healthy'" :size="24" class="status-icon text-success" />
                <AlertTriangle v-else-if="doctorReport.status === 'warning'" :size="24" class="status-icon text-warning" />
                <AlertTriangle v-else :size="24" class="status-icon text-danger" />
                <div>
                  <div class="summary-status-text">
                    {{ doctorReport.status === 'healthy' ? t('service.doctor_healthy') : (doctorReport.status === 'warning' ? t('service.doctor_warning') : t('service.doctor_error')) }}
                  </div>
                  <div class="summary-subtitle">
                    {{ doctorReport.timestamp }}
                  </div>
                </div>
              </div>
              <div class="summary-counts">
                <span class="count-tag passed">✓ {{ doctorReport.summary.passed }}</span>
                <span v-if="doctorReport.summary.warning > 0" class="count-tag warning">⚠ {{ doctorReport.summary.warning }}</span>
                <span v-if="doctorReport.summary.failed > 0" class="count-tag failed">✕ {{ doctorReport.summary.failed }}</span>
              </div>
            </div>

            <!-- Checks List -->
            <div class="doctor-checks-list">
              <div
                v-for="item in doctorReport.checks"
                :key="item.id"
                class="doctor-check-item"
                :class="`item-${item.status}`"
              >
                <div class="item-status-icon">
                  <CheckCircle2 v-if="item.status === 'pass'" :size="16" class="text-success" />
                  <AlertTriangle v-else-if="item.status === 'warn'" :size="16" class="text-warning" />
                  <AlertTriangle v-else :size="16" class="text-danger" />
                </div>
                <div class="item-content">
                  <div class="item-header">
                    <span class="item-name">{{ item.name }}</span>
                    <span class="item-status-badge" :class="item.status">{{ item.status.toUpperCase() }}</span>
                  </div>
                  <div class="item-message">{{ item.message }}</div>
                  <div v-if="item.detail" class="item-detail">{{ item.detail }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="doctor-modal-footer">
          <div class="footer-left">
            <button
              class="btn btn-secondary btn-sm"
              :disabled="runningDoctor"
              @click="runSystemDoctor"
            >
              <RefreshCw :size="14" :class="{ spin: runningDoctor }" />
              <span>{{ t('service.doctor_refresh') }}</span>
            </button>
            <button
              v-if="doctorReport"
              class="btn btn-secondary btn-sm"
              @click="copyDoctorReport"
            >
              <CheckCircle2 v-if="doctorReportCopied" :size="14" class="text-success" />
              <Copy v-else :size="14" />
              <span>{{ doctorReportCopied ? t('service.doctor_copied') : t('service.doctor_copy') }}</span>
            </button>
          </div>
          <button class="btn btn-primary btn-sm" @click="showDoctorModal = false">
            {{ t('service.doctor_close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.service-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  overflow-y: auto;
}

/* Header */
.service-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
  flex-shrink: 0;
}

.header-left .page-title-badge {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-icon {
  color: #0284c7;
}

.header-left h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.header-left .subtext {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Toast */
.toast-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 24px;
  font-size: 13px;
  font-weight: 500;
  animation: slideDown 0.2s ease-out;
}

.toast-banner.success {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border-bottom: 1px solid rgba(16, 185, 129, 0.3);
}

.toast-banner.error {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border-bottom: 1px solid rgba(239, 68, 68, 0.3);
}

@keyframes slideDown {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Content Area */
.service-content {
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Service Card */
.service-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.15);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  flex-wrap: wrap;
  gap: 16px;
}

.card-identity {
  display: flex;
  align-items: center;
  gap: 14px;
}

.icon-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 10px;
}

.dae-avatar {
  background: rgba(2, 132, 199, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(2, 132, 199, 0.3);
}

.webui-avatar {
  background: rgba(139, 92, 246, 0.15);
  color: #c084fc;
  border: 1px solid rgba(139, 92, 246, 0.3);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.title-row h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.desc {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

/* Badges */
.badge-state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
}

.badge-state.status-active {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.badge-state.status-inactive {
  background: rgba(100, 116, 139, 0.15);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.3);
}

.badge-state.status-failed {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.badge-state.status-unknown {
  background: rgba(148, 163, 184, 0.1);
  color: #94a3b8;
}

.pulse-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% { opacity: 0.4; }
  50% { opacity: 1; }
  100% { opacity: 0.4; }
}

.badge-autostart {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
}

.badge-autostart.enabled {
  background: rgba(2, 132, 199, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(2, 132, 199, 0.25);
}

.badge-autostart.disabled {
  background: rgba(100, 116, 139, 0.12);
  color: #64748b;
  border: 1px solid rgba(100, 116, 139, 0.25);
}

/* Card Toolbar Actions */
.card-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-action {
  font-size: 12px;
  font-weight: 500;
  padding: 6px 12px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid transparent;
}

.btn-start {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border-color: rgba(16, 185, 129, 0.3);
}

.btn-start:hover:not(:disabled) {
  background: #10b981;
  color: #ffffff;
}

.btn-stop {
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  border-color: rgba(239, 68, 68, 0.3);
}

.btn-stop:hover:not(:disabled) {
  background: #ef4444;
  color: #ffffff;
}

.btn-restart {
  background: #0284c7;
  color: #ffffff;
}

.btn-restart:hover:not(:disabled) {
  background: #0369a1;
}

.btn-reload {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #f8fafc);
  border-color: var(--border-color, #334155);
}

.btn-reload:hover:not(:disabled) {
  background: var(--border-color, #334155);
}

.v-divider {
  width: 1px;
  height: 20px;
  background: var(--border-color, #1e293b);
  margin: 0 4px;
}

/* Metrics Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1px;
  background: var(--border-color, #1e293b);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.metric-item {
  background: var(--bg-card, #0f172a);
  padding: 14px 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.m-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.m-value {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.m-value.highlight {
  color: #38bdf8;
}

.m-value.version {
  color: #34d399;
}

.m-value.path {
  font-size: 12px;
  color: #94a3b8;
}

.tag-soft {
  font-size: 11px;
  color: #64748b;
  margin-left: 4px;
}

/* Logs Section */
.logs-section {
  display: flex;
  flex-direction: column;
}

.logs-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 18px;
  background: rgba(0, 0, 0, 0.15);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.logs-header:hover {
  background: rgba(0, 0, 0, 0.25);
}

.logs-title-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.logs-tag {
  font-size: 11px;
  font-family: ui-monospace, monospace;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-input, #1e293b);
  color: var(--text-muted, #94a3b8);
}

.logs-header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.logs-body {
  border-top: 1px solid var(--border-color, #1e293b);
  background: #050811;
}

.logs-console {
  max-height: 240px;
  overflow-y: auto;
  padding: 10px 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.5;
}

.console-line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  color: #cbd5e1;
}

.console-line:hover {
  background: rgba(255, 255, 255, 0.03);
}

.line-num {
  font-size: 10px;
  color: #475569;
  width: 28px;
  text-align: right;
  flex-shrink: 0;
  user-select: none;
}

.line-text {
  flex: 1;
  word-break: break-all;
}

.logs-empty {
  padding: 24px;
  text-align: center;
  color: var(--text-muted, #64748b);
  font-size: 12px;
}

/* Reconnection Modal Overlay */
.reconnect-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
}

.reconnect-modal {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 12px;
  padding: 32px 36px;
  max-width: 440px;
  width: 90%;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
}

.reconnect-spinner {
  color: #0284c7;
  margin-bottom: 16px;
}

.reconnect-modal h3 {
  margin: 0 0 8px;
  font-size: 16px;
  color: var(--text-main, #f8fafc);
}

.reconnect-modal p {
  margin: 0 0 20px;
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.5;
}

.reconnect-bar {
  width: 100%;
  height: 4px;
  background: var(--bg-input, #1e293b);
  border-radius: 2px;
  overflow: hidden;
}

.bar-progress {
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, #0284c7, #38bdf8);
  animation: indeterminate 1.5s infinite ease-in-out;
}

@keyframes indeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

/* Helpers */
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn.active {
  background: rgba(2, 132, 199, 0.2);
  color: #0284c7;
  border-color: rgba(2, 132, 199, 0.4);
}

.btn-sm {
  padding: 5px 10px;
  font-size: 11px;
}

.btn-secondary {
  background: var(--bg-input, #1e293b);
  border-color: var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--border-color, #334155);
}

.btn-icon-xs {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
}

.btn-icon-xs:hover:not(:disabled) {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #f8fafc);
}

.text-accent {
  color: #0284c7;
}

.text-success {
  color: #10b981;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Light Theme Overrides */
[data-theme="light"] .service-page {
  background-color: #f8fafc !important;
  color: #0f172a !important;
}

[data-theme="light"] .service-header {
  background-color: #ffffff !important;
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .header-left h2 {
  color: #0f172a !important;
}

[data-theme="light"] .service-card {
  background-color: #ffffff !important;
  border-color: #e2e8f0 !important;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05) !important;
}

[data-theme="light"] .card-header {
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .title-row h3 {
  color: #0f172a !important;
}

[data-theme="light"] .metrics-grid {
  background-color: #e2e8f0 !important;
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .metric-item {
  background-color: #ffffff !important;
}

[data-theme="light"] .m-value {
  color: #0f172a !important;
}

[data-theme="light"] .logs-header {
  background-color: #f1f5f9 !important;
}

[data-theme="light"] .logs-title-left {
  color: #0f172a !important;
}

[data-theme="light"] .logs-body {
  border-top-color: #e2e8f0 !important;
  background-color: #f8fafc !important;
}

[data-theme="light"] .console-line {
  color: #334155 !important;
}

[data-theme="light"] .console-line:hover {
  background-color: #f1f5f9 !important;
}

[data-theme="light"] .btn-reload {
  background-color: #f1f5f9 !important;
  border-color: #cbd5e1 !important;
  color: #0f172a !important;
}

[data-theme="light"] .reconnect-modal {
  background-color: #ffffff !important;
  border-color: #e2e8f0 !important;
}

[data-theme="light"] .reconnect-modal h3 {
  color: #0f172a !important;
}

/* Doctor Modal Styles */
.doctor-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.doctor-modal-card {
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 12px;
  width: 100%;
  max-width: 680px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 8px 10px -6px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

.doctor-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #1e293b;
}

.modal-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.doctor-modal-icon {
  color: #0284c7;
}

.doctor-modal-header h3 {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #f8fafc;
}

.btn-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.btn-close:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #f8fafc;
}

.doctor-modal-body {
  padding: 20px;
  overflow-y: auto;
  flex: 1;
}

.doctor-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  gap: 16px;
  color: #94a3b8;
  font-size: 0.95rem;
}

.doctor-report-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.doctor-summary-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-radius: 8px;
  border: 1px solid transparent;
}

.doctor-summary-banner.status-healthy {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.3);
}

.doctor-summary-banner.status-warning {
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.3);
}

.doctor-summary-banner.status-error {
  background: rgba(239, 68, 68, 0.1);
  border-color: rgba(239, 68, 68, 0.3);
}

.summary-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.summary-status-text {
  font-weight: 600;
  font-size: 0.95rem;
  color: #f8fafc;
}

.summary-subtitle {
  font-size: 0.78rem;
  color: #94a3b8;
  margin-top: 2px;
}

.summary-counts {
  display: flex;
  gap: 6px;
}

.count-tag {
  font-size: 0.75rem;
  padding: 2px 8px;
  border-radius: 9999px;
  font-weight: 500;
}

.count-tag.passed {
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
}

.count-tag.warning {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
}

.count-tag.failed {
  background: rgba(239, 68, 68, 0.2);
  color: #f87171;
}

.doctor-checks-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.doctor-check-item {
  display: flex;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #1e293b;
  border: 1px solid #334155;
  transition: all 0.2s;
}

.doctor-check-item.item-warn {
  border-color: rgba(245, 158, 11, 0.4);
}

.doctor-check-item.item-fail {
  border-color: rgba(239, 68, 68, 0.4);
}

.item-status-icon {
  margin-top: 2px;
  flex-shrink: 0;
}

.item-content {
  flex: 1;
  min-width: 0;
}

.item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.item-name {
  font-weight: 600;
  font-size: 0.88rem;
  color: #f1f5f9;
}

.item-status-badge {
  font-size: 0.7rem;
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 700;
  text-transform: uppercase;
}

.item-status-badge.pass,
.item-status-badge.ok {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.item-status-badge.warn {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.item-status-badge.fail {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.item-message {
  font-size: 0.82rem;
  color: #cbd5e1;
  line-height: 1.4;
}

.item-detail {
  font-size: 0.76rem;
  color: #94a3b8;
  margin-top: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  background: rgba(0, 0, 0, 0.2);
  padding: 4px 8px;
  border-radius: 4px;
  word-break: break-all;
}

.doctor-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-top: 1px solid #1e293b;
  background: rgba(15, 23, 42, 0.6);
}

.footer-left {
  display: flex;
  gap: 8px;
}

/* Light Theme Overrides for Doctor Modal */
[data-theme="light"] .doctor-modal-card {
  background: #ffffff !important;
  border-color: #e2e8f0 !important;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.05);
}

[data-theme="light"] .doctor-modal-header {
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .doctor-modal-header h3 {
  color: #0f172a !important;
}

[data-theme="light"] .btn-close {
  color: #64748b !important;
}

[data-theme="light"] .btn-close:hover {
  background: #f1f5f9 !important;
  color: #0f172a !important;
}

[data-theme="light"] .summary-status-text {
  color: #0f172a !important;
}

[data-theme="light"] .doctor-check-item {
  background: #f8fafc !important;
  border-color: #e2e8f0 !important;
}

[data-theme="light"] .item-name {
  color: #0f172a !important;
}

[data-theme="light"] .item-message {
  color: #334155 !important;
}

[data-theme="light"] .item-detail {
  background: #f1f5f9 !important;
  color: #475569 !important;
}

[data-theme="light"] .doctor-modal-footer {
  border-top-color: #e2e8f0 !important;
  background: #f8fafc !important;
}
</style>
