<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Server,
  Zap,
  Route,
  RefreshCw,
  Search,
  Filter,
  CheckCircle2,
  Clock,
  Layers,
  Radio,
  Rss,
  ArrowRight,
  ShieldAlert,
  ChevronRight,
  ExternalLink,
  SlidersHorizontal,
  Compass,
  X,
  Play
} from 'lucide-vue-next'
import { theme, t } from '../composables/useSettings'

interface NodeItem {
  key: string
  protocol: string
  name: string
  address: string
  port: string
  country: string
  flag: string
  active: boolean
  latency: number
  url?: string
}

interface GroupOverviewItem {
  name: string
  policy: string
  filters: string[]
  nodeKeys: string[] | null
  nodeCount: number
  selectedNode?: NodeItem
}

interface SubscriptionOverviewItem {
  tag: string
  url: string
  nodeCount: number
}

interface NodesOverviewResponse {
  activeProfile: string
  nodes: NodeItem[]
  groups: GroupOverviewItem[]
  subscriptions: SubscriptionOverviewItem[]
  totalNodes: number
  activeNodes: number
  lastTestedAt?: number
}

interface RouteTraceResult {
  matched: boolean
  matchedRule: string
  target: string
  targetType: string
  selectedNode?: NodeItem
  traceSteps: string[]
}

const activeTab = ref<'nodes' | 'groups' | 'subscriptions'>('nodes')
const loading = ref(true)
const pingingAll = ref(false)
const pingingKeys = ref<Set<string>>(new Set())

const overview = ref<NodesOverviewResponse>({
  activeProfile: 'config.mock.dae',
  nodes: [],
  groups: [],
  subscriptions: [],
  totalNodes: 0,
  activeNodes: 0
})

// Search & Filters
const searchQuery = ref('')
const protocolFilter = ref('ALL')
const countryFilter = ref('ALL')
const sortBy = ref<'default' | 'latency-asc' | 'name-asc'>('default')

// Routing Simulator Modal
const showSimulator = ref(false)
const simDomain = ref('www.google.com')
const simIP = ref('142.250.190.46')
const simPort = ref('443')
const simProto = ref('tcp')
const simProcess = ref('')
const simLoading = ref(false)
const simResult = ref<RouteTraceResult | null>(null)

// Protocols & Countries for filtering
const availableProtocols = computed(() => {
  const set = new Set<string>()
  overview.value.nodes.forEach(n => {
    if (n.protocol) set.add(n.protocol)
  })
  return Array.from(set)
})

const availableCountries = computed(() => {
  const set = new Set<string>()
  overview.value.nodes.forEach(n => {
    if (n.country) set.add(n.country)
  })
  return Array.from(set)
})

const filteredNodes = computed(() => {
  let list = overview.value.nodes.filter(n => {
    if (protocolFilter.value !== 'ALL' && n.protocol !== protocolFilter.value) return false
    if (countryFilter.value !== 'ALL' && n.country !== countryFilter.value) return false
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = n.name.toLowerCase().includes(q)
      const matchKey = n.key.toLowerCase().includes(q)
      const matchAddr = n.address.toLowerCase().includes(q)
      if (!matchName && !matchKey && !matchAddr) return false
    }
    return true
  })

  if (sortBy.value === 'latency-asc') {
    list = [...list].sort((a, b) => a.latency - b.latency)
  } else if (sortBy.value === 'name-asc') {
    list = [...list].sort((a, b) => a.name.localeCompare(b.name))
  }
  return list
})

async function fetchOverview() {
  loading.value = true
  try {
    const res = await fetch('/api/nodes/overview')
    if (res.ok) {
      overview.value = await res.json()
    }
  } catch (e) {
    console.error('Fetch nodes overview failed:', e)
  } finally {
    loading.value = false
  }
}

async function pingAllNodes() {
  if (pingingAll.value) return
  pingingAll.value = true
  try {
    const res = await fetch('/api/nodes/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keys: [] })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.results && Array.isArray(data.results)) {
        const map = new Map<string, number>()
        data.results.forEach((r: any) => map.set(r.key, r.latency))
        overview.value.nodes.forEach(n => {
          if (map.has(n.key)) {
            n.latency = map.get(n.key)!
          }
        })
        overview.value.lastTestedAt = data.lastTestedAt
      }
    }
  } catch (e) {
    console.error('Ping all nodes failed:', e)
  } finally {
    pingingAll.value = false
  }
}

async function pingSingleNode(key: string) {
  if (pingingKeys.value.has(key)) return
  pingingKeys.value.add(key)
  try {
    const res = await fetch('/api/nodes/ping', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keys: [key] })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.results && data.results.length > 0) {
        const r = data.results[0]
        const target = overview.value.nodes.find(n => n.key === key)
        if (target) {
          target.latency = r.latency
        }
      }
    }
  } catch (e) {
    console.error('Ping single node failed:', e)
  } finally {
    pingingKeys.value.delete(key)
  }
}

async function runRoutingSimulator() {
  simLoading.value = true
  simResult.value = null
  try {
    const res = await fetch('/api/routing/trace', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        domain: simDomain.value,
        ip: simIP.value,
        port: simPort.value,
        protocol: simProto.value,
        process: simProcess.value
      })
    })
    if (res.ok) {
      simResult.value = await res.json()
    }
  } catch (e) {
    console.error('Run trace error:', e)
  } finally {
    simLoading.value = false
  }
}

function applyPreset(domain: string, ip: string, desc: string) {
  simDomain.value = domain
  simIP.value = ip
  runRoutingSimulator()
}

function formatTime(ts?: number) {
  if (!ts) return t('nodes.not_tested')
  const d = new Date(ts * 1000)
  return d.toLocaleTimeString([], { hour12: false })
}

onMounted(() => {
  fetchOverview()
})
</script>

<template>
  <div class="nodes-view">
    <!-- Top Header -->
    <header class="view-header">
      <div class="header-left">
        <div class="title-row">
          <h2>{{ t('nodes.title') }}</h2>
          <span class="profile-badge">
            <Radio :size="12" class="live-dot" />
            {{ t('nodes.active_profile') }}: {{ overview.activeProfile }}
          </span>
        </div>
        <p class="subtitle">{{ t('nodes.subtitle') }}</p>
      </div>

      <div class="header-actions">
        <button
          class="action-btn primary"
          :disabled="pingingAll"
          @click="pingAllNodes"
          :title="t('nodes.ping.all')"
        >
          <Zap :size="14" :class="{ spin: pingingAll }" />
          <span>{{ pingingAll ? t('nodes.ping.running') : t('nodes.ping.all') }}</span>
        </button>

        <button
          class="action-btn secondary"
          @click="showSimulator = true"
          :title="t('nodes.trace.title')"
        >
          <Compass :size="14" />
          <span>{{ t('nodes.trace.title') }}</span>
        </button>

        <button
          class="action-btn icon-only"
          :disabled="loading"
          @click="fetchOverview"
          :title="t('common.refresh')"
        >
          <RefreshCw :size="14" :class="{ spin: loading }" />
        </button>
      </div>
    </header>

    <!-- Stat Metric Bar -->
    <section class="stat-bar">
      <div class="stat-card">
        <div class="stat-label">
          <Server :size="14" /> {{ t('nodes.stat.total') }}
        </div>
        <div class="stat-val">
          {{ overview.totalNodes }}
          <span class="stat-sub">{{ t('nodes.stat.active') }} {{ overview.activeNodes }}</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">
          <Layers :size="14" /> {{ t('nodes.stat.groups') }}
        </div>
        <div class="stat-val">
          {{ overview.groups.length }}
          <span class="stat-sub">{{ t('nodes.stat.groups_sub') }}</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">
          <Rss :size="14" /> {{ t('nodes.stat.subs') }}
        </div>
        <div class="stat-val">
          {{ overview.subscriptions.length }}
          <span class="stat-sub">{{ t('nodes.stat.subs_sub') }}</span>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-label">
          <Clock :size="14" /> {{ t('nodes.stat.tested') }}
        </div>
        <div class="stat-val">
          {{ formatTime(overview.lastTestedAt) }}
          <span class="stat-sub">{{ t('nodes.stat.tested_sub') }}</span>
        </div>
      </div>
    </section>

    <!-- Sub Navigation Tabs -->
    <div class="view-tabs">
      <button
        class="tab-btn"
        :class="{ active: activeTab === 'nodes' }"
        @click="activeTab = 'nodes'"
      >
        <Server :size="15" />
        <span>{{ t('nodes.tab.nodes') }}</span>
        <span class="pill-count">{{ overview.nodes.length }}</span>
      </button>

      <button
        class="tab-btn"
        :class="{ active: activeTab === 'groups' }"
        @click="activeTab = 'groups'"
      >
        <Layers :size="15" />
        <span>{{ t('nodes.tab.groups') }}</span>
        <span class="pill-count">{{ overview.groups.length }}</span>
      </button>

      <button
        class="tab-btn"
        :class="{ active: activeTab === 'subscriptions' }"
        @click="activeTab = 'subscriptions'"
      >
        <Rss :size="15" />
        <span>{{ t('nodes.tab.subscriptions') }}</span>
        <span class="pill-count">{{ overview.subscriptions.length }}</span>
      </button>
    </div>

    <!-- Main Content Body -->
    <main class="view-body">
      <!-- ================= Tab 1: Nodes List ================= -->
      <section v-if="activeTab === 'nodes'" class="nodes-container">
        <!-- Search & Filter Controls -->
        <div class="filter-toolbar">
          <div class="search-box">
            <Search :size="14" class="search-icon" />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('nodes.filter.search_placeholder')"
              class="search-input"
            />
            <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">
              <X :size="12" />
            </button>
          </div>

          <div class="filter-group">
            <div class="select-wrap">
              <span class="sel-label">{{ t('nodes.filter.proto') }}:</span>
              <select v-model="protocolFilter" class="toolbar-select">
                <option value="ALL">{{ t('nodes.filter.proto.all') }}</option>
                <option v-for="p in availableProtocols" :key="p" :value="p">{{ p }}</option>
              </select>
            </div>

            <div class="select-wrap">
              <span class="sel-label">{{ t('nodes.filter.region') }}:</span>
              <select v-model="countryFilter" class="toolbar-select">
                <option value="ALL">{{ t('nodes.filter.region.all') }}</option>
                <option v-for="c in availableCountries" :key="c" :value="c">{{ c }}</option>
              </select>
            </div>

            <div class="select-wrap">
              <span class="sel-label">{{ t('nodes.filter.sort') }}:</span>
              <select v-model="sortBy" class="toolbar-select">
                <option value="default">{{ t('nodes.filter.sort.default') }}</option>
                <option value="latency-asc">{{ t('nodes.filter.sort.latency') }}</option>
                <option value="name-asc">{{ t('nodes.filter.sort.name') }}</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Nodes Table Wrapper -->
        <div class="table-card">
          <table class="nodes-table">
            <thead>
              <tr>
                <th style="width: 80px">{{ t('nodes.table.status') }}</th>
                <th style="width: 120px">{{ t('nodes.table.region') }}</th>
                <th style="width: 25%">{{ t('nodes.table.name') }}</th>
                <th style="width: 90px">{{ t('nodes.table.proto') }}</th>
                <th style="width: 28%">{{ t('nodes.table.address') }}</th>
                <th style="width: 140px">{{ t('nodes.table.latency') }}</th>
                <th style="width: 90px; text-align: right">{{ t('nodes.table.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="node in filteredNodes"
                :key="node.key"
                :class="{ 'row-active': node.active }"
              >
                <td>
                  <span v-if="node.active" class="active-pill" :title="t('nodes.active_traffic_title')">
                    <CheckCircle2 :size="12" /> {{ t('nodes.table.active') }}
                  </span>
                  <span v-else class="standby-pill">{{ t('nodes.table.standby') }}</span>
                </td>

                <td class="country-cell">
                  <span class="flag-icon">{{ node.flag }}</span>
                  <span class="country-name">{{ node.country }}</span>
                </td>

                <td class="name-cell">
                  <div class="node-title truncate" :title="node.name">{{ node.name }}</div>
                  <div class="node-key truncate mono" :title="node.key">{{ node.key }}</div>
                </td>

                <td>
                  <span
                    class="proto-badge"
                    :class="node.protocol.toLowerCase()"
                  >
                    {{ node.protocol }}
                  </span>
                </td>

                <td class="addr-cell">
                  <div class="addr-box mono truncate" :title="node.address + ':' + node.port">
                    <span>{{ node.address }}</span>
                    <span class="port-tag">:{{ node.port }}</span>
                  </div>
                </td>

                <td>
                  <div class="latency-box">
                    <div
                      class="latency-bar"
                      :class="{
                        fast: node.latency < 50,
                        medium: node.latency >= 50 && node.latency < 100,
                        slow: node.latency >= 100
                      }"
                      :style="{ width: Math.min(100, (node.latency / 200) * 100) + '%' }"
                    ></div>
                    <span
                      class="latency-val mono"
                      :class="{
                        fast: node.latency < 50,
                        medium: node.latency >= 50 && node.latency < 100,
                        slow: node.latency >= 100
                      }"
                    >
                      {{ node.latency }} ms
                    </span>
                  </div>
                </td>

                <td style="text-align: right">
                  <button
                    class="btn-ping"
                    :disabled="pingingKeys.has(node.key)"
                    @click="pingSingleNode(node.key)"
                    :title="t('nodes.ping.single')"
                  >
                    <Zap :size="12" :class="{ spin: pingingKeys.has(node.key) }" />
                    <span>{{ t('nodes.ping.single') }}</span>
                  </button>
                </td>
              </tr>

              <tr v-if="filteredNodes.length === 0">
                <td colspan="7" class="empty-row">
                  {{ t('nodes.table.empty') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <!-- ================= Tab 2: Policy Groups ================= -->
      <section v-else-if="activeTab === 'groups'" class="groups-container">
        <div class="groups-grid">
          <div
            v-for="group in overview.groups"
            :key="group.name"
            class="group-card"
          >
            <div class="group-header">
              <div class="group-title-wrap">
                <Layers :size="18" class="group-icon" />
                <h3 class="group-name">{{ group.name }}</h3>
              </div>
              <span class="policy-pill">
                {{ group.policy }}
              </span>
            </div>

            <!-- Active Selected Node in this Group -->
            <div class="group-selected-box">
              <div class="box-label">🎯 {{ t('nodes.group.candidate') }}:</div>
              <div v-if="group.selectedNode" class="selected-node-item">
                <div class="sel-info">
                  <span class="flag-icon">{{ group.selectedNode.flag }}</span>
                  <strong class="truncate">{{ group.selectedNode.name }}</strong>
                  <span class="proto-badge sm">{{ group.selectedNode.protocol }}</span>
                </div>
                <div class="sel-latency mono">
                  <span class="fast">{{ group.selectedNode.latency }} ms</span>
                </div>
              </div>
              <div v-else class="no-node-text">
                {{ t('nodes.group.no_node') }}
              </div>
            </div>

            <!-- Filter Rules inside Group -->
            <div class="group-filters-section">
              <div class="filter-title">{{ t('nodes.group.filters') }}:</div>
              <div class="filter-chips">
                <span
                  v-for="(f, idx) in group.filters"
                  :key="idx"
                  class="filter-chip mono"
                >
                  {{ f }}
                </span>
              </div>
            </div>

            <!-- Matched Nodes Count & List -->
            <div class="group-footer">
              <span class="node-count-label">
                {{ t('nodes.group.nodes_count') }}: <strong>{{ group.nodeCount }}</strong>
              </span>
              <div class="node-keys-preview mono truncate">
                {{ (group.nodeKeys || []).join(', ') || '-' }}
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- ================= Tab 3: Subscriptions ================= -->
      <section v-else-if="activeTab === 'subscriptions'" class="subs-container">
        <div class="subs-grid">
          <div
            v-for="sub in overview.subscriptions"
            :key="sub.tag"
            class="sub-card"
          >
            <div class="sub-header">
              <div class="sub-tag-box">
                <Rss :size="18" class="sub-icon" />
                <h3 class="sub-tag">{{ sub.tag }}</h3>
              </div>
              <span class="sub-status-pill">
                {{ t('nodes.sub.imported') }} {{ sub.nodeCount }}
              </span>
            </div>

            <div class="sub-url-box mono truncate" :title="sub.url">
              {{ sub.url }}
            </div>

            <div class="sub-actions">
              <span class="sub-hint">{{ t('nodes.sub.ready_hint') }}</span>
            </div>
          </div>
        </div>
      </section>
    </main>

    <!-- ================= Routing Simulator Modal ================= -->
    <div v-if="showSimulator" class="modal-backdrop" @click.self="showSimulator = false">
      <div class="simulator-modal">
        <div class="modal-header">
          <div class="modal-title">
            <Compass :size="18" class="sim-icon" />
            <h3>{{ t('nodes.sim.title') }}</h3>
          </div>
          <button class="close-btn" @click="showSimulator = false">
            <X :size="18" />
          </button>
        </div>

        <div class="modal-body">
          <!-- Presets -->
          <div class="presets-row">
            <span class="preset-label">{{ t('nodes.sim.presets') }}:</span>
            <button
              class="preset-btn"
              @click="applyPreset('www.google.com', '142.250.190.46', t('nodes.sim_preset_google'))"
            >
              Google
            </button>
            <button
              class="preset-btn"
              @click="applyPreset('github.com', '140.82.112.4', t('nodes.sim_preset_github'))"
            >
              GitHub
            </button>
            <button
              class="preset-btn"
              @click="applyPreset('netflix.com', '198.51.100.80', t('nodes.sim_preset_netflix'))"
            >
              Netflix
            </button>
            <button
              class="preset-btn"
              @click="applyPreset('chatgpt.com', '104.18.2.161', t('nodes.sim_preset_chatgpt'))"
            >
              OpenAI
            </button>
            <button
              class="preset-btn"
              @click="applyPreset('router.local', '192.168.1.1', t('nodes.sim_preset_lan'))"
            >
              192.168.1.1
            </button>
            <button
              class="preset-btn"
              @click="applyPreset('baidu.com', '110.242.68.66', t('nodes.sim_preset_baidu'))"
            >
              Baidu (.cn)
            </button>
          </div>

          <!-- Input Form -->
          <div class="sim-form-grid">
            <div class="form-field">
              <label>{{ t('nodes.sim.domain') }}</label>
              <input
                v-model="simDomain"
                type="text"
                placeholder="www.google.com"
                class="sim-input"
              />
            </div>

            <div class="form-field">
              <label>{{ t('nodes.sim.ip') }}</label>
              <input
                v-model="simIP"
                type="text"
                placeholder="142.250.190.46"
                class="sim-input mono"
              />
            </div>

            <div class="form-field">
              <label>{{ t('nodes.sim.port') }}</label>
              <input
                v-model="simPort"
                type="text"
                placeholder="443"
                class="sim-input mono"
              />
            </div>

            <div class="form-field">
              <label>{{ t('nodes.sim.proto') }}</label>
              <select v-model="simProto" class="sim-input">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </div>
          </div>

          <div class="sim-action-row">
            <button
              class="btn-run-sim"
              :disabled="simLoading"
              @click="runRoutingSimulator"
            >
              <Play :size="14" :class="{ spin: simLoading }" />
              <span>{{ simLoading ? t('nodes.sim.running') : t('nodes.sim.run') }}</span>
            </button>
          </div>

          <!-- Trace Result -->
          <div v-if="simResult" class="sim-result-box">
            <div class="result-summary-card">
              <div class="summary-top">
                <div class="res-target-badge" :class="simResult.targetType">
                  <span class="label">{{ t('nodes.sim.target') }}:</span>
                  <strong>{{ simResult.target }}</strong>
                  <span class="type-tag">({{ simResult.targetType }})</span>
                </div>

                <div class="hit-rule-box">
                  <span class="lbl">{{ t('nodes.sim.rule') }}:</span>
                  <code class="rule-code">{{ simResult.matchedRule }}</code>
                </div>
              </div>

              <!-- Selected Node Card if Group -->
              <div v-if="simResult.selectedNode" class="result-node-card">
                <span class="sel-tag">🚀 {{ t('nodes.sim.node') }}:</span>
                <div class="node-chip">
                  <span class="flag">{{ simResult.selectedNode.flag }}</span>
                  <strong>{{ simResult.selectedNode.name }}</strong>
                  <span class="proto-tag">{{ simResult.selectedNode.protocol }}</span>
                  <span class="latency-tag mono">{{ simResult.selectedNode.latency }} ms</span>
                  <span class="country-text">{{ simResult.selectedNode.country }}</span>
                </div>
              </div>
            </div>

            <!-- Trace Execution Steps -->
            <div class="trace-steps-container">
              <div class="trace-header">{{ t('nodes.sim.log') }}:</div>
              <div class="trace-console mono">
                <div
                  v-for="(step, idx) in simResult.traceSteps"
                  :key="idx"
                  class="step-line"
                  :class="{
                    hit: step.includes('命中！') || step.includes('出口路由选路') || step.includes('Hit!') || step.includes('Route selection'),
                    header: step.includes('=== 🚀')
                  }"
                >
                  {{ step }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.nodes-view {
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
.view-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-row h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.profile-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  color: #0284c7;
}

.live-dot {
  color: #10b981;
}

.subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-muted, #64748b);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
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
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #cbd5e1);
}
.action-btn.secondary:hover {
  background: var(--border-color, #1e293b);
  color: #0284c7;
}

.action-btn.icon-only {
  padding: 7px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  color: var(--text-muted, #94a3b8);
}
.action-btn.icon-only:hover {
  color: var(--text-main, #f8fafc);
}

.spin {
  animation: spin 1s linear infinite;
}
@keyframes spin {
  100% { transform: rotate(360deg); }
}

/* Stat Bar */
.stat-bar {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.stat-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: background-color 0.2s, border-color 0.2s;
}

.stat-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.stat-val {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main, #f8fafc);
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.stat-sub {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-muted, #64748b);
}

/* View Tabs */
.view-tabs {
  display: flex;
  gap: 6px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  padding-bottom: 2px;
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  color: var(--text-main, #f8fafc);
}

.tab-btn.active {
  color: #0284c7;
  border-bottom-color: #0284c7;
}

.pill-count {
  font-size: 10px;
  background: var(--bg-input, #1e293b);
  padding: 1px 6px;
  border-radius: 10px;
  color: var(--text-muted, #94a3b8);
}

/* Main Body */
.view-body {
  flex: 1;
}

/* Tab 1: Nodes Toolbar */
.filter-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  gap: 12px;
}

.search-box {
  position: relative;
  flex: 1;
  max-width: 380px;
}

.search-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-muted, #64748b);
}

.search-input {
  width: 100%;
  padding: 6px 30px 6px 32px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  font-size: 12px;
  color: var(--text-main, #f8fafc);
  outline: none;
}
.search-input:focus {
  border-color: #0284c7;
}

.clear-btn {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.select-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.toolbar-select {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #cbd5e1);
  padding: 5px 8px;
  font-size: 12px;
  outline: none;
  cursor: pointer;
}

/* Table Card */
.table-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  overflow: hidden;
  transition: background-color 0.2s, border-color 0.2s;
}

.nodes-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.nodes-table th {
  text-align: left;
  padding: 10px 14px;
  background: var(--bg-input, #0b1120);
  color: var(--text-muted, #64748b);
  border-bottom: 1px solid var(--border-color, #1e293b);
  font-weight: 500;
  white-space: nowrap;
}

.nodes-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #1e293b55);
  color: var(--text-main, #cbd5e1);
  vertical-align: middle;
}

.row-active {
  background: rgba(2, 132, 199, 0.05);
}

.active-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
}

.standby-pill {
  display: inline-flex;
  align-items: center;
  color: var(--text-muted, #64748b);
  font-size: 11px;
  padding: 2px 6px;
  white-space: nowrap;
}

.country-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.flag-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.node-title {
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.node-key {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.proto-badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  font-family: monospace;
  background: var(--bg-input, #1e293b);
  color: #0284c7;
}
.proto-badge.vless { color: #0284c7; }
.proto-badge.trojan { color: #10b981; }
.proto-badge.ss { color: #f59e0b; }
.proto-badge.vmess { color: #a855f7; }
.proto-badge.hysteria2 { color: #ec4899; }
.proto-badge.socks5 { color: #64748b; }

.addr-box {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}
.port-tag {
  color: var(--text-main, #cbd5e1);
}

.latency-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.latency-bar {
  height: 4px;
  border-radius: 2px;
  min-width: 12px;
}
.latency-bar.fast { background: #10b981; }
.latency-bar.medium { background: #d97706; }
.latency-bar.slow { background: #ef4444; }

.latency-val {
  font-size: 11px;
  font-weight: 600;
}
.latency-val.fast { color: #10b981; }
.latency-val.medium { color: #d97706; }
.latency-val.slow { color: #ef4444; }

.btn-ping {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: #0284c7;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-ping:hover:not(:disabled) {
  background: var(--border-color, #334155);
  color: #38bdf8;
}

.empty-row {
  text-align: center;
  padding: 30px;
  color: var(--text-muted, #64748b);
}

/* Tab 2: Policy Groups */
.groups-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
}

.group-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  transition: background-color 0.2s, border-color 0.2s;
}

.group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.group-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-icon {
  color: #a855f7;
}

.group-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.policy-pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 12px;
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
  font-family: monospace;
}

.group-selected-box {
  background: var(--bg-input, #0b1120);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  padding: 10px 12px;
}

.box-label {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  margin-bottom: 6px;
}

.selected-node-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sel-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.no-node-text {
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.filter-title {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  margin-bottom: 6px;
}

.filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.filter-chip {
  font-size: 11px;
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #cbd5e1);
  padding: 2px 6px;
  border-radius: 4px;
}

.group-footer {
  border-top: 1px solid var(--border-color, #1e293b);
  padding-top: 10px;
  font-size: 12px;
  color: var(--text-muted, #64748b);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.node-keys-preview {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

/* Tab 3: Subscriptions */
.subs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
}

.sub-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: background-color 0.2s, border-color 0.2s;
}

.sub-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sub-tag-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sub-icon {
  color: #10b981;
}

.sub-tag {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.sub-status-pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 12px;
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.sub-url-box {
  background: var(--bg-input, #0b1120);
  border: 1px solid var(--border-color, #1e293b);
  padding: 8px 10px;
  border-radius: 6px;
  font-size: 11px;
  color: #0284c7;
}

.sub-actions {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

/* Simulator Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.simulator-modal {
  width: 760px;
  max-width: 95%;
  max-height: 85vh;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3);
  color: var(--text-main, #f8fafc);
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.modal-title h3 {
  margin: 0;
  font-size: 16px;
  color: var(--text-main, #f8fafc);
}
.sim-icon {
  color: #0284c7;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
}
.close-btn:hover {
  color: var(--text-main, #f8fafc);
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.presets-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.preset-label {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.preset-btn {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 4px;
  cursor: pointer;
}
.preset-btn:hover {
  background: var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
}

.sim-form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  background: var(--bg-input, #0b1120);
  padding: 14px;
  border-radius: 8px;
  border: 1px solid var(--border-color, #1e293b);
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-field label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.sim-input {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  outline: none;
}
.sim-input:focus {
  border-color: #0284c7;
}

.sim-action-row {
  display: flex;
  justify-content: flex-end;
}

.btn-run-sim {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #0284c7;
  color: #fff;
  border: none;
  padding: 7px 16px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}
.btn-run-sim:hover:not(:disabled) {
  background: #0369a1;
}

.sim-result-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-summary-card {
  background: var(--bg-input, #0b1120);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.summary-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
}

.res-target-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
}
.res-target-badge.direct {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}
.res-target-badge.block {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}
.res-target-badge.group {
  background: rgba(2, 132, 199, 0.15);
  color: #38bdf8;
}

.hit-rule-box {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}
.rule-code {
  background: var(--bg-card, #0f172a);
  padding: 2px 8px;
  border-radius: 4px;
  color: #f59e0b;
}

.result-node-card {
  border-top: 1px solid var(--border-color, #1e293b);
  padding-top: 8px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.sel-tag {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.node-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.proto-tag {
  font-size: 10px;
  font-family: monospace;
  background: var(--bg-card, #0f172a);
  padding: 1px 5px;
  border-radius: 4px;
  color: #0284c7;
}

.latency-tag {
  color: #10b981;
  font-weight: 600;
}

.country-text {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.trace-steps-container {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.trace-header {
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.trace-console {
  background: #020617;
  border: 1px solid #1e293b;
  border-radius: 6px;
  padding: 12px;
  font-size: 11px;
  color: #94a3b8;
  max-height: 220px;
  overflow-y: auto;
  line-height: 1.6;
}

.step-line.hit {
  color: #34d399;
  font-weight: 600;
}

.step-line.header {
  color: #38bdf8;
  font-weight: 600;
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
