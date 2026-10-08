<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Home,
  FileText,
  Activity,
  Server,
  Terminal,
  Settings,
  CircleDot,
  RefreshCw,
  Cpu,
  ArrowUpCircle,
  GitFork
} from 'lucide-vue-next'
import ConfigView from './views/ConfigView.vue'
import RoutingView from './views/RoutingView.vue'
import DashboardView from './views/DashboardView.vue'
import NodesView from './views/NodesView.vue'
import UpdateView from './views/UpdateView.vue'
import SettingsView from './views/SettingsView.vue'
import LogsView from './views/LogsView.vue'
import ServiceView from './views/ServiceView.vue'
import { t, theme, applyTheme } from './composables/useSettings'

type Tab = 'home' | 'config' | 'routing' | 'nodes' | 'logs' | 'service' | 'update' | 'settings'

const activeTab = ref<Tab>('home')
const daeStatus = ref<Record<string, any> | null>(null)
const backendHealth = ref<Record<string, any> | null>(null)
const loadingStatus = ref(false)

async function refreshDaeStatus() {
  loadingStatus.value = true
  try {
    const [statusRes, healthRes] = await Promise.all([
      fetch('/api/dae/status'),
      fetch('/api/health')
    ])
    if (statusRes.ok) daeStatus.value = await statusRes.json()
    if (healthRes.ok) backendHealth.value = await healthRes.json()
  } catch (e) {
    console.error('Failed to get status:', e)
  } finally {
    loadingStatus.value = false
  }
}

onMounted(() => {
  applyTheme(theme.value)
  refreshDaeStatus()
  setInterval(refreshDaeStatus, 15000)
})
</script>

<template>
  <div class="app-layout" :data-theme="theme">
    <!-- Left Sidebar -->
    <aside class="sidebar">
      <!-- App Brand -->
      <div class="brand">
        <div class="brand-logo">
          <Cpu class="logo-icon" :size="20" />
        </div>
        <div class="brand-text">
          <h2>{{ t('brand.title') }}</h2>
          <span class="subtext">{{ t('brand.sub') }}</span>
        </div>
      </div>

      <!-- Navigation Menu -->
      <nav class="nav-menu">
        <div class="nav-section-title">{{ t('nav.section.overview') }}</div>
        <button
          class="nav-item"
          :class="{ active: activeTab === 'home' }"
          @click="activeTab = 'home'"
        >
          <Home class="nav-icon" :size="18" />
          <span>{{ t('nav.home') }}</span>
        </button>

        <div class="nav-section-title">{{ t('nav.section.core') }}</div>
        <button
          class="nav-item"
          :class="{ active: activeTab === 'config' }"
          @click="activeTab = 'config'"
        >
          <FileText class="nav-icon" :size="18" />
          <span>{{ t('nav.config') }}</span>
          <span class="badge">mock</span>
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'routing' }"
          @click="activeTab = 'routing'"
        >
          <GitFork class="nav-icon" :size="18" />
          <span>{{ t('nav.routing') }}</span>
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'nodes' }"
          @click="activeTab = 'nodes'"
        >
          <Server class="nav-icon" :size="18" />
          <span>{{ t('nav.nodes') }}</span>
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'logs' }"
          @click="activeTab = 'logs'"
        >
          <Terminal class="nav-icon" :size="18" />
          <span>{{ t('nav.logs') }}</span>
        </button>

        <div class="nav-section-title">{{ t('nav.section.system') }}</div>
        <button
          class="nav-item"
          :class="{ active: activeTab === 'service' }"
          @click="activeTab = 'service'"
        >
          <Activity class="nav-icon" :size="18" />
          <span>{{ t('nav.service') }}</span>
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'update' }"
          @click="activeTab = 'update'"
        >
          <ArrowUpCircle class="nav-icon" :size="18" />
          <span>{{ t('nav.update') }}</span>
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'settings' }"
          @click="activeTab = 'settings'"
        >
          <Settings class="nav-icon" :size="18" />
          <span>{{ t('nav.settings') }}</span>
        </button>
      </nav>

      <!-- Sidebar Footer: Daemon Status Info -->
      <div class="sidebar-footer">
        <div class="status-card">
          <div class="status-row">
            <span class="label">dae.service</span>
            <span
              class="status-pill"
              :class="{
                active: daeStatus?.ActiveState === 'active',
                inactive: daeStatus?.ActiveState !== 'active'
              }"
            >
              <CircleDot class="dot" :size="10" />
              {{ daeStatus?.ActiveState || 'unknown' }}
            </span>
          </div>

          <div class="status-meta">
            <div class="meta-row">
              <span>{{ t('status.version') }}:</span>
              <span class="version-tag">{{ daeStatus?.version?.split(' ')[2] || 'v2.x' }}</span>
            </div>
            <div class="meta-row">
              <span>{{ t('status.env') }}:</span>
              <span class="mock-tag">{{ t('status.env.mock') }}</span>
            </div>
          </div>

          <button
            class="refresh-btn"
            :disabled="loadingStatus"
            @click="refreshDaeStatus"
            :title="t('status.refresh')"
          >
            <RefreshCw class="icon" :size="12" :class="{ spin: loadingStatus }" />
            <span>{{ loadingStatus ? t('status.refreshing') : t('status.refresh') }}</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- Right Workspace Area -->
    <main class="workspace">
      <DashboardView v-if="activeTab === 'home'" />
      <ConfigView v-else-if="activeTab === 'config'" />
      <RoutingView v-else-if="activeTab === 'routing'" />
      <NodesView v-else-if="activeTab === 'nodes'" />

      <LogsView v-else-if="activeTab === 'logs'" />
      <ServiceView v-else-if="activeTab === 'service'" />
      <UpdateView v-else-if="activeTab === 'update'" />
      <SettingsView v-else-if="activeTab === 'settings'" />
    </main>
  </div>
</template>

<style>
/* Theme CSS Variables */
:root, html[data-theme="dark"] {
  --bg-main: #090d16;
  --bg-sidebar: #0b1120;
  --bg-card: #0f172a;
  --bg-input: #1e293b;
  --border-color: #1e293b;
  --border-subtle: #334155;
  --text-main: #f8fafc;
  --text-muted: #94a3b8;
  --nav-hover: rgba(30, 41, 59, 0.6);
  --nav-active: #1e293b;
  --status-bg: rgba(15, 23, 42, 0.7);
}

html[data-theme="light"] {
  --bg-main: #f8fafc;
  --bg-sidebar: #ffffff;
  --bg-card: #ffffff;
  --bg-input: #f1f5f9;
  --border-color: #e2e8f0;
  --border-subtle: #cbd5e1;
  --text-main: #0f172a;
  --text-muted: #64748b;
  --nav-hover: rgba(241, 245, 249, 0.9);
  --nav-active: #e2e8f0;
  --status-bg: rgba(241, 245, 249, 0.9);
}

/* Reset and global layout */
* {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  width: 100%;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  background-color: var(--bg-main);
  color: var(--text-main);
  overflow: hidden;
  transition: background-color 0.2s ease, color 0.2s ease;
}

#app {
  width: 100%;
  height: 100%;
}

.app-layout {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
}

/* Sidebar styling */
.sidebar {
  width: 260px;
  min-width: 260px;
  height: 100%;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-color);
}

.brand-logo {
  background: linear-gradient(135deg, #0284c7, #2563eb);
  color: #fff;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.3);
}

.brand-text h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.3px;
  color: var(--text-main);
}

.brand-text .subtext {
  font-size: 11px;
  color: var(--text-muted);
}

.nav-menu {
  flex: 1;
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
}

.nav-section-title {
  font-size: 11px;
  text-transform: uppercase;
  color: var(--text-muted);
  font-weight: 600;
  padding: 12px 10px 6px;
  letter-spacing: 0.5px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 7px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s ease;
}

.nav-item:hover {
  background: var(--nav-hover);
  color: var(--text-main);
}

.nav-item.active {
  background: var(--nav-active);
  color: #38bdf8;
  border-color: var(--border-subtle);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.nav-icon {
  flex-shrink: 0;
}

.badge {
  margin-left: auto;
  font-size: 10px;
  background: rgba(2, 132, 199, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(2, 132, 199, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
}

.sidebar-footer {
  padding: 14px;
  border-top: 1px solid var(--border-color);
  background: var(--bg-main);
}

.status-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 12px;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.status-row .label {
  font-size: 12px;
  font-weight: 600;
  color: #cbd5e1;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 12px;
}

.status-pill.active {
  background: #064e3b44;
  color: #34d399;
}

.status-pill.inactive {
  background: #7f1d1d44;
  color: #f87171;
}

.status-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 11px;
  color: #64748b;
  border-top: 1px solid #1e293b;
  padding-top: 8px;
  margin-bottom: 8px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
}

.version-tag {
  color: #94a3b8;
  font-family: monospace;
}

.mock-tag {
  color: #38bdf8;
  font-weight: 500;
}

.refresh-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: #1e293b;
  border: 1px solid #334155;
  color: #94a3b8;
  font-size: 11px;
  padding: 4px 8px;
  border-radius: 5px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.refresh-btn:hover {
  background: #334155;
  color: #fff;
}

.refresh-btn .spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  100% {
    transform: rotate(360deg);
  }
}

/* Workspace styling */
.workspace {
  flex: 1;
  height: 100%;
  background: var(--bg-main);
  overflow: hidden;
  position: relative;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.placeholder-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #64748b;
  padding: 40px;
  text-align: center;
}

.placeholder-view h3 {
  color: var(--text-main);
  margin: 16px 0 8px;
  font-size: 18px;
}

.placeholder-view p {
  max-width: 480px;
  line-height: 1.5;
  font-size: 13px;
}

.placeholder-view code {
  color: #38bdf8;
  background: var(--bg-input);
  padding: 2px 6px;
  border-radius: 4px;
}

.raw-status-box {
  margin-top: 20px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 16px;
  text-align: left;
  max-width: 600px;
  width: 100%;
  max-height: 300px;
  overflow: auto;
  font-size: 12px;
  color: var(--text-muted);
}

/* ========================================================
   GLOBAL LIGHT THEME OVERRIDES FOR ALL WORKSPACE VIEWS
   ======================================================== */
[data-theme="light"] .workspace,
[data-theme="light"] .home-dashboard,
[data-theme="light"] .config-view,
[data-theme="light"] .nodes-view,
[data-theme="light"] .update-view,
[data-theme="light"] .settings-view,
[data-theme="light"] .workspace-area,
[data-theme="light"] .form-mode-container,
[data-theme="light"] .form-section-body {
  background-color: #f8fafc !important;
  color: #0f172a !important;
}

[data-theme="light"] .settings-header,
[data-theme="light"] .view-header,
[data-theme="light"] .update-header,
[data-theme="light"] .toolbar,
[data-theme="light"] .form-subnav,
[data-theme="light"] .status-bar {
  background-color: #ffffff !important;
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .settings-card,
[data-theme="light"] .chart-card,
[data-theme="light"] .metric-card,
[data-theme="light"] .table-card,
[data-theme="light"] .item-card,
[data-theme="light"] .modal-card,
[data-theme="light"] .status-card,
[data-theme="light"] .nodes-section,
[data-theme="light"] .stat-card,
[data-theme="light"] .group-card,
[data-theme="light"] .sub-card,
[data-theme="light"] .simulator-modal,
[data-theme="light"] .update-card,
[data-theme="light"] .form-grid,
[data-theme="light"] .table-wrapper {
  background-color: #ffffff !important;
  border-color: #e2e8f0 !important;
  color: #0f172a !important;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.05), 0 1px 2px -1px rgba(0, 0, 0, 0.05) !important;
}

[data-theme="light"] .card-title,
[data-theme="light"] .modal-header,
[data-theme="light"] .section-title-bar {
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] .card-title h3,
[data-theme="light"] .modal-header h3,
[data-theme="light"] .section-title-bar h3,
[data-theme="light"] .dash-header .title,
[data-theme="light"] .settings-header h2,
[data-theme="light"] .metric-val,
[data-theme="light"] .bold {
  color: #0f172a !important;
}

[data-theme="light"] .card-info .row .lbl,
[data-theme="light"] .meta-label,
[data-theme="light"] .setting-label,
[data-theme="light"] .form-field label,
[data-theme="light"] .dash-header .subtitle,
[data-theme="light"] .subtitle,
[data-theme="light"] .section-title-bar p {
  color: #64748b !important;
}

[data-theme="light"] .card-info .row .val,
[data-theme="light"] .meta-value,
[data-theme="light"] .metric-title {
  color: #1e293b !important;
}

[data-theme="light"] .text-input,
[data-theme="light"] .select-input,
[data-theme="light"] input:not([type="checkbox"]),
[data-theme="light"] select {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #0f172a !important;
}

[data-theme="light"] .theme-btn {
  background-color: #f1f5f9 !important;
  border-color: #e2e8f0 !important;
  color: #64748b !important;
}
[data-theme="light"] .theme-btn:hover {
  background-color: #e2e8f0 !important;
  color: #0f172a !important;
}
[data-theme="light"] .theme-btn.active {
  background-color: #eff6ff !important;
  border-color: #3b82f6 !important;
  color: #2563eb !important;
}

[data-theme="light"] .user-meta-bar,
[data-theme="light"] .about-item,
[data-theme="light"] .raw-status-box {
  background-color: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme="light"] .notice-box {
  background-color: #eff6ff !important;
  border-color: #bfdbfe !important;
}
[data-theme="light"] .notice-text {
  color: #1e40af !important;
}

[data-theme="light"] .mode-switcher,
[data-theme="light"] .form-subnav {
  background-color: #f1f5f9 !important;
  border-color: #e2e8f0 !important;
}

[data-theme="light"] .mode-btn.active,
[data-theme="light"] .subnav-item.active {
  background-color: #ffffff !important;
  color: #0284c7 !important;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1) !important;
}

[data-theme="light"] .profile-select-wrapper {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
}
[data-theme="light"] .profile-select {
  color: #0284c7 !important;
}
[data-theme="light"] .profile-select option {
  background-color: #ffffff !important;
  color: #0f172a !important;
}

[data-theme="light"] .btn-secondary,
[data-theme="light"] .btn-icon-subtle,
[data-theme="light"] .refresh-pill {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #334155 !important;
}
[data-theme="light"] .btn-secondary:hover,
[data-theme="light"] .btn-icon-subtle:hover,
[data-theme="light"] .refresh-pill:hover {
  background-color: #f1f5f9 !important;
  color: #0f172a !important;
}

[data-theme="light"] .rule-table-wrap,
[data-theme="light"] .nodes-table-wrap,
[data-theme="light"] .sub-nodes-table-wrap,
[data-theme="light"] .table-container {
  background-color: #ffffff !important;
  border-color: #e2e8f0 !important;
}

[data-theme="light"] table th {
  background-color: #f8fafc !important;
  color: #64748b !important;
  border-bottom-color: #e2e8f0 !important;
}

[data-theme="light"] table td {
  border-bottom-color: #e2e8f0 !important;
  color: #1e293b !important;
}

[data-theme="light"] table tr:hover {
  background-color: #f8fafc !important;
}

[data-theme="light"] .empty-state-box {
  background-color: #f8fafc !important;
  border-color: #cbd5e1 !important;
}
[data-theme="light"] .empty-text {
  color: #334155 !important;
}

[data-theme="light"] .editor-host,
[data-theme="light"] .cm-editor {
  background-color: #ffffff !important;
  color: #0f172a !important;
}
[data-theme="light"] .cm-gutters {
  background-color: #f8fafc !important;
  border-right-color: #e2e8f0 !important;
  color: #94a3b8 !important;
}

[data-theme="light"] .proto-tag,
[data-theme="light"] .badge-mock,
[data-theme="light"] .proto-badge-sm,
[data-theme="light"] .code-tag,
[data-theme="light"] .filter-chip {
  background-color: #f1f5f9 !important;
  border-color: #cbd5e1 !important;
}

[data-theme="light"] .nodes-table {
  background-color: #ffffff !important;
}

[data-theme="light"] .connectivity-item {
  background-color: #f8fafc !important;
  border-color: #e2e8f0 !important;
}

[data-theme="light"] .connectivity-item:hover {
  background-color: #f1f5f9 !important;
  border-color: #0284c7 !important;
}

[data-theme="light"] .connectivity-card .latency-badge {
  background-color: #f1f5f9 !important;
}

[data-theme="light"] .btn-test-icon {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #64748b !important;
}

[data-theme="light"] .btn-test-icon:hover:not(:disabled) {
  background-color: #0284c7 !important;
  border-color: #0284c7 !important;
  color: #ffffff !important;
}
</style>
