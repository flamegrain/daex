<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
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

interface CountryStat {
  name: string
  count: number
  flag: string
}

interface TrafficPoint {
  timestamp: number
  uploadBps: number
  downloadBps: number
}

const dashboardData = ref<any>(null)
const loading = ref(true)
const speedChartEl = ref<HTMLDivElement | null>(null)
const countryChartEl = ref<HTMLDivElement | null>(null)

let speedChart: echarts.ECharts | null = null
let countryChart: echarts.ECharts | null = null
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

function initCharts() {
  if (speedChartEl.value) {
    speedChart = echarts.init(speedChartEl.value)
  }
  if (countryChartEl.value) {
    countryChart = echarts.init(countryChartEl.value)
  }

  window.addEventListener('resize', handleResize)
}

function handleResize() {
  speedChart?.resize()
  countryChart?.resize()
}

function updateCharts(data: any) {
  if (!data) return

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
        backgroundColor: '#0f172a',
        borderColor: '#1e293b',
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        formatter: (params: any) => {
          let str = `<div style="font-size:11px;color:#94a3b8;margin-bottom:4px">${params[0].axisValue}</div>`
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
        textStyle: { color: '#94a3b8', fontSize: 12 },
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
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: { color: '#64748b', fontSize: 11 }
      },
      yAxis: {
        type: 'value',
        axisLine: { show: false },
        splitLine: { lineStyle: { color: '#1e293b' } },
        axisLabel: { color: '#64748b', fontSize: 11, formatter: '{value} MB/s' }
      },
      series: [
        {
          name: '下行速率 (MB/s)',
          type: 'line',
          smooth: true,
          showSymbol: false,
          itemStyle: { color: '#38bdf8' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(56, 189, 248, 0.35)' },
              { offset: 1, color: 'rgba(56, 189, 248, 0.0)' }
            ])
          },
          data: downData
        },
        {
          name: '上行速率 (MB/s)',
          type: 'line',
          smooth: true,
          showSymbol: false,
          itemStyle: { color: '#a855f7' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(168, 85, 247, 0.3)' },
              { offset: 1, color: 'rgba(168, 85, 247, 0.0)' }
            ])
          },
          data: upData
        }
      ]
    }
    speedChart.setOption(speedOption)
  }

  // 2. Donut/Pie chart for Country Distribution
  if (countryChart && data.countries) {
    const pieData = data.countries.map((c: CountryStat) => ({
      name: `${c.flag} ${c.name}`,
      value: c.count
    }))

    const countryOption: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        backgroundColor: '#0f172a',
        borderColor: '#1e293b',
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        formatter: '{b}: <b>{c} 个节点</b> ({d}%)'
      },
      legend: {
        orient: 'vertical',
        right: '5%',
        top: 'center',
        textStyle: { color: '#94a3b8', fontSize: 11 }
      },
      series: [
        {
          name: '国家与地区分布',
          type: 'pie',
          radius: ['45%', '75%'],
          center: ['35%', '50%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 6,
            borderColor: '#0b1120',
            borderWidth: 2
          },
          label: { show: false },
          emphasis: {
            label: {
              show: true,
              fontSize: 12,
              fontWeight: 'bold',
              color: '#f8fafc'
            }
          },
          data: pieData
        }
      ]
    }
    countryChart.setOption(countryOption)
  }
}

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
  countryChart?.dispose()
})
</script>

<template>
  <div class="home-dashboard">
    <!-- Header Summary -->
    <header class="dash-header">
      <div>
        <h2 class="title">网关运行大盘</h2>
        <p class="subtitle">基于 eBPF 的轻量路由核心，安全沙箱模式 (Mock 数据流)</p>
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

    <!-- Charts Row -->
    <section class="charts-grid">
      <!-- Speed Trend Chart -->
      <div class="chart-card speed-card">
        <div class="card-head">
          <div class="head-title">
            <Activity class="h-icon" :size="16" />
            <span>实时数据传输速率 (Throughput Trend)</span>
          </div>
          <span class="tag live">● 实时推流 (2s)</span>
        </div>
        <div class="chart-container" ref="speedChartEl"></div>
      </div>

      <!-- Country Breakdown Chart -->
      <div class="chart-card country-card">
        <div class="card-head">
          <div class="head-title">
            <Globe2 class="h-icon" :size="16" />
            <span>节点国家与地区分布 (Geo Distribution)</span>
          </div>
        </div>
        <div class="chart-container" ref="countryChartEl"></div>
      </div>
    </section>

    <!-- Active Nodes Table -->
    <section class="nodes-section">
      <div class="section-head">
        <div class="head-title">
          <Layers class="h-icon" :size="16" />
          <span>生效与可用节点概览 (Active Nodes Pool)</span>
        </div>
        <span class="badge-mock">沙箱假数据安全展示</span>
      </div>

      <div class="table-wrapper">
        <table class="nodes-table">
          <thead>
            <tr>
              <th style="width: 90px">状态</th>
              <th style="width: 110px">国家/地区</th>
              <th style="width: 25%">节点名称 (Alias)</th>
              <th style="width: 80px">协议</th>
              <th style="width: 30%">目标地址 (Address)</th>
              <th style="width: 70px">端口</th>
              <th style="width: 90px">延迟 (Ping)</th>
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
  padding: 24px;
  background: #090d16;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.dash-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.dash-header .title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #f8fafc;
}

.dash-header .subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #64748b;
}

.refresh-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #1e293b;
  border: 1px solid #334155;
  color: #94a3b8;
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.refresh-pill:hover {
  background: #334155;
  color: #fff;
}

/* Metrics Grid */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.metric-card {
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.metric-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.m-label {
  font-size: 12px;
  color: #64748b;
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
  background: #0284c722;
  color: #38bdf8;
}
.icon-circle.up {
  background: #7e22ce22;
  color: #c084fc;
}
.icon-circle.status {
  background: #064e3b33;
  color: #34d399;
}
.icon-circle.node {
  background: #f59e0b22;
  color: #fbbf24;
}

.metric-value {
  font-size: 22px;
  font-weight: 700;
  color: #f8fafc;
  letter-spacing: -0.5px;
}

.metric-value.active-text {
  color: #34d399;
  font-size: 18px;
}

.metric-value .unit {
  font-size: 13px;
  font-weight: 400;
  color: #64748b;
}

.metric-sub {
  font-size: 11px;
  color: #64748b;
  display: flex;
  align-items: center;
  gap: 6px;
}
.metric-sub .sep {
  color: #334155;
}

/* Charts Grid */
.charts-grid {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 16px;
}

.chart-card {
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  height: 310px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.head-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #e2e8f0;
}

.h-icon {
  color: #38bdf8;
}

.tag.live {
  font-size: 11px;
  color: #34d399;
  background: #064e3b33;
  padding: 2px 8px;
  border-radius: 12px;
}

.chart-container {
  flex: 1;
  width: 100%;
  height: 100%;
}

/* Nodes Table Section */
.nodes-section {
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.badge-mock {
  font-size: 11px;
  background: #1e293b;
  border: 1px solid #334155;
  color: #94a3b8;
  padding: 2px 8px;
  border-radius: 4px;
}

.home-dashboard {
  height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 24px;
  background: #090d16;
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 100%;
  box-sizing: border-box;
}

.table-wrapper {
  width: 100%;
  overflow-x: hidden;
}

.nodes-table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: 12px;
}

.nodes-table th {
  text-align: left;
  padding: 8px 10px;
  color: #64748b;
  border-bottom: 1px solid #1e293b;
  font-weight: 500;
  white-space: nowrap;
}

.nodes-table td {
  padding: 10px;
  border-bottom: 1px solid #1e293b55;
  color: #cbd5e1;
  vertical-align: middle;
}

.truncate-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.row-active {
  background: rgba(56, 189, 248, 0.03);
}

.active-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #064e3b44;
  color: #34d399;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
}

.standby-pill {
  display: inline-flex;
  align-items: center;
  color: #64748b;
  font-size: 11px;
  padding: 2px 6px;
  white-space: nowrap;
}

.country-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
}

.country-cell .flag {
  font-size: 16px;
  flex-shrink: 0;
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.key-tag {
  font-family: monospace;
  font-size: 10px;
  color: #64748b;
}

.proto-tag {
  background: #1e293b;
  color: #38bdf8;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
  font-size: 10px;
  font-family: monospace;
  display: inline-block;
  white-space: nowrap;
}

.addr-cell {
  overflow: hidden;
  color: #94a3b8;
}

.latency {
  font-family: monospace;
  font-weight: 600;
  white-space: nowrap;
}
.latency.fast {
  color: #34d399;
}
.latency.medium {
  color: #fbbf24;
}
.latency.slow {
  color: #f87171;
}
</style>
