<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { t } from '../composables/useSettings'
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
const currentSelectedConfig = ref<string>('config.dae')
const activeRunningConfig = ref<string>('config.dae')
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

// Helper: Check if rule is local private network bypass
function isPrivateBypassRule(r: ConfigRouteRule): boolean {
  if (!r || !r.condition) return false
  const c = r.condition.trim().toLowerCase()
  return c === 'dip(geoip:private)' || (c.includes('geoip:private') && r.target === 'direct')
}

function ensurePrivateBypassOnTop() {
  const rules = configModel.value.routingRules
  const pIdx = rules.findIndex(isPrivateBypassRule)
  if (pIdx > 0) {
    const [pRule] = rules.splice(pIdx, 1)
    rules.unshift(pRule)
  }
}

// Load Config Profiles and Active Model
async function fetchConfigProfiles() {
  try {
    const res = await fetch('/api/configs')
    if (res.ok) {
      const data = await res.json()
      configProfiles.value = data.configs || data.profiles || []
      activeRunningConfig.value = data.currentActive || data.activeName || 'config.dae'
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
      ensurePrivateBypassOnTop()
      isDirty.value = false
    }
    await fetchConfigProfiles()
  } catch (e: any) {
    alert(t('routing.load_failed', { error: e.message }))
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
  if (!confirm(t('routing.confirm_activate', { name: currentSelectedConfig.value }))) {
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
      alert(t('routing.activate_success', { message: data.message || '' }))
      await fetchConfigProfiles()
      await loadConfig(currentSelectedConfig.value)
    } else {
      alert(t('routing.activate_failed', { error: data.error || '' }))
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
      alert(t('routing.tmpl_sync_success', { message: data.message || '' }))
      await loadRoutingTemplatesFromAPI()
    } else {
      alert(t('routing.tmpl_sync_failed', { error: data.message || 'Error' }))
    }
  } catch (e: any) {
    alert(t('routing.tmpl_sync_failed', { error: e.message }))
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

function getBuilderPlaceholder(item: HumanizedConditionItem): string {
  if (item.targetType === 'domain') {
    return item.subType === 'geosite' ? t('routing.ph_domain_geosite') : t('routing.ph_domain_suffix')
  }
  if (item.targetType === 'dip') {
    return item.subType === 'geoip' ? t('routing.ph_dip_geoip') : t('routing.ph_dip_cidr')
  }
  if (item.targetType === 'dport') return t('routing.ph_dport')
  if (item.targetType === 'sip') return t('routing.ph_sip')
  if (item.targetType === 'mac') return t('routing.ph_mac')
  if (item.targetType === 'pname') return t('routing.ph_pname')
  return '...'
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

function isRuleDraggable(r: ConfigRouteRule, idx: number): boolean {
  if (!r || !r.condition) return false
  if (r.condition.trim() === 'fallback') return false
  if (idx === 0 && isPrivateBypassRule(r)) return false
  return true
}

function openAddRuleEditor() {
  activeTab.value = 'rules'
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
    alert(t('routing.condition_target_required'))
    return
  }

  const newRule: ConfigRouteRule = {
    condition: cond,
    target: finalTarget
  }

  if (editorMode.value === 'add') {
    // 自动放置在兜底规则 (fallback) 之上
    const fbIdx = configModel.value.routingRules.findIndex(r => r.condition.trim() === 'fallback')
    if (fbIdx >= 0) {
      configModel.value.routingRules.splice(fbIdx, 0, newRule)
    } else {
      configModel.value.routingRules.push(newRule)
    }
  } else if (editingIndex.value >= 0) {
    configModel.value.routingRules[editingIndex.value] = newRule
  }

  // Update live statement at the bottom
  if (cond === 'fallback') {
    lastGeneratedRuleStatement.value = `fallback: ${finalTarget}`
    lastGeneratedExplain.value = t('routing.explain_fallback', { target: finalTarget })
  } else {
    lastGeneratedRuleStatement.value = `${cond} -> ${finalTarget}`
    lastGeneratedExplain.value = t('routing.explain_match', { target: finalTarget })
  }

  isEditorOpen.value = false
  await syncModelToBackend()
}

function deleteRule(idx: number) {
  if (!confirm(t('routing.confirm_delete_rule', { index: idx + 1 }))) return
  configModel.value.routingRules.splice(idx, 1)
  syncModelToBackend()
}

// Drag & Drop free reordering (HTML5 Native Drag & Drop)
const draggedRuleIndex = ref<number | null>(null)
const dragOverTargetIndex = ref<number | null>(null)

function onRuleDragStart(idx: number, event: DragEvent) {
  const rule = configModel.value.routingRules[idx]
  if (!isRuleDraggable(rule, idx)) {
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

  // 若首行为局域网私网直连，则不能拖动到首行之前（最小索引为 1）
  const hasPrivateTop = rules.length > 0 && isPrivateBypassRule(rules[0])
  if (hasPrivateTop) {
    to = Math.max(1, to)
  }

  // 若末尾有 fallback 规则，则不能放置在 fallback 之后
  const fbIdx = rules.findIndex(r => r.condition.trim() === 'fallback')
  if (fbIdx >= 0 && to >= fbIdx) {
    to = Math.max(hasPrivateTop ? 1 : 0, fbIdx - 1)
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
            <h2>{{ t('routing.page_title') }}</h2>
            <span class="subtext">{{ t('routing.page_subtitle') }}</span>
          </div>
        </div>

        <!-- Config profile switcher -->
        <div class="profile-switch-box">
          <Files class="text-muted" :size="14" />
          <select
            :value="currentSelectedConfig"
            @change="loadConfig(($event.target as HTMLSelectElement).value)"
            class="profile-select"
            :title="t('routing.select_profile_tip')"
          >
            <option v-if="configProfiles.length === 0" :value="currentSelectedConfig">
              {{ currentSelectedConfig }}
            </option>
            <option
              v-for="p in configProfiles"
              :key="p.name"
              :value="p.name"
            >
              {{ p.name }} {{ (p.isActive || p.name === activeRunningConfig) ? ` [${t('common.active')}]` : '' }}
            </option>
          </select>

          <span
            v-if="currentSelectedConfig === activeRunningConfig"
            class="badge-active"
            :title="t('routing.active_profile_hint')"
          >
            <span class="active-dot"></span>{{ t('common.active') }}
          </span>
          <button
            v-else
            class="btn btn-sm btn-activate"
            :disabled="activatingConfig"
            @click="activateCurrentConfig"
            :title="t('routing.activate_btn_tip')"
          >
            <Check :size="12" />
            <span>{{ activatingConfig ? t('common.activating') : t('routing.btn_activate') }}</span>
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
            <span>{{ t('routing.tab_rules') }} ({{ configModel.routingRules.length }})</span>
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'templates' }"
            @click="activeTab = 'templates'"
          >
            <Sparkles :size="14" />
            <span>{{ t('routing.tab_templates') }} ({{ routingTemplates.length }})</span>
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'guide' }"
            @click="activeTab = 'guide'"
          >
            <BookOpen :size="14" />
            <span>{{ t('routing.tab_guide') }}</span>
          </button>
        </div>

        <button
          v-if="!isEditorOpen"
          class="btn btn-primary"
          @click="openAddRuleEditor"
          :title="t('routing.btn_add_tip')"
        >
          <Plus :size="14" /> {{ t('routing.btn_add') }}
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
              <span class="title">{{ t('routing.health_ok_title', { count: configModel.routingRules.length }) }}</span>
              <span class="desc">{{ t('routing.health_ok_desc') }}</span>
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
                    {{ t('routing.conflict_report_title') }}
                    <span class="head-pill">{{ t('routing.conflict_items_count', { count: routingHealthReport.conflicts.length }) }}</span>
                  </h4>
                  <span class="head-sub">{{ t('routing.conflict_report_desc') }}</span>
                </div>
              </div>
              <button class="btn-icon-subtle" :title="showConflictDetails ? t('common.collapse') : t('common.expand')">
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
                    {{ c.severity === 'error' ? t('routing.diag_error') : (c.severity === 'warning' ? t('routing.diag_warning') : t('routing.diag_suggestion')) }}
                  </span>
                  <strong class="diag-title">{{ c.title }}</strong>
                  <div v-if="c.ruleIndices.length > 0" class="diag-involved">
                    <span>{{ t('routing.diag_involved_lines') }}:</span>
                    <button
                      v-for="idx in c.ruleIndices"
                      :key="idx"
                      class="rule-pill-btn"
                      @click="highlightRule(idx)"
                      :title="t('routing.diag_highlight_tip')"
                    >
                      #{{ idx + 1 }}
                    </button>
                  </div>
                </div>

                <p class="diag-msg">{{ c.message }}</p>

                <div class="diag-sugg-box">
                  <Lightbulb :size="14" class="sugg-icon" />
                  <div class="sugg-text">
                    <strong>{{ t('routing.diag_sugg_label') }}:</strong>
                    <span>{{ c.suggestion }}</span>
                  </div>
                  <div class="sugg-actions">
                    <button
                      v-if="c.id === 'missing-private-bypass'"
                      class="btn btn-sm btn-quick-fix"
                      @click="fixMissingPrivateBypass"
                    >
                      <Wand2 :size="12" /> {{ t('routing.diag_fix_private') }}
                    </button>
                    <button
                      v-if="c.id === 'fallback-not-last'"
                      class="btn btn-sm btn-quick-fix"
                      @click="fixFallbackOrder"
                    >
                      <Wand2 :size="12" /> {{ t('routing.diag_fix_fallback') }}
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
              <h3>{{ editorMode === 'add' ? t('routing.editor_title_add') : t('routing.editor_title_edit_indexed', { index: editingIndex + 1 }) }}</h3>
              <span class="editor-badge">{{ editorMode === 'add' ? t('routing.editor_badge_add') : t('routing.editor_badge_edit') }}</span>
            </div>
            <button class="btn-icon-subtle" @click="isEditorOpen = false" :title="t('routing.editor_collapse')">
              &times;
            </button>
          </div>

          <div class="editor-body">
            <!-- Mode Switcher -->
            <div class="builder-nav-bar">
              <span class="nav-label">{{ t('routing.builder_mode_label') }}:</span>
              <div class="pill-switch">
                <button
                  type="button"
                  class="pill-btn"
                  :class="{ active: builderMode === 'builder' }"
                  @click="builderMode = 'builder'"
                >
                  <Wand2 :size="13" />
                  <span>🛠️ {{ t('routing.mode_builder') }}</span>
                </button>
                <button
                  type="button"
                  class="pill-btn"
                  :class="{ active: builderMode === 'custom' }"
                  @click="builderMode = 'custom'"
                >
                  <Code :size="13" />
                  <span>✏️ {{ t('routing.mode_custom') }}</span>
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
                      <label class="ctrl-lbl">{{ t('routing.builder_intent_label') }}:</label>
                      <select v-model="item.intent" class="form-select intent-select">
                        <option value="match">{{ t('routing.intent_match_full') }}</option>
                        <option value="exclude">{{ t('routing.intent_exclude_full') }}</option>
                      </select>
                    </div>

                    <!-- 2. Target Type -->
                    <div class="ctrl-group">
                      <label class="ctrl-lbl">{{ t('routing.builder_target_label') }}:</label>
                      <select v-model="item.targetType" class="form-select type-select">
                        <option value="domain">{{ t('routing.type_domain_opt') }}</option>
                        <option value="dip">{{ t('routing.type_dip_opt') }}</option>
                        <option value="dport">{{ t('routing.type_dport_opt') }}</option>
                        <option value="sip">{{ t('routing.type_sip_opt') }}</option>
                        <option value="mac">{{ t('routing.type_mac_opt') }}</option>
                        <option value="l4proto">{{ t('routing.type_l4proto_opt') }}</option>
                        <option value="ipversion">{{ t('routing.type_ipver_opt') }}</option>
                        <option value="pname">{{ t('routing.type_pname_opt') }}</option>
                      </select>
                    </div>

                    <!-- 3. Subtype modifiers -->
                    <div v-if="item.targetType === 'domain'" class="ctrl-group">
                      <label class="ctrl-lbl">{{ t('routing.builder_scope_label') }}:</label>
                      <select v-model="item.subType" class="form-select">
                        <option value="geosite">{{ t('routing.sub_geosite_opt') }}</option>
                        <option value="suffix">{{ t('routing.sub_suffix_opt') }}</option>
                        <option value="keyword">{{ t('routing.sub_keyword_opt') }}</option>
                        <option value="full">{{ t('routing.sub_full_opt') }}</option>
                        <option value="regex">{{ t('routing.sub_regex_opt') }}</option>
                        <option value="none">{{ t('routing.sub_domain_default_opt') }}</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'dip'" class="ctrl-group">
                      <label class="ctrl-lbl">{{ t('routing.builder_sub_method') }}:</label>
                      <select v-model="item.subType" class="form-select">
                        <option value="geoip">{{ t('routing.sub_geoip_opt') }}</option>
                        <option value="none">{{ t('routing.sub_ip_cidr_opt') }}</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'l4proto'" class="ctrl-group">
                      <label class="ctrl-lbl">{{ t('routing.builder_proto_label') }}:</label>
                      <select v-model="item.value" class="form-select">
                        <option value="tcp">{{ t('routing.proto_tcp_opt') }}</option>
                        <option value="udp">{{ t('routing.proto_udp_opt') }}</option>
                      </select>
                    </div>

                    <div v-else-if="item.targetType === 'ipversion'" class="ctrl-group">
                      <label class="ctrl-lbl">{{ t('routing.builder_ipver_label') }}:</label>
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
                      :title="t('routing.btn_del_condition')"
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
                        :placeholder="getBuilderPlaceholder(item)"
                      />
                    </div>

                    <!-- Fast Clickable Tags -->
                    <div class="fast-tags-wrap">
                      <span class="tags-hint">{{ t('routing.fast_tags_hint') }}:</span>
                      <template v-if="item.targetType === 'domain' && item.subType === 'geosite'">
                        <button type="button" class="tag-pill" @click="item.value = 'cn'">{{ t('routing.tag_cn_domain') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'google'">{{ t('routing.tag_google') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'youtube'">{{ t('routing.tag_youtube') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'category-ads-all'">{{ t('routing.tag_ads') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'github'">{{ t('routing.tag_github') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'netflix,disney'">{{ t('routing.tag_streaming') }}</button>
                      </template>
                      <template v-else-if="item.targetType === 'dip' && item.subType === 'geoip'">
                        <button type="button" class="tag-pill" @click="item.value = 'private'">{{ t('routing.tag_private_ip') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'cn'">{{ t('routing.tag_cn_ip') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = 'us'">{{ t('routing.tag_us_ip') }}</button>
                      </template>
                      <template v-else-if="item.targetType === 'dport'">
                        <button type="button" class="tag-pill" @click="item.value = '80, 443'">{{ t('routing.tag_web_port') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = '53'">{{ t('routing.tag_dns_port') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = '22'">{{ t('routing.tag_ssh_port') }}</button>
                        <button type="button" class="tag-pill" @click="item.value = '6881:6889'">{{ t('routing.tag_bt_port') }}</button>
                      </template>
                    </div>
                  </div>

                  <!-- "AND" connector indicator -->
                  <div v-if="bIdx < builderItems.length - 1" class="and-connector">
                    <span class="and-badge">{{ t('routing.builder_and_badge') }}</span>
                  </div>
                </div>
              </div>

              <div class="builder-bottom-actions">
                <button type="button" class="btn btn-secondary btn-sm" @click="addBuilderItem">
                  <Plus :size="13" /> {{ t('routing.builder_btn_add_and') }}
                </button>
              </div>
            </div>

            <!-- Custom Mode: Code Expression -->
            <div v-else class="custom-editor-box">
              <div class="form-group">
                <label>{{ t('routing.custom_expr_label') }}</label>
                <input
                  type="text"
                  v-model="editingRule.condition"
                  class="form-input mono"
                  :placeholder="t('routing.custom_expr_ph')"
                />
                <span class="field-hint">{{ t('routing.custom_expr_hint') }}</span>
              </div>
            </div>

            <!-- Target Selection Row -->
            <div class="target-select-row">
              <div class="form-group flex-1">
                <label>{{ t('routing.target_action_label') }}</label>
                <select v-model="editingRule.target" class="form-select target-select">
                  <optgroup :label="t('routing.optgroup_builtin')">
                    <option value="direct">{{ t('routing.target_opt_direct') }}</option>
                    <option value="must_direct">{{ t('routing.target_opt_must_direct') }}</option>
                    <option value="block">{{ t('routing.target_opt_block') }}</option>
                  </optgroup>
                  <optgroup :label="t('routing.optgroup_groups')">
                    <option v-for="g in configModel.groups" :key="g.name" :value="g.name">
                      {{ g.name }} ({{ t('routing.group_policy_prefix') }}: {{ g.policy }})
                    </option>
                  </optgroup>
                  <optgroup :label="t('routing.optgroup_advanced')">
                    <option value="__custom__">{{ t('routing.target_opt_custom') }}</option>
                  </optgroup>
                </select>
                <span class="field-hint">
                  {{ t('routing.target_hint_dns') }}
                </span>
              </div>

              <div v-if="editingRule.target === '__custom__'" class="form-group flex-1">
                <label>{{ t('routing.custom_target_label') }}</label>
                <input
                  type="text"
                  v-model="editingRule.customTarget"
                  class="form-input mono"
                  :placeholder="t('routing.custom_target_ph')"
                />
              </div>
            </div>

            <!-- Editor Buttons -->
            <div class="editor-footer-actions">
              <button class="btn btn-secondary" @click="isEditorOpen = false">{{ t('common.cancel') }}</button>
              <button class="btn btn-primary" @click="saveRuleFromEditor">
                <Check :size="14" /> {{ t('routing.btn_save_rule') }}
              </button>
            </div>
          </div>
        </div>

        <!-- 3. Rules Table Main Section -->
        <div class="rules-table-card">
          <div class="table-card-header">
            <div class="table-title">
              <h3>{{ t('routing.table_title_order') }}</h3>
              <span class="rules-count-pill">{{ t('routing.table_rules_count', { count: configModel.routingRules.length }) }}</span>
            </div>

            <div class="table-right-tools">
              <span class="text-muted table-tip">
                <Info :size="13" /> {{ t('routing.table_tip_priority') }}
              </span>
            </div>
          </div>

          <div v-if="configModel.routingRules.length === 0" class="empty-state-card">
            <GitFork class="empty-icon" :size="48" />
            <h4>{{ t('routing.empty_title') }}</h4>
            <p>{{ t('routing.empty_desc') }}</p>
            <button class="btn btn-primary btn-sm" @click="openAddRuleEditor">
              <Plus :size="13" /> {{ t('routing.btn_add_now') }}
            </button>
          </div>

          <div v-else class="table-responsive">
            <table class="routing-table">
              <thead>
                <tr>
                  <th style="width: 44px; text-align: center" :title="t('routing.col_drag_tip')">{{ t('routing.col_drag') }}</th>
                  <th style="width: 50px">{{ t('routing.col_order') }}</th>
                  <th>{{ t('routing.col_condition') }}</th>
                  <th style="width: 170px">{{ t('routing.col_target') }}</th>
                  <th style="width: 130px">{{ t('routing.col_status') }}</th>
                  <th style="width: 140px; text-align: right">{{ t('routing.col_action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(rule, idx) in configModel.routingRules"
                  :key="idx"
                  :id="`routing-table-row-${idx}`"
                  :draggable="isRuleDraggable(rule, idx)"
                  @dragstart="onRuleDragStart(idx, $event)"
                  @dragover="onRuleDragOver(idx, $event)"
                  @drop="onRuleDrop(idx, $event)"
                  @dragend="onRuleDragEnd"
                  :class="{
                    'row-highlighted': highlightedRuleIndex === idx,
                    'row-dragging': draggedRuleIndex === idx,
                    'row-drag-over': dragOverTargetIndex === idx,
                    'row-pinned-fixed': idx === 0 && isPrivateBypassRule(rule),
                    'row-fallback-fixed': rule.condition === 'fallback',
                    'row-error': routingHealthReport.ruleStatusMap[idx]?.severity === 'error',
                    'row-warning': routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'
                  }"
                >
                  <td class="drag-grip-col">
                    <div
                      v-if="idx === 0 && isPrivateBypassRule(rule)"
                      class="drag-handle-grip disabled"
                      :title="t('routing.drag_tip_pinned')"
                    >
                      <GripVertical :size="16" />
                    </div>
                    <div
                      v-else-if="rule.condition === 'fallback'"
                      class="drag-handle-grip disabled"
                      :title="t('routing.drag_tip_fallback')"
                    >
                      <GripVertical :size="16" />
                    </div>
                    <div
                      v-else
                      class="drag-handle-grip"
                      :title="t('routing.drag_tip_movable')"
                    >
                      <GripVertical :size="16" />
                    </div>
                  </td>
                  <td class="idx-col">
                    <span class="idx-chip">{{ idx + 1 }}</span>
                  </td>
                  <td class="cond-col mono">
                    <span v-if="idx === 0 && isPrivateBypassRule(rule)" class="pinned-rule-wrap">
                      <span class="rule-code-text">{{ rule.condition }}</span>
                      <span class="pinned-badge">{{ t('routing.badge_pinned_lan') }}</span>
                    </span>
                    <span v-else-if="rule.condition === 'fallback'" class="fallback-badge">
                      {{ t('routing.badge_fallback') }}
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
                      <ShieldAlert :size="12" /> {{ t('routing.status_conflict') }}
                    </span>
                    <span
                      v-else-if="routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'"
                      class="health-tag warning"
                      :title="routingHealthReport.ruleStatusMap[idx]?.messages.join('\n')"
                    >
                      <AlertTriangle :size="12" /> {{ t('routing.status_shadow') }}
                    </span>
                    <span v-else class="health-tag ok">
                      <Check :size="12" /> {{ t('routing.status_ok') }}
                    </span>
                  </td>
                  <td class="actions-col">
                    <button
                      class="btn-icon"
                      @click="duplicateRule(idx)"
                      :title="t('routing.btn_copy_rule')"
                    >
                      <Copy :size="13" />
                    </button>
                    <button
                      class="btn-icon"
                      @click="openEditRuleEditor(rule, idx)"
                      :title="t('routing.btn_edit_rule')"
                    >
                      <Edit2 :size="13" />
                    </button>
                    <button
                      class="btn-icon delete"
                      @click="deleteRule(idx)"
                      :title="t('routing.btn_del_rule')"
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
              <h4>{{ t('routing.stmt_title') }}</h4>
              <span class="subtext">{{ t('routing.stmt_desc') }}</span>
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
              <h3>{{ t('routing.tmpl_title') }}</h3>
              <span class="ver-tag">{{ t('routing.tmpl_version', { version: templateVersion }) }}</span>
            </div>
            <p class="desc">
              {{ t('routing.tmpl_desc') }}
            </p>
          </div>

          <div class="top-actions">
            <button
              class="btn btn-secondary"
              :disabled="templatesUpdating"
              @click="updateRoutingTemplatesFromAPI"
              :title="t('routing.tmpl_sync_btn_tip')"
            >
              <RefreshCw :size="14" :class="{ spin: templatesUpdating }" />
              <span>{{ templatesUpdating ? t('routing.tmpl_syncing') : t('routing.tmpl_btn_sync') }}</span>
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
              :placeholder="t('routing.tmpl_search_ph')"
              class="search-input"
            />
          </div>

          <div class="category-tabs">
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'all' }"
              @click="templateCategoryFilter = 'all'"
            >
              {{ t('routing.tmpl_cat_all') }} ({{ routingTemplates.length }})
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'common' }"
              @click="templateCategoryFilter = 'common'"
            >
              {{ t('routing.tmpl_cat_common') }}
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'security' }"
              @click="templateCategoryFilter = 'security'"
            >
              {{ t('routing.tmpl_cat_security') }}
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'media' }"
              @click="templateCategoryFilter = 'media'"
            >
              {{ t('routing.tmpl_cat_media') }}
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'dev' }"
              @click="templateCategoryFilter = 'dev'"
            >
              {{ t('routing.tmpl_cat_dev') }}
            </button>
            <button
              class="cat-pill"
              :class="{ active: templateCategoryFilter === 'advanced' }"
              @click="templateCategoryFilter = 'advanced'"
            >
              {{ t('routing.tmpl_cat_advanced') }}
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
                <span class="tmpl-target-badge">{{ t('routing.tmpl_recommended_target') }}: {{ tmpl.recommendedTarget }}</span>
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
                <Plus :size="13" /> {{ t('routing.tmpl_btn_apply') }}
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
            <h3>{{ t('routing.guide_title') }}</h3>
          </div>
          <p class="banner-desc">
            {{ t('routing.guide_desc') }}
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
                    :title="t('routing.guide_apply_tip')"
                  >
                    <Plus :size="12" /> {{ t('routing.guide_btn_apply') }}
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

.rules-view-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Diagnostics Bar */
.health-diagnostics-bar {
  display: flex;
  flex-direction: column;
  margin-bottom: 6px;
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
  padding: 8px 12px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 13px;
  outline: none;
  cursor: pointer;
  transition: all 0.2s;
}

.form-select:focus {
  border-color: #0284c7;
  box-shadow: 0 0 0 2px rgba(2, 132, 199, 0.2);
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
  box-sizing: border-box;
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
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 13px;
  outline: none;
  transition: all 0.2s;
  box-sizing: border-box;
}

.form-input:focus {
  border-color: #0284c7;
  box-shadow: 0 0 0 2px rgba(2, 132, 199, 0.2);
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

.routing-table tr.row-pinned-fixed {
  background: rgba(2, 132, 199, 0.04) !important;
}

.routing-table tr.row-pinned-fixed:hover {
  background: rgba(2, 132, 199, 0.08) !important;
}

.pinned-rule-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pinned-badge {
  font-size: 11px;
  font-weight: 600;
  color: #0284c7;
  background: rgba(2, 132, 199, 0.12);
  border: 1px solid rgba(2, 132, 199, 0.3);
  padding: 1px 7px;
  border-radius: 10px;
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
  margin-top: 14px;
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
