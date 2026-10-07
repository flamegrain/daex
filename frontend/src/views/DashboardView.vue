<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import {
  Activity,
  ArrowUpRight,
  ArrowDownRight,
  Server,
  Globe2,
  CheckCircle2,
  Cpu,
  Clock,
  Layers,
  Zap,
  RefreshCw
} from 'lucide-vue-next'
import * as echarts from 'echarts'
import { theme } from '../composables/useSettings'

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
}

interface TrafficPoint {
  timestamp: number
  uploadBps: number
  downloadBps: number
}

const dashboardData = ref<any>(null)
const loading = ref(true)
const pingingNodes = ref(false)
const speedChartEl = ref<HTMLDivElement | null>(null)

let speedChart: echarts.ECharts | null = null
let timer: any = null

function formatBps(bytesPerSec: number) {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s'
  const k = 1024
  const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(k))
  return parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatBytes(bytes: number) {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchDashboard() {
  try {
    const res = await fetch('/api/dashboard')
    if (res.ok) {
      const data = await res.json()
      dashboardData.value = data
      updateCharts(data)
    }
  } catch (e) {
    console.error('Fetch dashboard error:', e)
  } finally {
    loading.value = false
  }
}

async function handleQuickPingAll() {
  pingingNodes.value = true
  try {
    const res = await fetch('/api/nodes/ping', { method: 'POST' })
    if (res.ok) {
      await fetchDashboard()
    }
  } catch (e) {
    console.error('Quick ping error:', e)
  } finally {
    pingingNodes.value = false
  }
}

function initCharts() {
  if (speedChartEl.value) {
    speedChart = echarts.init(speedChartEl.value)
  }
  window.addEventListener('resize', handleResize)
}

function handleResize() {
  speedChart?.resize()
}

function updateCharts(data: any) {
  if (!data) return
  const isLight = theme.value === 'light'

  // 1. Line chart for Traffic Speed
  if (speedChart && data.history) {
    const times = data.history.map((h: TrafficPoint) =>
      new Date(h.timestamp * 1000).toLocaleTimeString([], { hour12: false })
    )
    const downData = data.history.map((h: TrafficPoint) =>
      parseFloat((h.downloadBps / 1024 / 1024).toFixed(2))
    )
    const upData = data.history.map((h: TrafficPoint) =>
      parseFloat((h.uploadBps / 1024 / 1024).toFixed(2))
    )

    const speedOption: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: isLight ? '#ffffff' : '#0f172a',
        borderColor: isLight ? '#e2e8f0' : '#1e293b',
        textStyle: { color: isLight ? '#0f172a' : '#e2e8f0', fontSize: 12 },
        formatter: (params: any) => {
          let str = `<div style="font-size:11px;color:${isLight ? '#64748b' : '#94a3b8'};margin-bottom:4px">${params[0].axisValue}</div>`
          params.forEach((p: any) => {
            str += `<div style="display:flex;align-items:center;gap:6px;margin:2px 0;">
              <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color}"></span>
              <span>${p.seriesName}: <b>${p.value} MB/s</b></span>
            </div>`
          })
          return str
        }
      },
      legend: {
        data: ['下行速率 (MB/s)', '上行速率 (MB/s)'],
        textStyle: { color: isLight ? '#475569' : '#94a3b8', fontSize: 12 },
        top: 0,
        right: 10
      },
      grid: {
        left: '2%',
        right: '4%',
        bottom: '3%',
        top: '18%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: times,
        axisLine: { lineStyle: { color: isLight ? '#e2e8f0' : '#334155' } },
        axisLabel: { color: isLight ? '#64748b' : '#64748b', fontSize: 11 }
      },
      yAxis: {
        type: 'value',
        axisLine: { show: false },
        splitLine: { lineStyle: { color: isLight ? '#f1f5f9' : '#1e293b' } },
        axisLabel: { color: isLight ? '#64748b' : '#64748b', fontSize: 11, formatter: '{value} MB/s' }
      },
      series: [
        {
          name: '下行速率 (MB/s)',
          type: 'line',
          smooth: true,
          showSymbol: false,
          data: downData,
          lineStyle: { width: 2, color: '#38bdf8' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(56, 189, 248, 0.35)' },
              { offset: 1, color: 'rgba(56, 189, 248, 0.00)' }
            ])
          }
        },
        {
          name: '上行速率 (MB/s)',
          type: 'line',
          smooth: true,
          showSymbol: false,
          data: upData,
          lineStyle: { width: 2, color: '#10b981' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(16, 185, 129, 0.35)' },
              { offset: 1, color: 'rgba(16, 185, 129, 0.00)' }
            ])
          }
        }
      ]
    }
    speedChart.setOption(speedOption)
  }
}

watch(theme, () => {
  if (dashboardData.value) {
    updateCharts(dashboardData.value)
  }
})

onMounted(() => {
  nextTick(() => {
    initCharts()
    fetchDashboard()
    timer = setInterval(fetchDashboard, 3000)
  })
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  window.removeEventListener('resize', handleResize)
  speedChart?.dispose()
})
</script>

<template>
  <div class="home-dashboard">
    <!-- Header Summary -->
    <header class="dash-header">
      <div>
        <h2 class="title">网关运行首页</h2>
        <p class="subtitle">基于内核态 eBPF 的轻量路由核心与流量监控面板</p>
      </div>
      <button class="refresh-pill" @click="fetchDashboard">
        <RefreshCw :size="13" />
        <span>刷新仪表盘</span>
      </button>
    </header>

    <!-- Top Key Metrics Cards -->
    <section class="metrics-grid">
      <!-- 1. Download Speed -->
      <div class="metric-card">
        <div class="metric-top">
          <span class="m-label">当前下载速率</span>
          <div class="icon-circle down">
            <ArrowDownRight :size="16" />
          </div>
        </div>
        <div class="metric-value">
          {{ formatBps(dashboardData?.current?.downloadBps) }}
        </div>
        <div class="metric-sub">
          总接收: {{ formatBytes(dashboardData?.current?.totalRx) }}
        </div>
      </div>

      <!-- 2. Upload Speed -->
      <div class="metric-card">
        <div class="metric-top">
          <span class="m-label">当前上传速率</span>
          <div class="icon-circle up">
            <ArrowUpRight :size="16" />
          </div>
        </div>
        <div class="metric-value">
          {{ formatBps(dashboardData?.current?.uploadBps) }}
        </div>
        <div class="metric-sub">
          总发送: {{ formatBytes(dashboardData?.current?.totalTx) }}
        </div>
      </div>

      <!-- 3. Active Nodes & Gateway -->
      <div class="metric-card">
        <div class="metric-top">
          <span class="m-label">核心工作状态</span>
          <div class="icon-circle status">
            <Activity :size="16" />
          </div>
        </div>
        <div class="metric-value active-text">
          {{ dashboardData?.dae?.ActiveState === 'active' ? '正常运行中' : '未运行' }}
        </div>
        <div class="metric-sub">
          <span>PID: {{ dashboardData?.dae?.MainPID || '-' }}</span>
          <span class="sep">|</span>
          <span>{{ dashboardData?.dae?.version?.split(' ')[2] || 'v2.1.1' }}</span>
        </div>
      </div>

      <!-- 4. Node Counts -->
      <div class="metric-card">
        <div class="metric-top">
          <span class="m-label">就绪代理节点</span>
          <div class="icon-circle node">
            <Server :size="16" />
          </div>
        </div>
        <div class="metric-value">
          {{ dashboardData?.nodes?.length || 0 }} <span class="unit">个</span>
        </div>
        <div class="metric-sub">
          生效中: <b>2</b> 个分流出口
        </div>
      </div>
    </section>

    <!-- Charts & Nodes Summary Row -->
    <section class="charts-grid">
      <!-- Speed Trend Chart (Left) -->
      <div class="chart-card speed-card">
        <div class="card-head">
          <div class="head-title">
            <Activity class="h-icon" :size="16" />
            <span>实时数据传输速率</span>
          </div>
          <span class="tag live">● 实时推流</span>
        </div>
        <div class="chart-container" ref="speedChartEl"></div>
      </div>

      <!-- Node Summary, Country and Latency Results (Right) -->
      <div class="chart-card node-summary-card">
        <div class="card-head">
          <div class="head-title">
            <Server class="h-icon" :size="16" />
            <span>节点列表与测速状态</span>
          </div>
          <button
            class="btn-ping-quick"
            :disabled="pingingNodes"
            @click="handleQuickPingAll"
            title="一键测速全部可用节点"
          >
            <Zap :size="12" :class="{ spin: pingingNodes }" />
            <span>{{ pingingNodes ? '测速中...' : '快速测速' }}</span>
          </button>
        </div>

        <div class="node-compact-list">
          <div
            v-for="node in dashboardData?.nodes"
            :key="node.key"
            class="compact-node-item"
            :class="{ active: node.active }"
          >
            <!-- Country & Flag -->
            <div class="node-item-country">
              <span class="flag-icon">{{ node.flag }}</span>
              <span class="country-name">{{ node.country }}</span>
            </div>

            <!-- Node Name & Address -->
            <div class="node-item-info">
              <div class="node-item-title">
                <span class="node-name-text" :title="node.name">{{ node.name }}</span>
                <span class="proto-badge">{{ node.protocol }}</span>
              </div>
              <span class="node-addr-text mono" :title="`${node.address}:${node.port}`">
                {{ node.address }}:{{ node.port }}
              </span>
            </div>

            <!-- Ping Latency & State -->
            <div class="node-item-status">
              <span
                class="latency-pill"
                :class="{
                  fast: node.latency < 50,
                  medium: node.latency >= 50 && node.latency < 100,
                  slow: node.latency >= 100
                }"
              >
                <span class="latency-dot"></span>
                {{ node.latency }} ms
              </span>
              <span v-if="node.active" class="active-tag">生效中</span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Active Nodes Full Table -->
    <section class="nodes-section">
      <div class="section-head">
        <div class="head-title">
          <Layers class="h-icon" :size="16" />
          <span>生效与可用节点详细列表</span>
        </div>
        <span class="badge-mock">沙箱假数据安全展示</span>
      </div>

      <div class="table-wrapper">
        <table class="nodes-table">
          <thead>
            <tr>
              <th style="width: 90px">状态</th>
              <th style="width: 110px">国家/地区</th>
              <th style="width: 25%">节点名称</th>
              <th style="width: 80px">协议</th>
              <th style="width: 30%">目标地址</th>
              <th style="width: 70px">端口</th>
              <th style="width: 90px">测速延迟</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="node in dashboardData?.nodes" :key="node.key" :class="{ 'row-active': node.active }">
              <td>
                <span v-if="node.active" class="active-pill">
                  <CheckCircle2 :size="12" /> 生效中
                </span>
                <span v-else class="standby-pill">就绪</span>
              </td>
              <td class="country-cell">
                <span class="flag">{{ node.flag }}</span>
                <span class="truncate-text" :title="node.country">{{ node.country }}</span>
              </td>
              <td class="name-cell">
                <b class="truncate-text" :title="node.name">{{ node.name }}</b>
                <span class="key-tag truncate-text" :title="node.key">{{ node.key }}</span>
              </td>
              <td>
                <span class="proto-tag">{{ node.protocol }}</span>
              </td>
              <td class="addr-cell">
                <span class="truncate-text mono" :title="node.address">{{ node.address }}</span>
              </td>
              <td class="mono">{{ node.port }}</td>
              <td>
                <span class="latency" :class="{
                  fast: node.latency < 50,
                  medium: node.latency >= 50 && node.latency < 100,
                  slow: node.latency >= 100
                }">
                  {{ node.latency }} ms
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home-dashboard {
  height: 100%;
  overflow-y: auto;
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Header */
.dash-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title {
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

.refresh-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 20px;
  color: var(--text-muted, #94a3b8);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.refresh-pill:hover {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #f8fafc);
  border-color: #0284c7;
}

/* Top Metrics Cards */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 16px;
}

.metric-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.metric-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.m-label {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
}

.icon-circle {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-circle.down {
  background: rgba(56, 189, 248, 0.12);
  color: #38bdf8;
}

.icon-circle.up {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.icon-circle.status {
  background: rgba(168, 85, 247, 0.12);
  color: #a855f7;
}

.icon-circle.node {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
}

.metric-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main, #f8fafc);
}

.metric-value.active-text {
  color: #10b981;
}

.metric-value .unit {
  font-size: 13px;
  font-weight: 400;
  color: var(--text-muted, #94a3b8);
}

.metric-sub {
  font-size: 12px;
  color: var(--text-muted, #64748b);
  display: flex;
  align-items: center;
  gap: 6px;
}

.metric-sub .sep {
  opacity: 0.4;
}

/* Charts Grid */
.charts-grid {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 16px;
}

@media (max-width: 900px) {
  .charts-grid {
    grid-template-columns: 1fr;
  }
}

.chart-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.head-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.h-icon {
  color: #0284c7;
}

.tag.live {
  font-size: 11px;
  color: #10b981;
  background: rgba(16, 185, 129, 0.1);
  padding: 2px 8px;
  border-radius: 10px;
}

.chart-container {
  height: 240px;
  width: 100%;
}

/* Right Side: Node Summary Compact List */
.btn-ping-quick {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 11px;
  border-radius: 4px;
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.25);
  cursor: pointer;
  transition: all 0.15s;
}

.btn-ping-quick:hover:not(:disabled) {
  background: #0284c7;
  color: #fff;
}

.node-compact-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 240px;
  overflow-y: auto;
  padding-right: 4px;
}

.compact-node-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  transition: all 0.15s;
}

.compact-node-item:hover {
  border-color: #0284c7;
  background: var(--bg-input, #1e293b);
}

.compact-node-item.active {
  border-color: rgba(16, 185, 129, 0.35);
  background: rgba(16, 185, 129, 0.04);
}

.node-item-country {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 50px;
  gap: 2px;
}

.flag-icon {
  font-size: 18px;
}

.country-name {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  white-space: nowrap;
}

.node-item-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.node-item-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.node-name-text {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.proto-badge {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  text-transform: uppercase;
}

.node-addr-text {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-item-status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.latency-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-family: monospace;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 10px;
  background: var(--bg-card, #0f172a);
}

.latency-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}

.latency-pill.fast {
  color: #10b981;
}

.latency-pill.fast .latency-dot {
  background: #10b981;
}

.latency-pill.medium {
  color: #f59e0b;
}

.latency-pill.medium .latency-dot {
  background: #f59e0b;
}

.latency-pill.slow {
  color: #ef4444;
}

.latency-pill.slow .latency-dot {
  background: #ef4444;
}

.active-tag {
  font-size: 10px;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 500;
}

/* Bottom Nodes Full Table */
.nodes-section {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 16px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.badge-mock {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  background: var(--bg-input, #090d16);
  padding: 2px 8px;
  border-radius: 4px;
}

.table-wrapper {
  overflow-x: auto;
}

.nodes-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.nodes-table th {
  text-align: left;
  padding: 10px 12px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.nodes-table td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #f8fafc);
}

.nodes-table tr.row-active {
  background: rgba(16, 185, 129, 0.03);
}

.active-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  padding: 2px 8px;
  border-radius: 12px;
  font-weight: 500;
}

.standby-pill {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  background: var(--bg-input, #090d16);
  padding: 2px 8px;
  border-radius: 12px;
}

.country-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.flag {
  font-size: 16px;
}

.name-cell b {
  display: block;
}

.key-tag {
  display: block;
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.proto-tag {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.latency {
  font-family: monospace;
  font-size: 12px;
  font-weight: 600;
}

.latency.fast {
  color: #10b981;
}

.latency.medium {
  color: #f59e0b;
}

.latency.slow {
  color: #ef4444;
}

.mono {
  font-family: monospace;
}

.truncate-text {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
