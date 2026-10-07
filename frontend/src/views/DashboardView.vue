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
  RefreshCw,
  Gauge,
  PlayCircle
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

interface ConnectivityItem {
  id: string
  name: string
  category: string
  url: string
  expectedOut: string
  status: 'ok' | 'error' | 'pending'
  latency: number
  httpCode: number
  message: string
  lastTested?: string
}

const dashboardData = ref<any>(null)
const loading = ref(true)
const speedChartEl = ref<HTMLDivElement | null>(null)

let speedChart: echarts.ECharts | null = null
let timer: any = null

const connectivityList = ref<ConnectivityItem[]>([
  {
    id: 'aliyun',
    name: '阿里云访问检测',
    category: '国内直连',
    url: 'https://www.aliyun.com',
    expectedOut: '国内直连',
    status: 'ok',
    latency: 18,
    httpCode: 200,
    message: '国内高速骨干网直连畅通'
  },
  {
    id: 'google',
    name: 'Google 访问检测',
    category: '国际搜索',
    url: 'https://www.google.com/generate_204',
    expectedOut: '海外代理',
    status: 'ok',
    latency: 64,
    httpCode: 204,
    message: '海外代理链路畅通'
  },
  {
    id: 'github',
    name: 'GitHub 访问检测',
    category: '开发者服务',
    url: 'https://github.com',
    expectedOut: '海外代理',
    status: 'ok',
    latency: 82,
    httpCode: 200,
    message: 'Git 代码托管与网页访问正常'
  },
  {
    id: 'youtube',
    name: 'YouTube 访问检测',
    category: '流媒体服务',
    url: 'https://www.youtube.com/generate_204',
    expectedOut: '海外代理',
    status: 'ok',
    latency: 71,
    httpCode: 204,
    message: '流媒体视频 CDN 连通良好'
  },
  {
    id: 'openai',
    name: 'OpenAI 访问检测',
    category: 'AI 大模型',
    url: 'https://api.openai.com/v1/models',
    expectedOut: '原生解锁',
    status: 'ok',
    latency: 108,
    httpCode: 200,
    message: 'API 接口正常，原生节点已解锁'
  }
])

const testingTargets = ref<Record<string, boolean>>({})
const testingAll = ref(false)

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

async function fetchConnectivity() {
  try {
    const res = await fetch('/api/diagnostics/connectivity')
    if (res.ok) {
      const data = await res.json()
      if (data.targets && data.targets.length > 0) {
        connectivityList.value = data.targets
      }
    }
  } catch (e) {
    console.error('Fetch connectivity error:', e)
  }
}

async function testTarget(targetId: string) {
  testingTargets.value[targetId] = true
  try {
    const res = await fetch('/api/diagnostics/connectivity', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target: targetId })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.results && data.results.length > 0) {
        const item = data.results[0]
        const idx = connectivityList.value.findIndex(t => t.id === item.id)
        if (idx >= 0) {
          connectivityList.value[idx] = item
        }
      }
    }
  } catch (e) {
    console.error('Test target error:', e)
  } finally {
    testingTargets.value[targetId] = false
  }
}

async function testAllTargets() {
  testingAll.value = true
  connectivityList.value.forEach(t => {
    testingTargets.value[t.id] = true
  })
  try {
    const res = await fetch('/api/diagnostics/connectivity', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target: 'all' })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.results && data.results.length > 0) {
        connectivityList.value = data.results
      }
    }
  } catch (e) {
    console.error('Test all targets error:', e)
  } finally {
    testingAll.value = false
    connectivityList.value.forEach(t => {
      testingTargets.value[t.id] = false
    })
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

  // Line chart for Traffic Speed
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
    fetchConnectivity()
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

    <!-- Charts & Connectivity Quick Check Row -->
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

      <!-- Connectivity Quick Check Panel (Right) -->
      <div class="chart-card connectivity-card">
        <div class="card-head">
          <div class="head-title">
            <Gauge class="h-icon" :size="16" />
            <span>连通性快速检测面板</span>
          </div>
          <button
            class="btn-test-all"
            :disabled="testingAll"
            @click="testAllTargets"
            title="一键测试所有常用服务连通性与分流状态"
          >
            <Zap :size="12" :class="{ spin: testingAll }" />
            <span>{{ testingAll ? '正在全量检测...' : '一键全部检测' }}</span>
          </button>
        </div>

        <div class="connectivity-list">
          <div
            v-for="item in connectivityList"
            :key="item.id"
            class="connectivity-item"
            :class="{ testing: testingTargets[item.id] }"
          >
            <!-- Platform Brand Icon -->
            <div class="brand-icon-box" :class="item.id">
              <!-- Aliyun -->
              <svg v-if="item.id === 'aliyun'" viewBox="0 0 24 24" class="svg-icon" fill="currentColor">
                <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM19 18H6c-2.21 0-4-1.79-4-4 0-2.05 1.53-3.76 3.56-3.97l1.07-.11.5-.95C8.08 7.14 9.94 6 12 6c2.62 0 4.88 1.86 5.39 4.43l.3 1.5 1.53.11c1.56.1 2.78 1.41 2.78 2.96 0 1.65-1.35 3-3 3z"/>
              </svg>

              <!-- Google -->
              <svg v-else-if="item.id === 'google'" viewBox="0 0 24 24" class="svg-icon">
                <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.66-5.17 3.66-9.17z"/>
                <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.33 24 12 24z"/>
                <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.97 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
              </svg>

              <!-- GitHub -->
              <svg v-else-if="item.id === 'github'" viewBox="0 0 24 24" class="svg-icon" fill="currentColor">
                <path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/>
              </svg>

              <!-- YouTube -->
              <svg v-else-if="item.id === 'youtube'" viewBox="0 0 24 24" class="svg-icon" fill="currentColor">
                <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
              </svg>

              <!-- OpenAI -->
              <svg v-else-if="item.id === 'openai'" viewBox="0 0 24 24" class="svg-icon" fill="currentColor">
                <path d="M22.282 9.821a5.985 5.985 0 0 0-.516-4.91 6.046 6.046 0 0 0-6.51-2.9A6.065 6.065 0 0 0 4.981 4.18a5.985 5.985 0 0 0-3.998 2.9 6.046 6.046 0 0 0 .743 7.097 5.98 5.98 0 0 0 .51 4.911 6.051 6.051 0 0 0 6.515 2.9A5.985 5.985 0 0 0 13.26 24a6.056 6.056 0 0 0 5.772-4.206 5.99 5.99 0 0 0 3.997-2.9 6.056 6.056 0 0 0-.747-7.073zM13.26 22.43a4.476 4.476 0 0 1-2.876-1.04l.141-.081 4.779-2.758a.795.795 0 0 0 .392-.681v-6.737l2.02 1.168a.071.071 0 0 1 .038.052v5.583a4.504 4.504 0 0 1-4.494 4.494zM3.6 18.304a4.47 4.47 0 0 1-.535-3.014l.142.085 4.783 2.759a.771.771 0 0 0 .78 0l5.843-3.369v2.332a.08.08 0 0 1-.033.064l-4.84 2.793a4.499 4.499 0 0 1-6.14-1.65zM2.34 8.653a4.47 4.47 0 0 1 2.339-1.974V12.2a.78.78 0 0 0 .392.682l5.836 3.368-2.02 1.168a.08.08 0 0 1-.071.007L3.98 14.63A4.504 4.504 0 0 1 2.34 8.653zm16.597 3.855l-5.833-3.37L15.12 7.97a.08.08 0 0 1 .071-.007l4.836 2.795a4.5 4.5 0 0 1-.61 8.103v-5.672a.79.79 0 0 0-.48-.688zm2.01-4.423l-.141-.085-4.773-2.782a.776.776 0 0 0-.785 0L9.409 8.587V6.255a.08.08 0 0 1 .03-.064l4.84-2.793a4.499 4.499 0 0 1 6.668 4.258zm-9.742 5.093l2.8-1.616 2.805 1.616v3.232l-2.805 1.616-2.8-1.616z"/>
              </svg>
            </div>

            <!-- Target Detail Info -->
            <div class="target-meta">
              <div class="target-title-row">
                <span class="target-name">{{ item.name }}</span>
                <span
                  class="route-pill"
                  :class="{
                    direct: item.expectedOut.includes('直连'),
                    proxy: item.expectedOut.includes('代理'),
                    ai: item.expectedOut.includes('解锁')
                  }"
                >
                  {{ item.expectedOut }}
                </span>
              </div>
              <div class="target-sub-row">
                <span class="target-url mono">{{ item.url.replace('https://', '').split('/')[0] }}</span>
                <span class="sep">·</span>
                <span class="target-desc">{{ item.message }}</span>
              </div>
            </div>

            <!-- Status Indicator & Single Test Button -->
            <div class="target-actions">
              <!-- Latency Pill -->
              <span
                class="latency-badge"
                :class="{
                  fast: item.latency < 50,
                  medium: item.latency >= 50 && item.latency < 100,
                  slow: item.latency >= 100
                }"
              >
                <span class="dot"></span>
                {{ item.latency }} ms
              </span>

              <!-- Test Button -->
              <button
                class="btn-test-single"
                :disabled="testingTargets[item.id] || testingAll"
                @click="testTarget(item.id)"
                :title="`重新测试 ${item.name}`"
              >
                <RefreshCw
                  :size="12"
                  :class="{ spin: testingTargets[item.id] }"
                />
                <span>{{ testingTargets[item.id] ? '检测中' : '检测' }}</span>
              </button>
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
  grid-template-columns: 1.15fr 0.85fr;
  gap: 16px;
}

@media (max-width: 960px) {
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
  height: 250px;
  width: 100%;
}

/* Right Side: Connectivity Quick Check Panel */
.btn-test-all {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 4px;
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.25);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-test-all:hover:not(:disabled) {
  background: #0284c7;
  color: #fff;
}

.connectivity-list {
  display: flex;
  flex-direction: column;
  gap: 7px;
  height: 250px;
  overflow-y: auto;
  padding-right: 2px;
}

.connectivity-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  transition: all 0.15s ease;
}

.connectivity-item:hover {
  border-color: #0284c7;
  background: var(--bg-input, #1e293b);
}

.connectivity-item.testing {
  opacity: 0.75;
}

/* Brand Icons */
.brand-icon-box {
  width: 30px;
  height: 30px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.svg-icon {
  width: 16px;
  height: 16px;
}

.brand-icon-box.aliyun {
  background: rgba(255, 106, 0, 0.12);
  color: #ff6a00;
}

.brand-icon-box.google {
  background: rgba(66, 133, 244, 0.12);
}

.brand-icon-box.github {
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-main, #f8fafc);
}

.brand-icon-box.youtube {
  background: rgba(255, 0, 0, 0.12);
  color: #ff0000;
}

.brand-icon-box.openai {
  background: rgba(16, 163, 127, 0.12);
  color: #10a37f;
}

/* Target Details */
.target-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.target-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.target-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  white-space: nowrap;
}

.route-pill {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 10px;
  font-weight: 500;
  white-space: nowrap;
}

.route-pill.direct {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.route-pill.proxy {
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
}

.route-pill.ai {
  background: rgba(168, 85, 247, 0.12);
  color: #a855f7;
}

.target-sub-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted, #64748b);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.target-url {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}

.target-desc {
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Actions & Latency */
.target-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.latency-badge {
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

.latency-badge .dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}

.latency-badge.fast {
  color: #10b981;
}

.latency-badge.fast .dot {
  background: #10b981;
}

.latency-badge.medium {
  color: #f59e0b;
}

.latency-badge.medium .dot {
  background: #f59e0b;
}

.latency-badge.slow {
  color: #ef4444;
}

.latency-badge.slow .dot {
  background: #ef4444;
}

.btn-test-single {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 7px;
  font-size: 11px;
  border-radius: 4px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-test-single:hover:not(:disabled) {
  background: #0284c7;
  border-color: #0284c7;
  color: #ffffff;
}

.btn-test-single:disabled {
  opacity: 0.6;
  cursor: not-allowed;
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
