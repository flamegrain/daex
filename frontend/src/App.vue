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
  Cpu
} from 'lucide-vue-next'
import ConfigView from './views/ConfigView.vue'
import DashboardView from './views/DashboardView.vue'
import SettingsView from './views/SettingsView.vue'
import { t } from './composables/useSettings'

type Tab = 'home' | 'config' | 'nodes' | 'logs' | 'service' | 'settings'

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
  refreshDaeStatus()
  setInterval(refreshDaeStatus, 15000)
})
</script>

<template>
  <div class="app-layout">
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
              <span>PID:</span>
              <span>{{ daeStatus?.MainPID || '-' }}</span>
            </div>
            <div class="meta-row">
              <span>版本:</span>
              <span class="version-tag">{{ daeStatus?.version?.split(' ')[2] || 'v2.x' }}</span>
            </div>
            <div class="meta-row">
              <span>环境:</span>
              <span class="mock-tag">安全假数据</span>
            </div>
          </div>

          <button
            class="refresh-btn"
            :disabled="loadingStatus"
            @click="refreshDaeStatus"
            title="刷新服务状态"
          >
            <RefreshCw class="icon" :size="12" :class="{ spin: loadingStatus }" />
            <span>刷新状态</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- Right Workspace Area -->
    <main class="workspace">
      <DashboardView v-if="activeTab === 'home'" />
      <ConfigView v-else-if="activeTab === 'config'" />

      <!-- Placeholder Views for Next Steps -->
      <div v-else-if="activeTab === 'nodes'" class="placeholder-view">
        <Server :size="48" class="ph-icon" />
        <h3>节点与订阅组视图（下一步开发中）</h3>
        <p>将基于工作区 <code>config.mock.dae</code> 实时解析展示 node/group/subscription，并提供测速与分流调试。</p>
      </div>

      <div v-else-if="activeTab === 'logs'" class="placeholder-view">
        <Terminal :size="48" class="ph-icon" />
        <h3>实时日志与调试（下一步开发中）</h3>
        <p>集成 WebSocket / SSE 实时推流 <code>journalctl -u dae -f</code>，提供分级高亮与关键字过滤。</p>
      </div>

      <div v-else-if="activeTab === 'service'" class="placeholder-view">
        <Activity :size="48" class="ph-icon" />
        <h3>系统服务与内核调试</h3>
        <div class="raw-status-box">
          <pre>{{ JSON.stringify({ daeStatus, backendHealth }, null, 2) }}</pre>
        </div>
      </div>

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
  background: #090d16;
  overflow: hidden;
  position: relative;
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
  color: #cbd5e1;
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
  background: #1e293b;
  padding: 2px 6px;
  border-radius: 4px;
}

.raw-status-box {
  margin-top: 20px;
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 8px;
  padding: 16px;
  text-align: left;
  max-width: 600px;
  width: 100%;
  max-height: 300px;
  overflow: auto;
  font-size: 12px;
  color: #94a3b8;
}
</style>
