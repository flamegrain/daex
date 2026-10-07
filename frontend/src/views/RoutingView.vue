<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  GitFork,
  Plus,
  Edit2,
  Trash2,
  Copy,
  GripVertical,
  ShieldCheck,
  ShieldAlert,
  AlertTriangle,
  Lightbulb,
  Check,
  Save,
  RotateCcw,
  BookOpen,
  Sparkles,
  Wand2,
  Code,
  Files,
  Info,
  ChevronDown,
  ChevronUp,
  CheckCircle2,
  RefreshCw,
  Search,
  CheckSquare,
  HelpCircle,
  ExternalLink,
  Layers
} from 'lucide-vue-next'
import {
  analyzeRoutingConflicts,
  type ConfigRouteRule,
  type RoutingHealthReport,
  type ConflictSeverity
} from '../utils/routingAnalyzer'
import {
  ROUTING_SYNTAX_GUIDE,
  type SyntaxDocSection
} from '../utils/routingPresets'

interface ConfigProfile {
  name: string
  path: string
  isActive: boolean
  size: number
  modTime: string
}

interface RoutingTemplateItem {
  id: string
  name: string
  category: string
  condition: string
  recommendedTarget: string
  description: string
  tip?: string
}

interface RoutingTemplateResponse {
  version: string
  updatedAt: string
  source: string
  templates: RoutingTemplateItem[]
}

interface DaeConfigModel {
  globals: Record<string, string>
  subscriptions: any[]
  nodes: any[]
  groups: Array<{ name: string; policy: string; filters: string[] }>
  routingRules: ConfigRouteRule[]
  rawDnsBlock: string
  rawOtherLines?: string[]
}

// 1. Core Profile & Config States
const loading = ref(false)
const saving = ref(false)
const isDirty = ref(false)
const activatingConfig = ref(false)
const currentSelectedConfig = ref<string>('config.mock.dae')
const activeRunningConfig = ref<string>('config.mock.dae')
const configProfiles = ref<ConfigProfile[]>([])
const configModel = ref<DaeConfigModel>({
  globals: {},
  subscriptions: [],
  nodes: [],
  groups: [],
  routingRules: [],
  rawDnsBlock: ''
})

// 2. Routing Templates State
const templatesLoading = ref(false)
const templatesUpdating = ref(false)
const templateVersion = ref('2026.10.07')
const templateUpdatedAt = ref('-')
const routingTemplates = ref<RoutingTemplateItem[]>([])
const templateSearchQuery = ref('')
const templateCategoryFilter = ref('all')

// 3. UI Display Mode & Panels
type ViewTab = 'rules' | 'templates' | 'guide'
const activeTab = ref<ViewTab>('rules')
const showConflictDetails = ref(true)
const highlightedRuleIndex = ref<number | null>(null)

// 4. In-place Wide Editor State (No Modal!)
const isEditorOpen = ref(false)
const editorMode = ref<'add' | 'edit'>('add')
const editingIndex = ref<number>(-1)
type BuilderMode = 'builder' | 'preset' | 'custom'
const builderMode = ref<BuilderMode>('builder')

// Humanized condition item (NO "! 非")
interface HumanizedConditionItem {
  intent: 'match' | 'exclude' // "符合此条件" vs "排除此条件"
  targetType: 'domain' | 'dip' | 'dport' | 'sip' | 'mac' | 'l4proto' | 'ipversion' | 'pname'
  subType: string // geosite, suffix, keyword, full, regex, geoip, cidr, etc.
  value: string
}

const builderItems = ref<HumanizedConditionItem[]>([
  { intent: 'match', targetType: 'domain', subType: 'geosite', value: 'cn' }
])

const editingRule = ref<{
  condition: string
  target: string
  customTarget: string
}>({
  condition: 'domain(geosite:cn)',
  target: 'direct',
  customTarget: ''
})

// Last generated result display at the bottom
const lastGeneratedRuleStatement = ref<string>('')
const lastGeneratedExplain = ref<string>('')

// 5. Conflict Analysis
const routingHealthReport = computed<RoutingHealthReport>(() => {
  const groupNames = configModel.value.groups.map(g => g.name)
  return analyzeRoutingConflicts(configModel.value.routingRules, groupNames)
})

// Filtered templates list
const filteredTemplates = computed(() => {
  let list = routingTemplates.value
  if (templateCategoryFilter.value !== 'all') {
    list = list.filter(t => t.category === templateCategoryFilter.value)
  }
  if (templateSearchQuery.value.trim()) {
    const q = templateSearchQuery.value.toLowerCase().trim()
    list = list.filter(t =>
      t.name.toLowerCase().includes(q) ||
      t.condition.toLowerCase().includes(q) ||
      t.description.toLowerCase().includes(q)
    )
  }
  return list
})

// Load Config Profiles and Active Model
async function fetchConfigProfiles() {
  try {
    const res = await fetch('/api/configs')
    if (res.ok) {
      const data = await res.json()
      configProfiles.value = data.profiles || []
      activeRunningConfig.value = data.activeName || 'config.mock.dae'
      if (!currentSelectedConfig.value && configProfiles.value.length > 0) {
        currentSelectedConfig.value = activeRunningConfig.value || configProfiles.value[0].name
      }
    }
  } catch (err) {
    console.error('Fetch profiles error:', err)
  }
}

async function loadConfig(profileName?: string) {
  loading.value = true
  if (profileName) {
    currentSelectedConfig.value = profileName
  }
  try {
    const res = await fetch(`/api/config/model?name=${encodeURIComponent(currentSelectedConfig.value)}`)
    if (res.ok) {
      const m = await res.json()
      configModel.value = {
        globals: m.globals || {},
        subscriptions: m.subscriptions || [],
        nodes: m.nodes || [],
        groups: m.groups || [],
        routingRules: m.routingRules || [],
        rawDnsBlock: m.rawDnsBlock || '',
        rawOtherLines: m.rawOtherLines || []
      }
      isDirty.value = false
    }
    await fetchConfigProfiles()
  } catch (e: any) {
    alert('加载分流规则失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function syncModelToBackend() {
  try {
    const res = await fetch('/api/config/serialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(configModel.value)
    })
    const data = await res.json()
    if (res.ok) {
      isDirty.value = true
      // Also auto-save to file
      await fetch(`/api/config?name=${encodeURIComponent(currentSelectedConfig.value)}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: currentSelectedConfig.value,
          content: data.raw,
          autoBackup: true,
          reloadConfig: false
        })
      })
      isDirty.value = false
    }
  } catch (e) {
    console.error('Save routing rules failed:', e)
  }
}

async function activateCurrentConfig() {
  if (!confirm(`确认使配置文件 [${currentSelectedConfig.value}] 生效到主 dae 服务吗？\n生效后系统将自动重载网络分流规则。`)) {
    return
  }
  activatingConfig.value = true
  try {
    const res = await fetch('/api/configs/activate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: currentSelectedConfig.value })
    })
    const data = await res.json()
    if (res.ok) {
      alert(`生效成功！\n${data.message || ''}`)
      await fetchConfigProfiles()
      await loadConfig(currentSelectedConfig.value)
    } else {
      alert('生效失败: ' + (data.error || ''))
    }
  } finally {
    activatingConfig.value = false
  }
}

// 6. Templates API
async function loadRoutingTemplatesFromAPI() {
  templatesLoading.value = true
  try {
    const res = await fetch('/api/routing/templates')
    if (res.ok) {
      const data: RoutingTemplateResponse = await res.json()
      templateVersion.value = data.version || '2026.10.07'
      templateUpdatedAt.value = data.updatedAt || '-'
      routingTemplates.value = data.templates || []
    }
  } catch (e) {
    console.error('Fetch routing templates failed:', e)
  } finally {
    templatesLoading.value = false
  }
}

async function updateRoutingTemplatesFromAPI() {
  templatesUpdating.value = true
  try {
    const res = await fetch('/api/routing/templates/update', { method: 'POST' })
    const data = await res.json()
    if (res.ok && data.success) {
      alert(`分流规则预设模板库更新成功！\n${data.message || ''}`)
      await loadRoutingTemplatesFromAPI()
    } else {
      alert('更新模板失败: ' + (data.message || '未知错误'))
    }
  } catch (e: any) {
    alert('更新模板请求失败: ' + e.message)
  } finally {
    templatesUpdating.value = false
  }
}

// 7. Humanized Builder Helpers
function addBuilderItem() {
  builderItems.value.push({
    intent: 'match',
    targetType: 'domain',
    subType: 'geosite',
    value: ''
  })
}

function removeBuilderItem(idx: number) {
  if (builderItems.value.length > 1) {
    builderItems.value.splice(idx, 1)
  }
}

function assembleConditionFromHumanizedBuilder(): string {
  const parts: string[] = []
  for (const item of builderItems.value) {
    const val = item.value.trim()
    if (!val) continue
    let sub = ''
    if (item.targetType === 'domain') {
      if (item.subType && item.subType !== 'none') {
        sub = `domain(${item.subType}:${val})`
      } else {
        sub = `domain(${val})`
      }
    } else if (item.targetType === 'dip') {
      if (item.subType === 'geoip') {
        sub = `dip(geoip:${val})`
      } else {
        sub = `dip(${val})`
      }
    } else if (item.targetType === 'dport') {
      sub = `dport(${val})`
    } else if (item.targetType === 'sip') {
      sub = `sip(${val})`
    } else if (item.targetType === 'mac') {
      sub = `mac('${val}')`
    } else if (item.targetType === 'l4proto') {
      sub = `l4proto(${val})`
    } else if (item.targetType === 'ipversion') {
      sub = `ipversion(${val})`
    } else if (item.targetType === 'pname') {
      sub = `pname(${val})`
    }

    if (sub) {
      if (item.intent === 'exclude') {
        parts.push(`!${sub}`)
      } else {
        parts.push(sub)
      }
    }
  }
  return parts.join(' && ')
}

function openAddRuleEditor() {
  editorMode.value = 'add'
  editingIndex.value = -1
  builderMode.value = 'builder'
  builderItems.value = [
    { intent: 'match', targetType: 'domain', subType: 'geosite', value: 'cn' }
  ]
  editingRule.value = {
    condition: 'domain(geosite:cn)',
    target: 'direct',
    customTarget: ''
  }
  isEditorOpen.value = true
  // Scroll to editor
  setTimeout(() => {
    document.getElementById('rule-in-place-editor')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, 50)
}

function openEditRuleEditor(r: ConfigRouteRule, idx: number) {
  editorMode.value = 'edit'
  editingIndex.value = idx
  const knownBuiltins = ['direct', 'must_direct', 'block']
  const groupNames = configModel.value.groups.map(g => g.name)
  let targetVal = r.target
  let customVal = ''

  if (!knownBuiltins.includes(r.target) && !groupNames.includes(r.target)) {
    targetVal = '__custom__'
    customVal = r.target
  }

  editingRule.value = {
    condition: r.condition,
    target: targetVal,
    customTarget: customVal
  }

  // Attempt to parse into humanized builder items or fallback to custom code
  if (r.condition === 'fallback') {
    builderMode.value = 'custom'
  } else {
    builderMode.value = 'custom'
  }

  isEditorOpen.value = true
  setTimeout(() => {
    document.getElementById('rule-in-place-editor')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, 50)
}

function applyTemplateToEditor(tmpl: RoutingTemplateItem) {
  editingRule.value.condition = tmpl.condition
  editingRule.value.target = tmpl.recommendedTarget || 'direct'
  builderMode.value = 'custom'
  isEditorOpen.value = true
  activeTab.value = 'rules'
  setTimeout(() => {
    document.getElementById('rule-in-place-editor')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, 50)
}

function applyExampleToEditor(ex: { code: string; explain: string }, target = 'direct') {
  editingRule.value.condition = ex.code
  editingRule.value.target = target
  builderMode.value = 'custom'
  isEditorOpen.value = true
  activeTab.value = 'rules'
  setTimeout(() => {
    document.getElementById('rule-in-place-editor')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, 50)
}

async function saveRuleFromEditor() {
  let cond = editingRule.value.condition.trim()
  if (builderMode.value === 'builder') {
    cond = assembleConditionFromHumanizedBuilder()
  }

  let finalTarget = editingRule.value.target
  if (finalTarget === '__custom__') {
    finalTarget = editingRule.value.customTarget.trim()
  }

  if (!cond || !finalTarget) {
    alert('分流匹配条件与出口动作不能为空')
    return
  }

  const newRule: ConfigRouteRule = {
    condition: cond,
    target: finalTarget
  }

  if (editorMode.value === 'add') {
    configModel.value.routingRules.push(newRule)
  } else if (editingIndex.value >= 0) {
    configModel.value.routingRules[editingIndex.value] = newRule
  }

  // Update live statement at the bottom
  if (cond === 'fallback') {
    lastGeneratedRuleStatement.value = `fallback: ${finalTarget}`
    lastGeneratedExplain.value = `所有未命中上方规则的剩余网络流量，兜底导向 [${finalTarget}] 出口。`
  } else {
    lastGeneratedRuleStatement.value = `${cond} -> ${finalTarget}`
    lastGeneratedExplain.value = `当网络流量符合上述条件判定时，立即导向 [${finalTarget}] 动作出口并终止后续规则判定。`
  }

  isEditorOpen.value = false
  await syncModelToBackend()
}

function deleteRule(idx: number) {
  if (!confirm(`确认删除第 ${idx + 1} 条分流规则？`)) return
  configModel.value.routingRules.splice(idx, 1)
  syncModelToBackend()
}

// Drag & Drop free reordering (HTML5 Native Drag & Drop)
const draggedRuleIndex = ref<number | null>(null)
const dragOverTargetIndex = ref<number | null>(null)

function onRuleDragStart(idx: number, event: DragEvent) {
  const rule = configModel.value.routingRules[idx]
  if (rule.condition.trim() === 'fallback') {
    event.preventDefault()
    return
  }
  draggedRuleIndex.value = idx
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(idx))
  }
}

function onRuleDragOver(idx: number, event: DragEvent) {
  if (draggedRuleIndex.value === null || draggedRuleIndex.value === idx) return
  event.preventDefault()
  dragOverTargetIndex.value = idx
}

function onRuleDrop(idx: number, event: DragEvent) {
  event.preventDefault()
  if (draggedRuleIndex.value === null || draggedRuleIndex.value === idx) {
    onRuleDragEnd()
    return
  }

  const from = draggedRuleIndex.value
  let to = idx
  const rules = configModel.value.routingRules

  // 如果拖放到 fallback 身上，限制放在 fallback 前一位
  if (rules[to]?.condition.trim() === 'fallback') {
    to = Math.max(0, to - 1)
  }

  const [movedItem] = rules.splice(from, 1)
  rules.splice(to, 0, movedItem)

  onRuleDragEnd()
  syncModelToBackend()
}

function onRuleDragEnd() {
  draggedRuleIndex.value = null
  dragOverTargetIndex.value = null
}

function duplicateRule(idx: number) {
  const r = configModel.value.routingRules[idx]
  configModel.value.routingRules.splice(idx + 1, 0, { ...r })
  syncModelToBackend()
}

function highlightRule(idx: number) {
  highlightedRuleIndex.value = idx
  setTimeout(() => {
    const el = document.getElementById(`routing-table-row-${idx}`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, 50)
  setTimeout(() => {
    if (highlightedRuleIndex.value === idx) {
      highlightedRuleIndex.value = null
    }
  }, 3500)
}

function fixMissingPrivateBypass() {
  configModel.value.routingRules.unshift({
    condition: 'dip(geoip:private)',
    target: 'direct'
  })
  syncModelToBackend()
}

function fixFallbackOrder() {
  const rules = configModel.value.routingRules
  const fbIdx = rules.findIndex(r => r.condition.trim() === 'fallback')
  if (fbIdx >= 0 && fbIdx < rules.length - 1) {
    const [fbRule] = rules.splice(fbIdx, 1)
    rules.push(fbRule)
    syncModelToBackend()
  }
}

onMounted(async () => {
  await fetchConfigProfiles()
  await loadConfig()
  await loadRoutingTemplatesFromAPI()
})
</script>

<template>
  <div class="routing-page">
    <!-- Top Toolbar Header -->
    <header class="routing-toolbar">
      <div class="toolbar-left">
        <div class="page-title-badge">
          <GitFork class="title-icon" :size="20" />
          <div>
            <h2>分流与路由规则管理</h2>
            <span class="subtext">eBPF 内核级流量特征精准匹配与 First-Match-Wins 策略出口管控</span>
          </div>
        </div>

        <!-- Config profile switcher -->
        <div class="profile-switch-box">
          <Files class="text-muted" :size="14" />
          <select
            :value="currentSelectedConfig"
            @change="loadConfig(($event.target as HTMLSelectElement).value)"
            class="profile-select"
            title="选择要配置的分流方案"
          >
            <option
              v-for="p in configProfiles"
              :key="p.name"
              :value="p.name"
            >
              {{ p.name }} {{ p.isActive ? ' [正在生效]' : '' }}
            </option>
          </select>

          <span
            v-if="currentSelectedConfig === activeRunningConfig"
            class="badge-active"
            title="该配置文件正在作为主运行服务生效"
          >
            <span class="active-dot"></span>已生效
          </span>
          <button
            v-else
            class="btn btn-sm btn-activate"
            :disabled="activatingConfig"
            @click="activateCurrentConfig"
            title="将当前分流方案应用并热生效到系统"
          >
            <Check :size="12" />
            <span>{{ activatingConfig ? '生效中...' : '一键生效' }}</span>
          </button>
        </div>
      </div>

      <div class="toolbar-right">
        <!-- View tabs switch -->
        <div class="tab-toggle-group">
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'rules' }"
            @click="activeTab = 'rules'"
          >
            <GitFork :size="14" />
            <span>分流规则列表 ({{ configModel.routingRules.length }})</span>
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'templates' }"
            @click="activeTab = 'templates'"
          >
            <Sparkles :size="14" />
            <span>预设模板库 ({{ routingTemplates.length }})</span>
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'guide' }"
            @click="activeTab = 'guide'"
          >
            <BookOpen :size="14" />
            <span>官方语法指南</span>
          </button>
        </div>

        <button
          v-if="!isEditorOpen"
          class="btn btn-primary"
          @click="openAddRuleEditor"
          title="新增一条分流规则"
        >
          <Plus :size="14" /> 新增路由规则
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="routing-main">
      <!-- ========================================================== -->
      <!-- Tab 1: Rules List & In-place Wide Editor -->
      <!-- ========================================================== -->
      <div v-if="activeTab === 'rules'" class="rules-view-container">
        <!-- 1. Conflict & Health Diagnostics Bar -->
        <div class="health-diagnostics-bar">
          <div
            v-if="!routingHealthReport.hasErrors && !routingHealthReport.hasWarnings && routingHealthReport.conflicts.length === 0"
            class="health-status-banner ok"
          >
            <ShieldCheck class="banner-icon" :size="18" />
            <div class="banner-info">
              <span class="title">分流规则完全健康 (共 {{ configModel.routingRules.length }} 条)</span>
              <span class="desc">所有规则自上而下匹配，未检测到死规则遮蔽 (Shadowed)、重复规则或 Fallback 兜底顺序错误。</span>
            </div>
          </div>

          <div
            v-else
            class="health-card"
            :class="{ error: routingHealthReport.hasErrors }"
          >
            <div class="health-card-head" @click="showConflictDetails = !showConflictDetails">
              <div class="head-left">
                <ShieldAlert v-if="routingHealthReport.hasErrors" class="status-icon danger" :size="18" />
                <AlertTriangle v-else class="status-icon warning" :size="18" />
                <div>
                  <h4 class="head-title">
                    规则冲突与健康自诊断报告
                    <span class="head-pill">共 {{ routingHealthReport.conflicts.length }} 项提示</span>
                  </h4>
                  <span class="head-sub">dae 严格按照从上到下的顺序进行路由判定（First-Match-Wins）。检测到以下潜在遮蔽或网络问题：</span>
                </div>
              </div>
              <button class="btn-icon-subtle" :title="showConflictDetails ? '收起' : '展开'">
                <ChevronUp v-if="showConflictDetails" :size="16" />
                <ChevronDown v-else :size="16" />
              </button>
            </div>

            <div v-if="showConflictDetails" class="health-card-body">
              <div
                v-for="c in routingHealthReport.conflicts"
                :key="c.id"
                class="diag-item"
                :class="c.severity"
              >
                <div class="diag-item-head">
                  <span class="diag-tag" :class="c.severity">
                    {{ c.severity === 'error' ? '错误 / 逻辑冲突' : (c.severity === 'warning' ? '警告 / 阴影死规则' : '最佳实践建议') }}
                  </span>
                  <strong class="diag-title">{{ c.title }}</strong>
                  <div v-if="c.ruleIndices.length > 0" class="diag-involved">
                    <span>涉及行号:</span>
                    <button
                      v-for="idx in c.ruleIndices"
                      :key="idx"
                      class="rule-pill-btn"
                      @click="highlightRule(idx)"
                      title="点击定位高亮该条规则"
                    >
                      #{{ idx + 1 }}
                    </button>
                  </div>
                </div>

                <p class="diag-msg">{{ c.message }}</p>

                <div class="diag-sugg-box">
                  <Lightbulb :size="14" class="sugg-icon" />
                  <div class="sugg-text">
                    <strong>修改优化建议:</strong>
                    <span>{{ c.suggestion }}</span>
                  </div>
                  <div class="sugg-actions">
                    <button
                      v-if="c.id === 'missing-private-bypass'"
                      class="btn btn-sm btn-quick-fix"
                      @click="fixMissingPrivateBypass"
                    >
                      <Wand2 :size="12" /> 一键在顶部插入私网直连
                    </button>
                    <button
                      v-if="c.id === 'fallback-not-last'"
                      class="btn btn-sm btn-quick-fix"
                      @click="fixFallbackOrder"
                    >
                      <Wand2 :size="12" /> 一键将 Fallback 移至末尾
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 2. IN-PLACE WIDE WORKSPACE EDITOR (NO MODAL!) -->
        <div v-if="isEditorOpen" id="rule-in-place-editor" class="in-place-editor-card">
          <div class="editor-header">
            <div class="header-title">
              <Wand2 class="icon-accent" :size="18" />
              <h3>{{ editorMode === 'add' ? '新增路由分流规则' : `编辑第 ${editingIndex + 1} 条分流规则` }}</h3>
              <span class="editor-badge">{{ editorMode === 'add' ? '添加模式' : '编辑中' }}</span>
            </div>
            <button class="btn-icon-subtle" @click="isEditorOpen = false" title="收起编辑器">
              &times;
            </button>
          </div>

          <div class="editor-body">
            <!-- Mode Switcher -->
            <div class="builder-nav-bar">
              <span class="nav-label">配置模式:</span>
              <div class="pill-switch">
                <button
                  type="button"
                  class="pill-btn"
                  :class="{ active: builderMode === 'builder' }"
                  @click="builderMode = 'builder'"
                >
                  <Wand2 :size="13" />
                  <span>🛠️ 人性化条件构造器 (推荐)</span>
                </button>
                <button
                  type="button"
                  class="pill-btn"
                  :class="{ active: builderMode === 'custom' }"
                  @click="builderMode = 'custom'"
                >
                  <Code :size="13" />
                  <span>✏️ 手写专家代码 (dae 表达式)</span>
                </button>
              </div>
            </div>

            <!-- Builder Mode: Humanized (NO "! 非") -->
            <div v-if="builderMode === 'builder'" class="humanized-builder-box">
              <div class="builder-block-list">
                <div
                  v-for="(item, bIdx) in builderItems"
                  :key="bIdx"
                  class="builder-block"
                >
                  <div class="block-top-row">
                    <!-- 1. Intent: Match vs Exclude -->
                    <div class="ctrl-group">
                      <label class="ctrl-lbl">匹配方式:</label>
                      <select v-model="item.intent" class="form-select intent-select">
                        <option value="match">🟢 符合此条件 (匹配)</option>
                        <option value="exclude">🔴 排除此条件 (取反)</option>
                      </select>
                    </div>

                    <!-- 2. Target Type -->
                    <div class="ctrl-group">
                      <label class="ctrl-lbl">流量对象:</label>
                      <select v-model="item.targetType" class="form-select type-select">
                        <option value="domain">🌐 网站域名 (domain)</option>
                        <option value="dip">🎯 目标 IP / 网段 (dip)</option>
                        <option value="dport">🔌 网络端口 (dport)</option>
                        <option value="sip">💻 客户端设备 IP (sip)</option>
                        <option value="mac">🏷️ 客户端 MAC 地址 (mac)</option>
                        <option value="l4proto">⚡ 传输协议 (l4proto)</option>
                        <option value="ipversion">🔢 IP 协议版本 (ipversion)</option>
                        <option value="pname">📦 本机进程名 (pname)</option>
                      </select>
                    </div>

                    <!-- 3. Subtype modifiers -->
                    <div v-if="item.targetType === 'domain'" class="ctrl-group">
                      <label class="ctrl-lbl">匹配范围:</label>
                      <select v-model="item.subType" class="form-select">
                        <option value="geosite">预置规则库 (geosite: 推荐)</option>
                        <option value="suffix">域名后缀 (suffix:)</option>
                        <option value="keyword">包含关键字 (keyword:)</option>
                        <option value="full">精准全域名 (full:)</option>
                        <option value="regex">正则表达式 (regex:)</option>
                        <option value="none">默认泛域名</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'dip'" class="ctrl-group">
                      <label class="ctrl-lbl">匹配方式:</label>
                      <select v-model="item.subType" class="form-select">
                        <option value="geoip">地理/私网规则库 (geoip: 如 cn, private)</option>
                        <option value="none">指定 IP 地址或 CIDR 网段</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'l4proto'" class="ctrl-group">
                      <label class="ctrl-lbl">协议类型:</label>
                      <select v-model="item.value" class="form-select">
                        <option value="tcp">TCP 传输连接</option>
                        <option value="udp">UDP 数据报文</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'ipversion'" class="ctrl-group">
                      <label class="ctrl-lbl">协议版本:</label>
                      <select v-model="item.value" class="form-select">
                        <option value="4">IPv4</option>
                        <option value="6">IPv6</option>
                      </select>
                    </div>

                    <!-- Delete button -->
                    <button
                      type="button"
                      class="btn-icon delete remove-block-btn"
                      :disabled="builderItems.length <= 1"
                      @click="removeBuilderItem(bIdx)"
                      title="删除此项条件"
                    >
                      <Trash2 :size="14" />
                    </button>
                  </div>

                  <!-- Value input row with quick tags -->
                  <div
                    v-if="item.targetType !== 'l4proto' && item.targetType !== 'ipversion'"
                    class="block-value-row"
                  >
                    <div class="val-input-wrap">
                      <input
                        type="text"
                        v-model="item.value"
                        class="form-input val-input mono"
                        :placeholder="
                          item.targetType === 'domain' ? (item.subType === 'geosite' ? '输入或点击下方规则标签 (如: cn, google)' : '输入域名 (如: baidu.com)') :
                          item.targetType === 'dip' ? (item.subType === 'geoip' ? '输入 geoip 标识 (如: private 或 cn)' : '如: 192.168.1.0/24 或 223.5.5.5') :
                          item.targetType === 'dport' ? '输入端口号 (如: 80, 443 或 53 或 10000:20000)' :
                          item.targetType === 'sip' ? '输入内网客户端 IP (如: 192.168.1.100)' :
                          item.targetType === 'mac' ? '输入客户端 MAC (如: 00:11:22:33:44:55)' :
                          item.targetType === 'pname' ? '输入本机进程名 (如: curl, chrome)' : '输入参数值'
                        "
                      />
                    </div>

                    <!-- Fast Clickable Tags -->
                    <div class="fast-tags-wrap">
                      <span class="tags-hint">常用快捷选择:</span>
                      <template v-if="item.targetType === 'domain' && item.subType === 'geosite'">
                        <button type="button" class="tag-pill" @click="item.value = 'cn'">🇨🇳 cn (大陆网站)</button>
                        <button type="button" class="tag-pill" @click="item.value = 'google'">Google 全系</button>
                        <button type="button" class="tag-pill" @click="item.value = 'youtube'">YouTube 视频</button>
                        <button type="button" class="tag-pill" @click="item.value = 'category-ads-all'">🚫 广告拦截</button>
                        <button type="button" class="tag-pill" @click="item.value = 'github'">GitHub 开源</button>
                        <button type="button" class="tag-pill" @click="item.value = 'netflix,disney'">流媒体服务</button>
                      </template>
                      <template v-else-if="item.targetType === 'dip' && item.subType === 'geoip'">
                        <button type="button" class="tag-pill" @click="item.value = 'private'">🔒 private (内网私网)</button>
                        <button type="button" class="tag-pill" @click="item.value = 'cn'">🇨🇳 cn (大陆IP)</button>
                        <button type="button" class="tag-pill" @click="item.value = 'us'">🇺🇸 us (美国IP)</button>
                      </template>
                      <template v-else-if="item.targetType === 'dport'">
                        <button type="button" class="tag-pill" @click="item.value = '80, 443'">🌐 80, 443 (Web网页)</button>
                        <button type="button" class="tag-pill" @click="item.value = '53'">🔍 53 (DNS解析)</button>
                        <button type="button" class="tag-pill" @click="item.value = '22'">💻 22 (SSH运维)</button>
                        <button type="button" class="tag-pill" @click="item.value = '6881:6889'">⚡ BT下载端口</button>
                      </template>
                    </div>
                  </div>

                  <!-- "AND" connector indicator -->
                  <div v-if="bIdx < builderItems.length - 1" class="and-connector">
                    <span class="and-badge">并且 (需同时满足)</span>
                  </div>
                </div>
              </div>

              <div class="builder-bottom-actions">
                <button type="button" class="btn btn-secondary btn-sm" @click="addBuilderItem">
                  <Plus :size="13" /> 添加组合条件 (并且同时满足)
                </button>
              </div>
            </div>

            <!-- Custom Mode: Code Expression -->
            <div v-else class="custom-editor-box">
              <div class="form-group">
                <label>手写匹配表达式 (Condition 语法)</label>
                <input
                  type="text"
                  v-model="editingRule.condition"
                  class="form-input mono"
                  placeholder="例如: domain(geosite:cn) && dport(443) 或 fallback"
                />
                <span class="field-hint">支持使用 && 组合条件，支持 domain, dip, dport, l4proto, pname, sip, mac 及 fallback</span>
              </div>
            </div>

            <!-- Target Selection Row -->
            <div class="target-select-row">
              <div class="form-group flex-1">
                <label>目标动作 (Target / 出口分流)</label>
                <select v-model="editingRule.target" class="form-select target-select">
                  <optgroup label="内置直连与阻断动作 (Built-in Actions)">
                    <option value="direct">direct (直接放行 / 常规直连)</option>
                    <option value="must_direct">must_direct (强制直连 / 避开本地 DNS 劫持)</option>
                    <option value="block">block (阻断连接 / 丢弃数据包)</option>
                  </optgroup>
                  <optgroup label="代理策略组 (Proxy Groups)">
                    <option v-for="g in configModel.groups" :key="g.name" :value="g.name">
                      {{ g.name }} (出站策略: {{ g.policy }})
                    </option>
                  </optgroup>
                  <optgroup label="高级">
                    <option value="__custom__">自定义策略组或动作名...</option>
                  </optgroup>
                </select>
                <span class="field-hint">
                  提示：上游 DNS 解析强烈推荐使用 <code>must_direct</code> 避免形成递归回环；广告拦截使用 <code>block</code>。
                </span>
              </div>

              <div v-if="editingRule.target === '__custom__'" class="form-group flex-1">
                <label>输入自定义目标出口名称</label>
                <input
                  type="text"
                  v-model="editingRule.customTarget"
                  class="form-input mono"
                  placeholder="输入合法的组名"
                />
              </div>
            </div>

            <!-- Editor Buttons -->
            <div class="editor-footer-actions">
              <button class="btn btn-secondary" @click="isEditorOpen = false">取消</button>
              <button class="btn btn-primary" @click="saveRuleFromEditor">
                <Check :size="14" /> 确定保存分流规则
              </button>
            </div>
          </div>
        </div>

        <!-- 3. Rules Table Main Section -->
        <div class="rules-table-card">
          <div class="table-card-header">
            <div class="table-title">
              <h3>规则列表执行顺序 (自上而下 First-Match-Wins)</h3>
              <span class="rules-count-pill">共 {{ configModel.routingRules.length }} 条规则</span>
            </div>

            <div class="table-right-tools">
              <span class="text-muted table-tip">
                <Info :size="13" /> 优先命中的特定细分规则应上移放置于上方
              </span>
            </div>
          </div>

          <div v-if="configModel.routingRules.length === 0" class="empty-state-card">
            <GitFork class="empty-icon" :size="48" />
            <h4>当前方案中暂无分流规则</h4>
            <p>你可以点击上方「新增路由规则」，或者前往「预设模板库」一键套用常用场景。</p>
            <button class="btn btn-primary btn-sm" @click="openAddRuleEditor">
              <Plus :size="13" /> 立即新增分流规则
            </button>
          </div>

          <div v-else class="table-responsive">
            <table class="routing-table">
              <thead>
                <tr>
                  <th style="width: 44px; text-align: center" title="按住手柄上下拖动">拖动</th>
                  <th style="width: 50px"># 序号</th>
                  <th>匹配条件 (Condition)</th>
                  <th style="width: 170px">动作出口 (Target)</th>
                  <th style="width: 130px">状态健康</th>
                  <th style="width: 140px; text-align: right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(rule, idx) in configModel.routingRules"
                  :key="idx"
                  :id="`routing-table-row-${idx}`"
                  :draggable="rule.condition !== 'fallback'"
                  @dragstart="onRuleDragStart(idx, $event)"
                  @dragover="onRuleDragOver(idx, $event)"
                  @drop="onRuleDrop(idx, $event)"
                  @dragend="onRuleDragEnd"
                  :class="{
                    'row-highlighted': highlightedRuleIndex === idx,
                    'row-dragging': draggedRuleIndex === idx,
                    'row-drag-over': dragOverTargetIndex === idx,
                    'row-fallback-fixed': rule.condition === 'fallback',
                    'row-error': routingHealthReport.ruleStatusMap[idx]?.severity === 'error',
                    'row-warning': routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'
                  }"
                >
                  <td class="drag-grip-col">
                    <div
                      v-if="rule.condition !== 'fallback'"
                      class="drag-handle-grip"
                      title="按住鼠标拖拽自由上下调整优先级"
                    >
                      <GripVertical :size="16" />
                    </div>
                    <div
                      v-else
                      class="drag-handle-grip disabled"
                      title="兜底规则固定于末尾，无法拖动"
                    >
                      <GripVertical :size="16" />
                    </div>
                  </td>
                  <td class="idx-col">
                    <span class="idx-chip">{{ idx + 1 }}</span>
                  </td>
                  <td class="cond-col mono">
                    <span v-if="rule.condition === 'fallback'" class="fallback-badge">
                      兜底规则 (fallback - 固定于末尾)
                    </span>
                    <span v-else class="rule-code-text">{{ rule.condition }}</span>
                  </td>
                  <td>
                    <span class="target-chip" :class="rule.target">{{ rule.target }}</span>
                  </td>
                  <td>
                    <span
                      v-if="routingHealthReport.ruleStatusMap[idx]?.severity === 'error'"
                      class="health-tag danger"
                      :title="routingHealthReport.ruleStatusMap[idx]?.messages.join('\n')"
                    >
                      <ShieldAlert :size="12" /> 严重冲突
                    </span>
                    <span
                      v-else-if="routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'"
                      class="health-tag warning"
                      :title="routingHealthReport.ruleStatusMap[idx]?.messages.join('\n')"
                    >
                      <AlertTriangle :size="12" /> 规则遮蔽
                    </span>
                    <span v-else class="health-tag ok">
                      <Check :size="12" /> 正常
                    </span>
                  </td>
                  <td class="actions-col">
                    <button
                      class="btn-icon"
                      @click="duplicateRule(idx)"
                      title="复制规则"
                    >
                      <Copy :size="13" />
                    </button>
                    <button
                      class="btn-icon"
                      @click="openEditRuleEditor(rule, idx)"
                      title="编辑此规则"
                    >
                      <Edit2 :size="13" />
                    </button>
                    <button
                      class="btn-icon delete"
                      @click="deleteRule(idx)"
                      title="删除规则"
                    >
                      <Trash2 :size="13" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- 4. LIVE GENERATED DAE STATEMENT RESULT (AT THE VERY BOTTOM) -->
        <div v-if="lastGeneratedRuleStatement" class="bottom-statement-result-card">
          <div class="result-header">
            <CheckCircle2 class="text-success" :size="18" />
            <div>
              <h4>最近生成的 dae 规则语句</h4>
              <span class="subtext">这是完成配置操作后最终转换输出的精准 dae 规则代码</span>
            </div>
          </div>
          <div class="result-code-box">
            <code class="code-mono">{{ lastGeneratedRuleStatement }}</code>
          </div>
          <p class="result-explain">{{ lastGeneratedExplain }}</p>
        </div>
      </div>

      <!-- ========================================================== -->
      <!-- Tab 2: Routing Templates (Single Column List Layout) -->
      <!-- ========================================================== -->
      <div v-else-if="activeTab === 'templates'" class="templates-view-container">
        <!-- Templates Top Bar -->
        <div class="templates-top-card">
          <div class="top-info">
            <div class="title-row">
              <Sparkles class="icon-accent" :size="20" />
              <h3>分流规则预设模板库</h3>
              <span class="ver-tag">版本: v{{ templateVersion }}</span>
            </div>
            <p class="desc">
              收录局域网放行、中国大陆直连、广告阻断、境外服务、AI工具等高频分流范式，支持一键载入编辑表单。
            </p>
          </div>

          <div class="top-actions">
            <button
              class="btn btn-secondary"
              :disabled="templatesUpdating"
              @click="updateRoutingTemplatesFromAPI"
              title="从服务端拉取最新模板配置"
            >
              <RefreshCw :size="14" :class="{ spin: templatesUpdating }" />
              <span>{{ templatesUpdating ? '正在同步...' : '检查与同步最新模板' }}</span>
            </button>
          </div>
        </div>

        <!-- Templates Filter Bar -->
        <div class="templates-filter-bar">
          <div class="search-input-wrap">
            <Search :size="14" class="search-icon text-muted" />
            <input
              type="text"
              v-model="templateSearchQuery"
              placeholder="搜索预设模板名称、匹配条件或用途说明..."
              class="search-input"
            />
          </div>

          <div class="category-tabs">
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'all' }"
              @click="templateCategoryFilter = 'all'"
            >
              全部 ({{ routingTemplates.length }})
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'common' }"
              @click="templateCategoryFilter = 'common'"
            >
              基础常用
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'security' }"
              @click="templateCategoryFilter = 'security'"
            >
              安全与DNS
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'media' }"
              @click="templateCategoryFilter = 'media'"
            >
              媒体与游戏
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'dev' }"
              @click="templateCategoryFilter = 'dev'"
            >
              开发与AI
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'advanced' }"
              @click="templateCategoryFilter = 'advanced'"
            >
              高级端口
            </button>
          </div>
        </div>

        <!-- Single Column Templates List -->
        <div class="templates-single-col-list">
          <div
            v-for="tmpl in filteredTemplates"
            :key="tmpl.id"
            class="template-list-row"
          >
            <div class="tmpl-left-info">
              <div class="tmpl-head-line">
                <span class="tmpl-name">{{ tmpl.name }}</span>
                <span class="tmpl-cat-badge">{{ tmpl.category }}</span>
                <span class="tmpl-target-badge">推荐出口: {{ tmpl.recommendedTarget }}</span>
              </div>

              <div class="tmpl-cond-line">
                <code class="tmpl-cond-code">{{ tmpl.condition }}</code>
              </div>

              <div class="tmpl-desc-text">
                {{ tmpl.description }}
              </div>

              <div v-if="tmpl.tip" class="tmpl-tip-text">
                <Lightbulb :size="13" class="tip-icon" />
                <span>{{ tmpl.tip }}</span>
              </div>
            </div>

            <div class="tmpl-right-actions">
              <button
                class="btn btn-primary btn-sm btn-apply"
                @click="applyTemplateToEditor(tmpl)"
              >
                <Plus :size="13" /> 填入并使用此模板
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- ========================================================== -->
      <!-- Tab 3: Syntax Guide (Single Column List Layout) -->
      <!-- ========================================================== -->
      <div v-else-if="activeTab === 'guide'" class="guide-view-container">
        <!-- Guide Top Banner -->
        <div class="guide-intro-banner">
          <div class="banner-title-line">
            <BookOpen class="icon-accent" :size="20" />
            <h3>dae 官方分流规则语法与核心机制说明</h3>
          </div>
          <p class="banner-desc">
            dae 基于 eBPF 在 Linux 内核层捕获网络流量，并遵循自上而下的<strong>「首条命中即终止」(First-Match-Wins)</strong> 原则。匹配条件由一个或多个 Matcher 构成，支持用 <code>&amp;&amp;</code> 组合与 <code>!</code> 取反。
          </p>
        </div>

        <!-- Single Column Syntax Docs List -->
        <div class="syntax-single-col-list">
          <div
            v-for="(sec, sIdx) in ROUTING_SYNTAX_GUIDE"
            :key="sIdx"
            class="syntax-doc-row"
          >
            <div class="doc-row-header">
              <h4 class="doc-title">{{ sec.title }}</h4>
              <code class="doc-syntax-chip">{{ sec.syntax }}</code>
            </div>

            <!-- Full explanation text without cutting/ellipsis -->
            <p class="doc-summary-full">{{ sec.summary }}</p>

            <div class="doc-examples-list">
              <div
                v-for="(ex, eIdx) in sec.examples"
                :key="eIdx"
                class="example-full-item"
              >
                <div class="ex-left-content">
                  <div class="ex-code-wrap">
                    <code class="ex-code">{{ ex.code }}</code>
                  </div>
                  <div class="ex-explain-full">
                    {{ ex.explain }}
                  </div>
                </div>

                <div class="ex-right-action">
                  <button
                    class="btn btn-secondary btn-xs btn-use-ex"
                    @click="applyExampleToEditor(ex)"
                    title="将此示例表达式填入主工作区编辑表单"
                  >
                    <Plus :size="12" /> 填入表单
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.routing-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  overflow-y: auto;
}

/* Toolbar Header */
.routing-toolbar {
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

.profile-switch-box {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  padding: 4px 10px;
  border-radius: 6px;
}

.profile-select {
  background: transparent;
  border: none;
  color: #0284c7;
  font-size: 13px;
  font-weight: 500;
  outline: none;
  cursor: pointer;
}

.badge-active {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  padding: 2px 6px;
  border-radius: 10px;
}

.active-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #10b981;
}

.btn-activate {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.3);
  padding: 2px 8px;
  font-size: 11px;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.tab-toggle-group {
  display: flex;
  gap: 4px;
  background: var(--bg-input, #090d16);
  padding: 3px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: transparent;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.tab-btn:hover {
  color: var(--text-main, #f8fafc);
}

.tab-btn.active {
  background: var(--bg-card, #1e293b);
  color: #0284c7;
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

/* Main Layout */
.routing-main {
  flex: 1;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Diagnostics Bar */
.health-diagnostics-bar {
  display: flex;
  flex-direction: column;
}

.health-status-banner.ok {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-radius: 8px;
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.25);
}

.health-status-banner.ok .banner-icon {
  color: #10b981;
  flex-shrink: 0;
}

.banner-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.banner-info .title {
  font-size: 13px;
  font-weight: 600;
  color: #10b981;
}

.banner-info .desc {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.health-card {
  border-radius: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid rgba(245, 158, 11, 0.3);
  overflow: hidden;
}

.health-card.error {
  border-color: rgba(239, 68, 68, 0.35);
}

.health-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--bg-input, #090d16);
  cursor: pointer;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.head-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.head-left .status-icon.warning {
  color: #f59e0b;
}

.head-left .status-icon.danger {
  color: #ef4444;
}

.head-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  display: flex;
  align-items: center;
  gap: 8px;
}

.head-pill {
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 10px;
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.health-card.error .head-pill {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.head-sub {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.health-card-body {
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.diag-item {
  padding: 10px 14px;
  border-radius: 6px;
  background: var(--bg-input, #090d16);
  border-left: 3px solid #0284c7;
}

.diag-item.error {
  border-left-color: #ef4444;
  background: rgba(239, 68, 68, 0.04);
}

.diag-item.warning {
  border-left-color: #f59e0b;
  background: rgba(245, 158, 11, 0.04);
}

.diag-item-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.diag-tag {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
}

.diag-tag.error {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.diag-tag.warning {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.diag-tag.info {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.diag-title {
  font-size: 13px;
  color: var(--text-main, #f8fafc);
}

.diag-involved {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.rule-pill-btn {
  padding: 1px 6px;
  background: var(--bg-card, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  font-size: 11px;
  color: #0284c7;
  font-weight: 600;
  cursor: pointer;
}

.diag-msg {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--text-main, #e2e8f0);
  line-height: 1.5;
}

.diag-sugg-box {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 4px;
  background: var(--bg-card, #0f172a);
  border: 1px dashed var(--border-color, #334155);
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  flex-wrap: wrap;
}

.sugg-icon {
  color: #f59e0b;
  flex-shrink: 0;
  margin-top: 2px;
}

.sugg-text strong {
  color: #f59e0b;
  margin-right: 4px;
}

.sugg-actions {
  margin-left: auto;
}

.btn-quick-fix {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.3);
  font-size: 11px;
  gap: 4px;
}

/* ========================================================== */
/* IN-PLACE WIDE WORKSPACE EDITOR (NO MODAL!) */
/* ========================================================== */
.in-place-editor-card {
  border-radius: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid #0284c7;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25);
  overflow: hidden;
  margin-bottom: 16px;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  background: var(--bg-input, #090d16);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.icon-accent {
  color: #0284c7;
}

.header-title h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.editor-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  font-weight: 500;
}

.editor-body {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.builder-nav-bar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.nav-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.pill-switch {
  display: flex;
  gap: 6px;
  background: var(--bg-input, #090d16);
  padding: 3px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
}

/* Humanized Builder Blocks */
.humanized-builder-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.builder-block-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.builder-block {
  padding: 12px 14px;
  background: var(--bg-input, #090d16);
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.block-top-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.ctrl-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ctrl-lbl {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
}

.form-select {
  padding: 6px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  outline: none;
}

.intent-select {
  font-weight: 600;
}

.type-select {
  font-weight: 600;
  color: #0284c7;
}

.remove-block-btn {
  margin-left: auto;
}

.block-value-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.val-input {
  width: 100%;
}

.fast-tags-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.tags-hint {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.tag-pill {
  padding: 2px 7px;
  background: var(--bg-card, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  font-size: 11px;
  color: var(--text-main, #cbd5e1);
  cursor: pointer;
  transition: all 0.15s;
}

.tag-pill:hover {
  background: #0284c7;
  color: #fff;
  border-color: #0284c7;
}

.and-connector {
  display: flex;
  justify-content: center;
  margin-top: 4px;
}

.and-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 12px;
  background: rgba(2, 132, 199, 0.12);
  color: #0284c7;
  border: 1px dashed rgba(2, 132, 199, 0.3);
}

.builder-bottom-actions {
  display: flex;
  justify-content: flex-start;
}

/* Custom Editor */
.custom-editor-box {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.form-input {
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 13px;
  outline: none;
}

.field-hint {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.target-select-row {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.target-select {
  padding: 8px 12px;
  font-size: 13px;
}

.flex-1 {
  flex: 1;
  min-width: 250px;
}

.editor-footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-color, #1e293b);
}

/* Table Card */
.rules-table-card {
  background: var(--bg-card, #0f172a);
  border-radius: 8px;
  border: 1px solid var(--border-color, #1e293b);
  overflow: hidden;
}

.table-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 18px;
  background: var(--bg-input, #090d16);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.table-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.table-title h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.rules-count-pill {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--bg-card, #1e293b);
  color: #0284c7;
}

.table-tip {
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.empty-state-card {
  padding: 48px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.empty-icon {
  color: var(--text-muted, #94a3b8);
  opacity: 0.5;
}

.empty-state-card h4 {
  margin: 0;
  font-size: 15px;
}

.empty-state-card p {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.table-responsive {
  overflow-x: auto;
}

.routing-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.routing-table th {
  padding: 10px 14px;
  text-align: left;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.routing-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #f8fafc);
}

.routing-table tr:hover {
  background: var(--nav-hover, rgba(30, 41, 59, 0.4));
}

.routing-table tr.row-highlighted {
  background: rgba(2, 132, 199, 0.18) !important;
  animation: pulseHighlight 1.5s ease-in-out infinite alternate;
}

.routing-table tr.row-error {
  border-left: 3px solid #ef4444;
}

.routing-table tr.row-warning {
  border-left: 3px solid #f59e0b;
}

/* Drag & Drop Grip and Fixed Fallback */
.drag-grip-col {
  text-align: center;
  width: 44px;
}

.drag-handle-grip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #94a3b8);
  cursor: grab;
  padding: 3px;
  border-radius: 4px;
  transition: all 0.15s;
}

.drag-handle-grip:hover {
  color: #0284c7;
  background: var(--bg-input, #1e293b);
}

.drag-handle-grip:active {
  cursor: grabbing;
}

.drag-handle-grip.disabled {
  opacity: 0.25;
  cursor: not-allowed;
}

.routing-table tr.row-dragging {
  opacity: 0.4;
  background: var(--bg-input, #1e293b) !important;
}

.routing-table tr.row-drag-over {
  box-shadow: inset 0 2px 0 #0284c7, inset 0 -2px 0 #0284c7;
  background: rgba(2, 132, 199, 0.08) !important;
}

.routing-table tr.row-fallback-fixed {
  background: rgba(148, 163, 184, 0.04) !important;
  cursor: not-allowed;
}

.routing-table tr.row-fallback-fixed td {
  color: var(--text-muted, #94a3b8) !important;
}

.idx-chip {
  display: inline-block;
  padding: 2px 7px;
  border-radius: 4px;
  background: var(--bg-input, #1e293b);
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.fallback-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(147, 51, 234, 0.15);
  color: #a855f7;
  font-weight: 600;
  font-size: 12px;
}

.rule-code-text {
  font-size: 12px;
}

.target-chip {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.target-chip.direct {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.target-chip.must_direct {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.target-chip.block {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.health-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.health-tag.ok {
  color: #10b981;
  background: rgba(16, 185, 129, 0.08);
}

.health-tag.warning {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.12);
}

.health-tag.danger {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.12);
}

.actions-col {
  text-align: right;
  white-space: nowrap;
}

/* Bottom Statement Result */
.bottom-statement-result-card {
  padding: 14px 18px;
  border-radius: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.result-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.result-header h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.result-code-box {
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border-left: 3px solid #10b981;
  border-radius: 4px;
}

.result-code-box code {
  color: #10b981;
  font-size: 13px;
  font-weight: 600;
}

.result-explain {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

/* ========================================================== */
/* Tab 2: Single Column List Templates */
/* ========================================================== */
.templates-view-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.templates-top-card {
  padding: 16px 20px;
  border-radius: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

.title-row h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.ver-tag {
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 10px;
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  font-weight: 500;
}

.templates-top-card .desc {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.templates-filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}

.search-input-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  padding: 6px 12px;
  border-radius: 6px;
  flex: 1;
  max-width: 400px;
}

.search-input {
  background: transparent;
  border: none;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  outline: none;
  width: 100%;
}

.category-tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.cat-pill {
  padding: 4px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.cat-pill:hover {
  color: var(--text-main, #f8fafc);
}

.cat-pill.active {
  background: var(--bg-input, #1e293b);
  color: #0284c7;
  border-color: #0284c7;
  font-weight: 600;
}

/* Single column list */
.templates-single-col-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.template-list-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  gap: 16px;
  transition: border-color 0.15s;
}

.template-list-row:hover {
  border-color: #0284c7;
}

.tmpl-left-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.tmpl-head-line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.tmpl-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.tmpl-cat-badge {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-input, #1e293b);
  color: var(--text-muted, #94a3b8);
  text-transform: uppercase;
}

.tmpl-target-badge {
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 4px;
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
  font-weight: 500;
}

.tmpl-cond-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: #0284c7;
  font-weight: 600;
  background: var(--bg-input, #090d16);
  padding: 2px 8px;
  border-radius: 4px;
  display: inline-block;
}

.tmpl-desc-text {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.45;
}

.tmpl-tip-text {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #f59e0b;
  font-weight: 500;
}

.tmpl-right-actions {
  flex-shrink: 0;
}

.btn-apply {
  white-space: nowrap;
}

/* ========================================================== */
/* Tab 3: Single Column List Syntax Guide */
/* ========================================================== */
.guide-view-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.guide-intro-banner {
  padding: 16px 20px;
  border-radius: 8px;
  background: rgba(2, 132, 199, 0.08);
  border: 1px solid rgba(2, 132, 199, 0.25);
}

.banner-title-line {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.banner-title-line h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #0284c7;
}

.banner-desc {
  margin: 0;
  font-size: 12px;
  color: var(--text-main, #cbd5e1);
  line-height: 1.5;
}

.syntax-single-col-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.syntax-doc-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 20px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
}

.doc-row-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.doc-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.doc-syntax-chip {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  padding: 2px 8px;
  background: var(--bg-input, #1e293b);
  color: #0284c7;
  border-radius: 4px;
}

.doc-summary-full {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.5;
}

.doc-examples-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 4px;
}

.example-full-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border-radius: 6px;
  gap: 16px;
}

.ex-left-content {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex: 1;
  flex-wrap: wrap;
}

.ex-code-wrap {
  min-width: 280px;
}

.ex-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: #10b981;
  font-weight: 600;
}

.ex-explain-full {
  font-size: 12px;
  color: var(--text-main, #cbd5e1);
  line-height: 1.45;
  flex: 1;
}

.ex-right-action {
  flex-shrink: 0;
}

.btn-use-ex {
  white-space: nowrap;
}

/* Generic Helpers */
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.text-muted {
  color: var(--text-muted, #94a3b8);
}

.text-success {
  color: #10b981;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.15s;
}

.btn-sm {
  padding: 4px 10px;
  font-size: 11px;
}

.btn-xs {
  padding: 3px 8px;
  font-size: 10px;
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
  width: 26px;
  height: 26px;
  border-radius: 4px;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.btn-icon:hover {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #f8fafc);
}

.btn-icon.delete:hover {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.btn-icon:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.btn-icon-subtle {
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  font-size: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
