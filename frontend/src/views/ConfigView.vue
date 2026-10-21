<script setup lang="ts">
import { ref, onMounted, shallowRef, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { t } from '../composables/useSettings'
import {
  Save,
  CheckCircle2,
  AlertCircle,
  Download,
  Upload,
  RotateCcw,
  Archive,
  Trash2,
  FileCode,
  ShieldCheck,
  RefreshCw,
  Plus,
  Edit2,
  Sliders,
  Server,
  Layers,
  GitFork,
  Code,
  LayoutGrid,
  Files,
  Check,
  Copy,
  ListFilter,
  CheckSquare,
  ArrowUp,
  ArrowDown,
  ArrowLeft,
  FileText,
  AlertTriangle,
  ShieldAlert,
  BookOpen,
  Lightbulb,
  HelpCircle,
  Crosshair,
  Wand2,
  Sparkles,
  ExternalLink,
  ChevronDown,
  Info,
  Compass,
  Globe,
  Shield,
  Zap,
  Eye,
  Flag,
  Radio,
  Search,
  X
} from 'lucide-vue-next'
import {
  analyzeRoutingConflicts,
  type RoutingHealthReport,
  type RoutingConflict,
  type ConflictSeverity
} from '../utils/routingAnalyzer'
import {
  ROUTING_PRESETS,
  ROUTING_SYNTAX_GUIDE,
  type RoutingPreset,
  type SyntaxDocSection
} from '../utils/routingPresets'
import { EditorState, StateField, StateEffect } from '@codemirror/state'
import { EditorView, lineNumbers, keymap, Decoration, type DecorationSet } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'

interface BackupItem {
  name: string
  size: number
  modTime: string
}

interface ConfigProfile {
  name: string
  path: string
  isActive: boolean
  isValid?: boolean
  errorMsg?: string
  note: string
  rulesCount: number
  nodesCount: number
  size: number
  modTime: string
}

interface ConfigSubItem {
  tag: string
  url: string
}

interface ConfigNodeItem {
  key: string
  url: string
  protocol: string
  name: string
  address: string
  port: string
}

interface ConfigGroupItem {
  name: string
  policy: string
  filters: string[]
}

interface ConfigRouteRule {
  condition: string
  target: string
}

interface DnsUpstreamItem {
  tag: string
  address: string
}

interface DnsRequestRule {
  condition: string
  target: string
}

interface DnsConfig {
  ipVersionPrefer: number
  fixedDomainTTL: number
  optimisticCache: boolean
  upstreams: DnsUpstreamItem[]
  requestRules: DnsRequestRule[]
  fallback: string
}

interface DaeConfigModel {
  globals: Record<string, string>
  subscriptions: ConfigSubItem[]
  nodes: ConfigNodeItem[]
  dns: DnsConfig
  groups: ConfigGroupItem[]
  routingRules: ConfigRouteRule[]
  rawDnsBlock: string
  rawOtherLines?: string[]
}

type Mode = 'form' | 'code'
type FormSection = 'global' | 'subscriptions' | 'nodes' | 'dns' | 'groups' | 'routing'

const viewLevel = ref<'list' | 'detail'>('list')
const currentMode = ref<Mode>('form')
const activeSection = ref<FormSection>('global')

const editorContainer = ref<HTMLDivElement | null>(null)
const editorView = shallowRef<EditorView | null>(null)

function backToProfilesList() {
  if (editorView.value) {
    editorView.value.destroy()
    editorView.value = null
  }
  viewLevel.value = 'list'
}

onBeforeUnmount(() => {
  if (editorView.value) {
    editorView.value.destroy()
    editorView.value = null
  }
})

const configContent = ref('')
const configPath = ref('')
const lastModTime = ref('')
const isDirty = ref(false)
const loading = ref(false)
const saving = ref(false)
const validating = ref(false)

const configModel = ref<DaeConfigModel>({
  globals: {},
  subscriptions: [],
  nodes: [],
  dns: {
    ipVersionPrefer: 4,
    fixedDomainTTL: 0,
    optimisticCache: false,
    upstreams: [
      { tag: 'alidns', address: 'udp://223.5.5.5:53' },
      { tag: 'googledns', address: 'tcp+udp://dns.google:53' }
    ],
    requestRules: [
      { condition: 'qname(geosite:cn)', target: 'alidns' }
    ],
    fallback: 'googledns'
  },
  groups: [],
  routingRules: [],
  rawDnsBlock: ''
})

const validationStatus = ref<{
  checked: boolean
  valid: boolean
  message: string
}>({
  checked: false,
  valid: true,
  message: ''
})

const autoBackup = ref(true)
const reloadAfterSave = ref(false)
const showBackupModal = ref(false)
const backups = ref<BackupItem[]>([])
const newBackupLabel = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

// Config profile management
const configProfiles = ref<ConfigProfile[]>([])
const currentSelectedConfig = ref<string>('config.dae')
const activeRunningConfig = ref<string>('config.dae')
const showNewProfileModal = ref(false)
const newProfileName = ref('')
const newProfileNote = ref('')
const activatingConfig = ref(false)
const profileFileInput = ref<HTMLInputElement | null>(null)
const importingProfile = ref(false)
const showSyntaxErrorModal = ref(false)
const currentErrorProfile = ref<ConfigProfile | null>(null)
const showDnsSyntaxDoc = ref(false)

// CodeMirror Error Line Decoration
const setErrorLineEffect = StateEffect.define<number | null>()

const errorLineField = StateField.define<DecorationSet>({
  create() {
    return Decoration.none
  },
  update(decorations, tr) {
    decorations = decorations.map(tr.changes)
    for (const e of tr.effects) {
      if (e.is(setErrorLineEffect)) {
        if (e.value === null || e.value <= 0 || e.value > tr.state.doc.lines) {
          decorations = Decoration.none
        } else {
          const line = tr.state.doc.line(e.value)
          decorations = Decoration.set([
            Decoration.line({
              attributes: { class: 'cm-error-line' }
            }).range(line.from)
          ])
        }
      }
    }
    return decorations
  },
  provide: f => EditorView.decorations.from(f)
})

function findErrorLineInContent(content: string, msg: string): { line: number; col?: number; message: string } | null {
  if (!msg) return null
  const lines = (content || '').split('\n')

  // 1. ANTLR syntax error: "line X:Y" or "line X"
  const lineMatch = msg.match(/(?:line|第)\s*(\d+)(?::(\d+))?/i)
  if (lineMatch && lineMatch[1]) {
    let line = parseInt(lineMatch[1], 10)
    const col = lineMatch[2] ? parseInt(lineMatch[2], 10) : undefined
    if (!isNaN(line) && line > 0) {
      if (lines.length > 0) {
        if (line > lines.length) line = lines.length
        const curLineText = (lines[line - 1] || '').trim()
        // If ANTLR points to closing brace '}' and previous line was incomplete:
        if (curLineText === '}' || curLineText.startsWith('}')) {
          for (let i = line - 2; i >= 0; i--) {
            const prev = (lines[i] || '').trim()
            if (prev && !prev.startsWith('#')) {
              if (prev.endsWith('->') || prev.endsWith('&&') || prev.endsWith(',') || prev.endsWith('(') || prev.endsWith(':') || msg.includes("mismatched input '}'") || msg.includes('mismatched input \'}\'')) {
                line = i + 1
              }
              break
            }
          }
        }
      }
      return { line, col, message: msg }
    }
  }

  // 2. unexpected key: <key>
  const keyMatch = msg.match(/unexpected key:\s*([a-zA-Z0-9_\-]+)/i)
  if (keyMatch && keyMatch[1] && lines.length > 0) {
    const key = keyMatch[1]
    for (let i = 0; i < lines.length; i++) {
      const lineStr = lines[i]
      if (new RegExp('^\\s*' + key + '\\s*:').test(lineStr) || lineStr.includes(key)) {
        return { line: i + 1, message: msg }
      }
    }
  }

  // 3. Quoted tokens: e.g. outbound (group) "xxx" not found, or failed to parse "xxx"
  if (lines.length > 0) {
    const quotedMatches: string[] = []
    const regex = /["']([^"'\s]{2,})["']/g
    let qm: RegExpExecArray | null
    while ((qm = regex.exec(msg)) !== null) {
      const token = qm[1]
      if (!['global', 'subscription', 'node', 'dns', 'group', 'routing', 'true', 'false', 'info', 'warn', 'error', 'debug'].includes(token.toLowerCase())) {
        quotedMatches.push(token)
      }
    }
    for (const token of quotedMatches) {
      for (let i = 0; i < lines.length; i++) {
        if (lines[i].includes(token)) {
          return { line: i + 1, message: msg }
        }
      }
    }
  }

  // 4. Section fallback (e.g. failed to parse "global")
  if (lines.length > 0) {
    const secMatch = msg.match(/(?:failed to parse|section)\s*["']?([a-zA-Z]+)["']?/i)
    if (secMatch && secMatch[1]) {
      const sec = secMatch[1].toLowerCase()
      for (let i = 0; i < lines.length; i++) {
        if (new RegExp('^\\s*' + sec + '\\s*\\{').test(lines[i])) {
          return { line: i + 1, message: msg }
        }
      }
    }
  }

  return { line: 1, message: msg }
}

const activeSyntaxError = ref<{ line: number; col?: number; message: string } | null>(null)

const errorLineInfo = computed<{ line: number; col?: number; message: string } | null>(() => {
  if (activeSyntaxError.value) {
    return activeSyntaxError.value
  }
  let msg = ''
  if (!validationStatus.value.valid && validationStatus.value.checked) {
    msg = validationStatus.value.message || ''
  } else {
    const prof = configProfiles.value.find(p => p.name === currentSelectedConfig.value)
    if (prof && prof.isValid === false && prof.errorMsg) {
      msg = prof.errorMsg
    }
  }
  if (!msg) return null
  return findErrorLineInContent(configContent.value, msg)
})

const erroneousSection = computed<FormSection | null>(() => {
  if (validationStatus.value.valid && validationStatus.value.checked && !activeSyntaxError.value) return null
  let msg = activeSyntaxError.value?.message || ''
  if (!msg) {
    if (!validationStatus.value.valid && validationStatus.value.checked) {
      msg = validationStatus.value.message || ''
    } else {
      const prof = configProfiles.value.find(p => p.name === currentSelectedConfig.value)
      if (prof && prof.isValid === false && prof.errorMsg) {
        msg = prof.errorMsg
      }
    }
  }
  if (!msg) return null

  const errLine = errorLineInfo.value || findErrorLineInContent(configContent.value, msg)
  if (errLine && configContent.value) {
    const lines = configContent.value.split('\n')
    let currentSection: FormSection | null = null
    let braceDepth = 0
    for (let i = 0; i < lines.length && i < errLine.line; i++) {
      const line = lines[i].trim()
      if (braceDepth === 0) {
        if (/^global\s*\{/.test(line)) currentSection = 'global'
        else if (/^subscription\s*\{/.test(line)) currentSection = 'subscriptions'
        else if (/^node\s*\{/.test(line)) currentSection = 'nodes'
        else if (/^dns\s*\{/.test(line)) currentSection = 'dns'
        else if (/^group\s*\{/.test(line)) currentSection = 'groups'
        else if (/^routing\s*\{/.test(line)) currentSection = 'routing'
      }
      for (const ch of line) {
        if (ch === '{') braceDepth++
        else if (ch === '}') braceDepth = Math.max(0, braceDepth - 1)
      }
    }
    if (currentSection) return currentSection
  }

  const lower = msg.toLowerCase()
  if (lower.includes('global') || lower.includes('tproxy') || lower.includes('dial_mode') || lower.includes('lan_interface')) return 'global'
  if (lower.includes('subscription')) return 'subscriptions'
  if (lower.includes('node') || lower.includes('link:')) return 'nodes'
  if (lower.includes('dns') || lower.includes('upstream') || lower.includes('qname') || lower.includes('qtype')) return 'dns'
  if (lower.includes('group') || lower.includes('policy')) return 'groups'
  if (lower.includes('routing') || lower.includes('route') || lower.includes('dip(') || lower.includes('sip(') || lower.includes('dport(')) return 'routing'

  return null
})

function highlightErrorLineInEditor(lineNum?: number | null) {
  const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
  if (!isAttached) {
    if (editorContainer.value) {
      initEditor(configContent.value)
    } else {
      return
    }
  }
  if (!editorView.value) return

  const target = lineNum || errorLineInfo.value?.line || activeSyntaxError.value?.line
  if (!target || target <= 0) {
    editorView.value.dispatch({
      effects: setErrorLineEffect.of(null)
    })
    return
  }
  const doc = editorView.value.state.doc
  const validLine = Math.min(Math.max(1, target), doc.lines)
  const line = doc.line(validLine)
  editorView.value.dispatch({
    effects: setErrorLineEffect.of(validLine),
    selection: { anchor: line.from },
    scrollIntoView: true
  })
}

async function switchToCodeAndLocate(targetLine?: number) {
  if (validationStatus.value.valid !== false && !activeSyntaxError.value) {
    await syncModelToCode()
  }
  currentMode.value = 'code'
  await nextTick()
  const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
  if (!isAttached && editorContainer.value) {
    initEditor(configContent.value)
  }
  const lineToJump = targetLine || errorLineInfo.value?.line || activeSyntaxError.value?.line || 1
  highlightErrorLineInEditor(lineToJump)
}

function quickInsertDnsRule(condition: string, preferredTarget?: string) {
  if (!configModel.value.dns) return
  let target = preferredTarget || configModel.value.dns.fallback || configModel.value.dns.upstreams[0]?.tag || 'alidns'
  if (!configModel.value.dns.upstreams.some(u => u.tag === target)) {
    target = configModel.value.dns.upstreams[0]?.tag || 'alidns'
  }
  configModel.value.dns.requestRules.push({ condition, target })
  isDirty.value = true
}

function triggerProfileImport() {
  profileFileInput.value?.click()
}

async function handleProfileFileImport(e: Event) {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  importingProfile.value = true
  try {
    const formData = new FormData()
    formData.append('file', file)

    const res = await fetch('/api/configs/import', {
      method: 'POST',
      body: formData
    })
    const data = await res.json()
    if (res.ok && data.ok) {
      if (data.configs) {
        configProfiles.value = data.configs
      } else {
        await fetchConfigProfiles()
      }

      if (data.valid) {
        alert(t('config.import_success_valid', { name: data.name }))
      } else {
        const found = configProfiles.value.find(p => p.name === data.name)
        if (found) {
          openErrorModal(found)
        } else {
          currentErrorProfile.value = {
            name: data.name,
            path: '',
            isActive: false,
            isValid: false,
            errorMsg: data.errorMsg,
            note: '',
            rulesCount: 0,
            nodesCount: 0,
            size: file.size,
            modTime: ''
          }
          showSyntaxErrorModal.value = true
        }
        alert(t('config.import_warn_invalid', { name: data.name }))
      }
    } else {
      alert(data.error || '导入配置文件失败')
    }
  } catch (err: any) {
    alert('导入请求失败: ' + err.message)
  } finally {
    importingProfile.value = false
    if (target) target.value = ''
  }
}

function downloadProfile(name: string) {
  window.open(`/api/configs/${encodeURIComponent(name)}/download`, '_blank')
}

function openErrorModal(profile: ConfigProfile) {
  currentErrorProfile.value = profile
  showSyntaxErrorModal.value = true
}

const currentProfileNote = computed(() => {
  const p = configProfiles.value.find(item => item.name === currentSelectedConfig.value)
  return p?.note || ''
})

// Batch profile selection & deletion
const selectedProfileNames = ref<string[]>([])
const batchDeleting = ref(false)

const selectableProfiles = computed(() => {
  return configProfiles.value.filter(p => !p.isActive)
})

const isAllSelected = computed(() => {
  if (selectableProfiles.value.length === 0) return false
  return selectableProfiles.value.every(p => selectedProfileNames.value.includes(p.name))
})

const isIndeterminate = computed(() => {
  const count = selectedProfileNames.value.length
  return count > 0 && !isAllSelected.value
})

function toggleSelectAllProfiles() {
  if (isAllSelected.value) {
    selectedProfileNames.value = []
  } else {
    selectedProfileNames.value = selectableProfiles.value.map(p => p.name)
  }
}

// Modal states for CRUD
const showNodeModal = ref(false)
const nodeModalMode = ref<'add' | 'edit'>('add')
const editingNode = ref<ConfigNodeItem>({
  key: '',
  url: '',
  protocol: '',
  name: '',
  address: '',
  port: ''
})

// Overview nodes and subscriptions for reactive group linkage
interface OverviewNodeItem {
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

interface OverviewSubItem {
  tag: string
  url: string
  nodeCount: number
}

const overviewNodes = ref<OverviewNodeItem[]>([])
const overviewSubscriptions = ref<OverviewSubItem[]>([])
const loadingOverview = ref(false)

async function fetchNodesOverview() {
  loadingOverview.value = true
  try {
    const res = await fetch('/api/nodes/overview')
    if (res.ok) {
      const data = await res.json()
      overviewNodes.value = data.nodes || []
      overviewSubscriptions.value = data.subscriptions || []
    }
  } catch (e) {
    console.error('Failed to fetch nodes overview in config view:', e)
  } finally {
    loadingOverview.value = false
  }
}

const availableSubscriptions = computed(() => {
  const map = new Map<string, { tag: string; url: string; nodeCount: number }>()
  for (const s of overviewSubscriptions.value) {
    map.set(s.tag, { tag: s.tag, url: s.url, nodeCount: s.nodeCount || 0 })
  }
  for (const s of configModel.value.subscriptions) {
    if (!map.has(s.tag)) {
      map.set(s.tag, { tag: s.tag, url: s.url, nodeCount: 0 })
    }
  }
  return Array.from(map.values())
})

const availableNodes = computed(() => {
  const map = new Map<string, OverviewNodeItem>()
  for (const n of overviewNodes.value) {
    map.set(n.key, n)
  }
  for (const n of configModel.value.nodes) {
    if (!map.has(n.key)) {
      map.set(n.key, {
        key: n.key,
        name: n.name || n.key,
        protocol: n.protocol || 'TCP',
        address: n.address || '',
        port: n.port || '',
        country: '自建',
        flag: '🛡️',
        active: true,
        latency: 15
      })
    }
  }
  return Array.from(map.values())
})

interface EditingGroupState {
  name: string
  sourceType: 'sub' | 'nodes' | 'custom'
  selectedSubTag: string
  selectedNodeKeys: string[]
  policyMode: 'min_moving_avg' | 'min_avg10' | 'min' | 'random' | 'fixed'
  fixedNodeIndex: number
  customFilterText: string
}

const showGroupModal = ref(false)
const groupModalMode = ref<'add' | 'edit'>('add')
const nodeSearchQuery = ref('')

const editingGroup = ref<EditingGroupState>({
  name: '',
  sourceType: 'sub',
  selectedSubTag: '',
  selectedNodeKeys: [],
  policyMode: 'min_moving_avg',
  fixedNodeIndex: 0,
  customFilterText: ''
})

const groupCandidateNodes = computed(() => {
  if (editingGroup.value.sourceType === 'sub') {
    const sub = editingGroup.value.selectedSubTag.toLowerCase()
    if (!sub) return []
    const matched = availableNodes.value.filter(n => {
      const k = n.key.toLowerCase()
      const nm = n.name.toLowerCase()
      return k.includes(sub) || nm.includes(sub)
    })
    if (matched.length === 0 && availableSubscriptions.value.length === 1) {
      return availableNodes.value
    }
    return matched
  } else if (editingGroup.value.sourceType === 'nodes') {
    const keys = editingGroup.value.selectedNodeKeys
    return keys.map(k => availableNodes.value.find(n => n.key === k)).filter(Boolean) as OverviewNodeItem[]
  } else {
    return availableNodes.value
  }
})

const filteredAllNodes = computed(() => {
  const q = nodeSearchQuery.value.trim().toLowerCase()
  if (!q) return availableNodes.value
  return availableNodes.value.filter(n =>
    n.name.toLowerCase().includes(q) ||
    n.key.toLowerCase().includes(q) ||
    n.protocol.toLowerCase().includes(q) ||
    n.address.toLowerCase().includes(q)
  )
})

const generatedPolicyString = computed(() => {
  if (editingGroup.value.policyMode === 'fixed') {
    return `fixed(${editingGroup.value.fixedNodeIndex})`
  }
  return editingGroup.value.policyMode
})

const generatedFilters = computed(() => {
  if (editingGroup.value.sourceType === 'sub') {
    if (editingGroup.value.selectedSubTag) {
      return [`subtag(${editingGroup.value.selectedSubTag})`]
    }
    return []
  } else if (editingGroup.value.sourceType === 'nodes') {
    if (editingGroup.value.selectedNodeKeys.length > 0) {
      return [`name(${editingGroup.value.selectedNodeKeys.join(', ')})`]
    }
    return []
  } else {
    return editingGroup.value.customFilterText.split('\n').map(s => s.trim().replace(/^filter:\s*/, '')).filter(Boolean)
  }
})

const groupDaePreview = computed(() => {
  const name = editingGroup.value.name.trim() || 'my_group'
  const pol = generatedPolicyString.value
  const filts = generatedFilters.value.map(f => `        filter: ${f}`).join('\n') || '        # 未设定过滤规则'
  return `group {\n    ${name} {\n${filts}\n        policy: ${pol}\n    }\n}`
})

function toggleSelectNodeKey(key: string) {
  const idx = editingGroup.value.selectedNodeKeys.indexOf(key)
  if (idx >= 0) {
    editingGroup.value.selectedNodeKeys.splice(idx, 1)
  } else {
    editingGroup.value.selectedNodeKeys.push(key)
  }
  if (editingGroup.value.fixedNodeIndex >= editingGroup.value.selectedNodeKeys.length) {
    editingGroup.value.fixedNodeIndex = 0
  }
}

function selectAllFilteredNodes() {
  const keys = filteredAllNodes.value.map(n => n.key)
  const set = new Set([...editingGroup.value.selectedNodeKeys, ...keys])
  editingGroup.value.selectedNodeKeys = Array.from(set)
}

function clearSelectedNodes() {
  editingGroup.value.selectedNodeKeys = []
  editingGroup.value.fixedNodeIndex = 0
}

const showRouteModal = ref(false)
const routeModalMode = ref<'add' | 'edit'>('add')
const routeModalTab = ref<'form' | 'guide'>('form')
const routeInputMode = ref<'preset' | 'builder' | 'custom'>('preset')
const selectedPresetId = ref<string>('lan-private')

interface ConditionBuilderItem {
  not: boolean
  type: 'domain' | 'dip' | 'dport' | 'sip' | 'sport' | 'l4proto' | 'ipversion' | 'pname' | 'mac'
  modifier: string
  value: string
}

const builderItems = ref<ConditionBuilderItem[]>([
  { not: false, type: 'domain', modifier: 'geosite', value: 'cn' }
])

const editingRoute = ref<{
  condition: string
  target: string
  customTarget: string
  index: number
}>({
  condition: '',
  target: 'proxy',
  customTarget: '',
  index: -1
})

// Conflict diagnosis
const showConflictDetails = ref(true)
const highlightedRuleIndex = ref<number | null>(null)

const routingHealthReport = computed<RoutingHealthReport>(() => {
  const groupNames = configModel.value.groups.map(g => g.name)
  return analyzeRoutingConflicts(configModel.value.routingRules, groupNames)
})

const showSubModal = ref(false)
const editingSub = ref<ConfigSubItem>({ tag: '', url: '' })

// Initialize CodeMirror editor
function initEditor(content: string) {
  if (!editorContainer.value) return

  if (editorView.value) {
    editorView.value.destroy()
    editorView.value = null
  }
  editorContainer.value.innerHTML = ''

  const textToLoad = content ?? configContent.value ?? ''

  const startState = EditorState.create({
    doc: textToLoad,
    extensions: [
      lineNumbers(),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      errorLineField,
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          configContent.value = update.state.doc.toString()
          isDirty.value = true
          if (activeSyntaxError.value) {
            const rechecked = findErrorLineInContent(configContent.value, activeSyntaxError.value.message)
            if (rechecked) {
              activeSyntaxError.value = rechecked
              if (editorView.value && rechecked.line) {
                const doc = editorView.value.state.doc
                const validLine = Math.min(rechecked.line, doc.lines)
                editorView.value.dispatch({ effects: setErrorLineEffect.of(validLine) })
              }
            }
          }
        }
      }),
      EditorView.theme({
        '&': {
          height: '100%',
          fontSize: '13px',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
          backgroundColor: 'var(--bg-card, #0f172a)',
          color: 'var(--text-main, #e2e8f0)',
        },
        '.cm-content': { padding: '12px 0' },
        '.cm-line': { padding: '0 16px' },
        '.cm-gutters': {
          backgroundColor: 'var(--bg-main, #090d16)',
          color: 'var(--text-muted, #475569)',
          borderRight: '1px solid var(--border-color, #1e293b)',
        },
        '.cm-activeLine': { backgroundColor: 'var(--bg-input, #1e293b55)' },
        '.cm-activeLineGutter': { backgroundColor: 'var(--bg-input, #1e293b)', color: 'var(--text-main, #94a3b8)' },
      })
    ]
  })

  editorView.value = new EditorView({
    state: startState,
    parent: editorContainer.value
  })

  if (errorLineInfo.value?.line) {
    setTimeout(() => {
      highlightErrorLineInEditor(errorLineInfo.value?.line)
    }, 40)
  }
}

function setEditorText(text: string) {
  configContent.value = text
  isDirty.value = false
  const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
  if (!isAttached) {
    if (editorContainer.value) {
      initEditor(text)
    }
    return
  }
  const curr = editorView.value!.state.doc.toString()
  if (curr !== text) {
    editorView.value!.dispatch({
      changes: { from: 0, to: curr.length, insert: text }
    })
  }
}

async function fetchConfigProfiles() {
  try {
    const res = await fetch('/api/configs')
    if (res.ok) {
      const data = await res.json()
      configProfiles.value = data.configs || []
      activeRunningConfig.value = data.currentActive || 'config.dae'
    }
  } catch (e) {
    console.error('Fetch config profiles failed:', e)
  }
}

async function loadConfig(profileName?: string) {
  loading.value = true
  if (profileName) {
    currentSelectedConfig.value = profileName
  }
  const target = currentSelectedConfig.value
  try {
    const [resRaw, resModel] = await Promise.all([
      fetch(`/api/config?name=${encodeURIComponent(target)}`),
      fetch(`/api/config/model?name=${encodeURIComponent(target)}`)
    ])
    if (resRaw.ok) {
      const data = await resRaw.json()
      configPath.value = data.path
      lastModTime.value = data.modTime
      configContent.value = data.content
      setEditorText(data.content)
      const prof = configProfiles.value.find(p => p.name === target)
      if (prof && prof.isValid === false && prof.errorMsg) {
        activeSyntaxError.value = findErrorLineInContent(data.content, prof.errorMsg)
        validationStatus.value = {
          checked: true,
          valid: false,
          message: prof.errorMsg
        }
      }
      await validateContent(data.content, false)
    }
    if (resModel.ok) {
      const m = await resModel.json()
      configModel.value = {
        globals: m.globals || {},
        subscriptions: m.subscriptions || [],
        nodes: m.nodes || [],
        dns: m.dns || {
          ipVersionPrefer: 4,
          fixedDomainTTL: 0,
          optimisticCache: false,
          upstreams: [
            { tag: 'alidns', address: 'udp://223.5.5.5:53' },
            { tag: 'googledns', address: 'tcp+udp://dns.google:53' }
          ],
          requestRules: [
            { condition: 'qname(geosite:cn)', target: 'alidns' }
          ],
          fallback: 'googledns'
        },
        groups: m.groups || [],
        routingRules: m.routingRules || [],
        rawDnsBlock: m.rawDnsBlock || '',
        rawOtherLines: m.rawOtherLines || []
      }
    }
    await fetchConfigProfiles()
    await fetchNodesOverview()
    isDirty.value = false
  } catch (err: any) {
    alert(t('config.load_fail', { error: err.message }))
  } finally {
    loading.value = false
  }
}

async function openEditProfile(name: string, targetTab?: FormSection, jumpToCode?: boolean) {
  currentSelectedConfig.value = name
  viewLevel.value = 'detail'
  if (jumpToCode) {
    currentMode.value = 'code'
  } else {
    currentMode.value = 'form'
    if (targetTab) {
      activeSection.value = targetTab
    }
  }
  await nextTick()
  await loadConfig(name)
  await nextTick()
  if (jumpToCode || currentMode.value === 'code') {
    const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
    if (!isAttached && editorContainer.value) {
      initEditor(configContent.value)
    }
    const lineToHighlight = errorLineInfo.value?.line || activeSyntaxError.value?.line
    if (lineToHighlight) {
      highlightErrorLineInEditor(lineToHighlight)
    }
  } else {
    if (targetTab) {
      activeSection.value = targetTab
    } else if (erroneousSection.value) {
      activeSection.value = erroneousSection.value
    } else {
      activeSection.value = 'global'
    }
  }
}

function addDnsUpstream(tag?: string, address?: string) {
  if (!configModel.value.dns) {
    configModel.value.dns = {
      ipVersionPrefer: 4,
      fixedDomainTTL: 0,
      optimisticCache: false,
      upstreams: [],
      requestRules: [],
      fallback: ''
    }
  }
  const newTag = tag || `dns_${configModel.value.dns.upstreams.length + 1}`
  const newAddr = address || 'udp://223.5.5.5:53'
  let finalTag = newTag
  if (configModel.value.dns.upstreams.some(u => u.tag === finalTag)) {
    finalTag = `${newTag}_${Date.now() % 1000}`
  }
  configModel.value.dns.upstreams.push({ tag: finalTag, address: newAddr })
  if (!configModel.value.dns.fallback) {
    configModel.value.dns.fallback = finalTag
  }
  isDirty.value = true
}

function removeDnsUpstream(index: number) {
  if (!configModel.value.dns) return
  const removed = configModel.value.dns.upstreams[index]
  configModel.value.dns.upstreams.splice(index, 1)
  if (configModel.value.dns.fallback === removed?.tag) {
    configModel.value.dns.fallback = configModel.value.dns.upstreams[0]?.tag || ''
  }
  isDirty.value = true
}

function addDnsRequestRule(condition?: string, target?: string) {
  if (!configModel.value.dns) return
  const fallbackTarget = configModel.value.dns.upstreams[0]?.tag || 'alidns'
  configModel.value.dns.requestRules.push({
    condition: condition || 'qname(geosite:cn)',
    target: target || fallbackTarget
  })
  isDirty.value = true
}

function removeDnsRequestRule(index: number) {
  if (!configModel.value.dns) return
  configModel.value.dns.requestRules.splice(index, 1)
  isDirty.value = true
}

function moveDnsRequestRule(index: number, direction: 'up' | 'down') {
  if (!configModel.value.dns) return
  const rules = configModel.value.dns.requestRules
  const targetIndex = direction === 'up' ? index - 1 : index + 1
  if (targetIndex < 0 || targetIndex >= rules.length) return
  const temp = rules[index]
  rules[index] = rules[targetIndex]
  rules[targetIndex] = temp
  isDirty.value = true
}

async function handleSelectProfile(name: string) {
  if (isDirty.value) {
    if (!confirm(t('config.confirm_unsaved_switch'))) {
      return
    }
  }
  currentSelectedConfig.value = name
  await loadConfig(name)
}

async function activateSelectedProfile(name?: string) {
  const target = name || currentSelectedConfig.value
  const targetProfile = configProfiles.value.find(p => p.name === target)
  if (targetProfile && targetProfile.isValid === false) {
    alert(t('config.cannot_activate_invalid', { name: target, error: targetProfile.errorMsg || '语法校验未通过' }))
    openErrorModal(targetProfile)
    return
  }
  if (!confirm(t('config.confirm_activate', { name: target }))) {
    return
  }
  activatingConfig.value = true
  try {
    const res = await fetch('/api/configs/activate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: target })
    })
    const data = await res.json()
    if (res.ok) {
      if (data.configs) {
        configProfiles.value = data.configs
      }
      activeRunningConfig.value = data.currentActive || target
      selectedProfileNames.value = selectedProfileNames.value.filter(n => n !== target)
      alert(t('config.activate_success', { message: data.message || '' }))
      await fetchConfigProfiles()
      if (viewLevel.value === 'detail') {
        await loadConfig(target)
      }
    } else {
      alert(t('config.activate_fail', { error: data.error || '', details: data.details || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  } finally {
    activatingConfig.value = false
  }
}

function openCreateProfileModal() {
  newProfileName.value = ''
  newProfileNote.value = ''
  showNewProfileModal.value = true
}

async function createNewProfile() {
  let name = newProfileName.value.trim()
  if (!name) {
    alert(t('config.name_required'))
    return
  }
  if (!name.endsWith('.dae')) {
    name += '.dae'
  }
  const note = newProfileNote.value.trim()
  try {
    const res = await fetch('/api/configs/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, note, content: '' })
    })
    const data = await res.json()
    if (res.ok) {
      showNewProfileModal.value = false
      alert(t('config.create_success', { name: data.name }))
      await fetchConfigProfiles()
      viewLevel.value = 'list'
    } else {
      alert(t('config.create_fail', { error: data.error || '', details: data.details || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  }
}

async function deleteProfile(name: string) {
  if (name === activeRunningConfig.value) {
    alert(t('config.cannot_delete_active'))
    return
  }
  if (!confirm(t('config.confirm_delete_profile', { name }))) return
  try {
    const res = await fetch(`/api/configs/${encodeURIComponent(name)}`, { method: 'DELETE' })
    const data = await res.json()
    if (res.ok) {
      alert(t('config.delete_success'))
      await fetchConfigProfiles()
      if (currentSelectedConfig.value === name) {
        await handleSelectProfile(activeRunningConfig.value)
      }
    } else {
      alert(t('config.delete_fail', { error: data.error || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  }
}

async function batchDeleteProfiles() {
  const targets = selectedProfileNames.value.filter(name => name !== activeRunningConfig.value)
  if (targets.length === 0) {
    alert(t('config.batch_delete_no_selection'))
    return
  }
  if (!confirm(t('config.batch_delete_confirm', { count: targets.length }))) {
    return
  }
  batchDeleting.value = true
  let successCount = 0
  let failCount = 0
  for (const name of targets) {
    try {
      const res = await fetch(`/api/configs/${encodeURIComponent(name)}`, { method: 'DELETE' })
      if (res.ok) {
        successCount++
      } else {
        failCount++
      }
    } catch {
      failCount++
    }
  }
  selectedProfileNames.value = []
  batchDeleting.value = false
  await fetchConfigProfiles()
  if (failCount > 0) {
    alert(t('config.batch_delete_partial', { success: successCount, fail: failCount }))
  } else {
    alert(t('config.batch_delete_success', { count: successCount }))
  }
}

// Copy current profile states & methods
const showCopyProfileModal = ref(false)
const copyProfileName = ref('')

function openCopyProfileModal() {
  const base = currentSelectedConfig.value.replace(/\.dae$/, '')
  copyProfileName.value = `${base}_copy.dae`
  showCopyProfileModal.value = true
}

async function copyCurrentProfile() {
  let name = copyProfileName.value.trim()
  if (!name) {
    alert(t('config.copy_name_required'))
    return
  }
  if (!name.endsWith('.dae')) {
    name += '.dae'
  }
  try {
    const res = await fetch('/api/configs/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, content: configContent.value })
    })
    const data = await res.json()
    if (res.ok) {
      showCopyProfileModal.value = false
      alert(t('config.copy_success', { name: data.name }))
      await fetchConfigProfiles()
      await handleSelectProfile(data.name)
    } else {
      alert(t('config.copy_fail', { error: data.error || '', details: data.details || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  }
}

// Subscription nodes fetching & importing
interface SubFetchedNode {
  key: string
  protocol: string
  name: string
  address: string
  port: string
  country: string
  flag: string
  url: string
  selected?: boolean
}

const showSubNodesModal = ref(false)
const fetchingSubNodes = ref(false)
const currentSubTag = ref('')
const subFetchedNodes = ref<SubFetchedNode[]>([])
const selectAllSubNodes = ref(true)

async function fetchSubNodes(sub: ConfigSubItem) {
  currentSubTag.value = sub.tag
  fetchingSubNodes.value = true
  showSubNodesModal.value = true
  subFetchedNodes.value = []
  try {
    const res = await fetch('/api/subscription/nodes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tag: sub.tag, url: sub.url })
    })
    const data = await res.json()
    if (res.ok) {
      subFetchedNodes.value = (data.nodes || []).map((n: any) => ({
        ...n,
        selected: true
      }))
      selectAllSubNodes.value = true
    } else {
      alert(t('config.sub_read_fail', { error: data.error || '' }))
      showSubNodesModal.value = false
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
    showSubNodesModal.value = false
  } finally {
    fetchingSubNodes.value = false
  }
}

function toggleSelectAllSubNodes() {
  selectAllSubNodes.value = !selectAllSubNodes.value
  for (const n of subFetchedNodes.value) {
    n.selected = selectAllSubNodes.value
  }
}

const selectedSubNodesCount = computed(() => {
  return subFetchedNodes.value.filter(n => n.selected).length
})

function importSelectedSubNodes() {
  const selected = subFetchedNodes.value.filter(n => n.selected)
  if (selected.length === 0) {
    alert(t('config.select_at_least_one_node'))
    return
  }

  let importedCount = 0
  for (const item of selected) {
    let candidateKey = item.key || `${currentSubTag.value}_${importedCount + 1}`
    let suffix = 1
    while (configModel.value.nodes.some(n => n.key === candidateKey)) {
      candidateKey = `${item.key}_${suffix}`
      suffix++
    }

    configModel.value.nodes.push({
      key: candidateKey,
      url: item.url,
      protocol: item.protocol,
      name: item.name || candidateKey,
      address: item.address,
      port: String(item.port || '443')
    })
    importedCount++
  }

  isDirty.value = true
  syncModelToCode()
  showSubNodesModal.value = false
  alert(t('config.import_success', { count: importedCount, name: currentSelectedConfig.value }))
  activeSection.value = 'nodes'
}

async function switchMode(mode: Mode) {
  if (currentMode.value === mode) {
    if (mode === 'code') {
      await nextTick()
      const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
      if (!isAttached && editorContainer.value) {
        initEditor(configContent.value)
      }
      const lineToHighlight = errorLineInfo.value?.line || activeSyntaxError.value?.line
      if (lineToHighlight) {
        highlightErrorLineInEditor(lineToHighlight)
      }
    }
    return
  }

  if (mode === 'code') {
    if (validationStatus.value.valid !== false && !activeSyntaxError.value) {
      await syncModelToCode()
    }
    currentMode.value = 'code'
    await nextTick()
    const isAttached = !!(editorView.value && editorContainer.value && editorContainer.value.contains(editorView.value.dom))
    if (!isAttached && editorContainer.value) {
      initEditor(configContent.value)
    } else if (editorView.value) {
      setEditorText(configContent.value)
    }
    const lineToHighlight = errorLineInfo.value?.line || activeSyntaxError.value?.line
    if (lineToHighlight) {
      highlightErrorLineInEditor(lineToHighlight)
    }
  } else {
    currentMode.value = 'form'
    await loadConfig(currentSelectedConfig.value)
  }
}

async function syncModelToCode() {
  try {
    const res = await fetch('/api/config/serialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(configModel.value)
    })
    const data = await res.json()
    if (res.ok) {
      configContent.value = data.raw
      setEditorText(data.raw)
      isDirty.value = true
      validationStatus.value = {
        checked: true,
        valid: data.valid,
        message: data.message
      }
    }
  } catch (e) {
    console.error('Serialize model failed:', e)
  }
}

async function validateContent(content: string, showAlert = true) {
  validating.value = true
  try {
    const res = await fetch('/api/config/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content })
    })
    const data = await res.json()
    validationStatus.value = {
      checked: true,
      valid: data.valid,
      message: data.message || (data.valid ? t('config.val_valid') : t('config.val_invalid'))
    }
    if (!data.valid) {
      const err = findErrorLineInContent(content, data.message)
      activeSyntaxError.value = err
      if (currentMode.value === 'code' && err?.line) {
        highlightErrorLineInEditor(err.line)
      }
      if (showAlert) {
        alert(t('config.val_error_alert', { message: data.message }))
      }
    } else {
      activeSyntaxError.value = null
      if (editorView.value) {
        editorView.value.dispatch({ effects: setErrorLineEffect.of(null) })
      }
    }
  } catch (e: any) {
    validationStatus.value = {
      checked: true,
      valid: false,
      message: t('config.req_fail', { error: e.message })
    }
  } finally {
    validating.value = false
  }
}

async function saveConfig() {
  if (currentMode.value === 'form') {
    await syncModelToCode()
  }

  if (!validationStatus.value.valid && validationStatus.value.checked) {
    if (!confirm(t('config.confirm_force_save'))) {
      return
    }
  }

  saving.value = true
  try {
    const res = await fetch(`/api/config?name=${encodeURIComponent(currentSelectedConfig.value)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: currentSelectedConfig.value,
        content: configContent.value,
        autoBackup: autoBackup.value,
        reloadConfig: reloadAfterSave.value
      })
    })
    const data = await res.json()
    if (res.ok) {
      isDirty.value = false
      activeSyntaxError.value = null
      alert(t('config.save_success', { name: currentSelectedConfig.value }) + (data.reloaded ? t('config.reloaded_notice') : ''))
      loadConfig(currentSelectedConfig.value)
    } else {
      if (data.details) {
        const err = findErrorLineInContent(configContent.value, data.details)
        activeSyntaxError.value = err
        validationStatus.value = {
          checked: true,
          valid: false,
          message: data.details
        }
        if (currentMode.value === 'code' && err?.line) {
          highlightErrorLineInEditor(err.line)
        }
      }
      alert(t('config.save_fail', { error: data.error || '', details: data.details || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  } finally {
    saving.value = false
  }
}

// Node CRUD
function openAddNode() {
  nodeModalMode.value = 'add'
  editingNode.value = {
    key: 'custom_node_' + (configModel.value.nodes.length + 1),
    url: '',
    protocol: 'VLESS',
    name: '',
    address: '',
    port: '443'
  }
  showNodeModal.value = true
}

function openEditNode(node: ConfigNodeItem) {
  nodeModalMode.value = 'edit'
  editingNode.value = { ...node }
  showNodeModal.value = true
}

function saveNodeModal() {
  if (!editingNode.value.key || !editingNode.value.url) {
    alert(t('config.node_key_url_req'))
    return
  }
  if (nodeModalMode.value === 'add') {
    configModel.value.nodes.push({ ...editingNode.value })
  } else {
    const idx = configModel.value.nodes.findIndex(n => n.key === editingNode.value.key)
    if (idx >= 0) {
      configModel.value.nodes[idx] = { ...editingNode.value }
    }
  }
  showNodeModal.value = false
  isDirty.value = true
  syncModelToCode()
}

function deleteNode(key: string) {
  if (!confirm(t('config.confirm_delete_node', { key }))) return
  configModel.value.nodes = configModel.value.nodes.filter(n => n.key !== key)
  isDirty.value = true
  syncModelToCode()
}

// Group CRUD
async function openAddGroup() {
  await fetchNodesOverview()
  groupModalMode.value = 'add'
  const defaultSub = availableSubscriptions.value[0]?.tag || ''
  editingGroup.value = {
    name: '',
    sourceType: defaultSub ? 'sub' : (availableNodes.value.length > 0 ? 'nodes' : 'custom'),
    selectedSubTag: defaultSub,
    selectedNodeKeys: availableNodes.value.slice(0, 3).map(n => n.key),
    policyMode: 'min_moving_avg',
    fixedNodeIndex: 0,
    customFilterText: ''
  }
  nodeSearchQuery.value = ''
  showGroupModal.value = true
}

async function openEditGroup(g: ConfigGroupItem) {
  await fetchNodesOverview()
  groupModalMode.value = 'edit'
  
  let sourceType: 'sub' | 'nodes' | 'custom' = 'custom'
  let selectedSubTag = ''
  let selectedNodeKeys: string[] = []
  let customFilterText = (g.filters || []).join('\n')

  if (g.filters && g.filters.length > 0) {
    const first = g.filters[0].trim()
    if (first.startsWith('subtag(') && first.endsWith(')')) {
      sourceType = 'sub'
      selectedSubTag = first.slice(7, -1).trim().replace(/['"]/g, '')
    } else if (first.startsWith('name(') && first.endsWith(')') && !first.includes('keyword:')) {
      sourceType = 'nodes'
      selectedNodeKeys = first.slice(5, -1).split(',').map(s => s.trim().replace(/['"]/g, '')).filter(Boolean)
    }
  }

  let policyMode: 'min_moving_avg' | 'min_avg10' | 'min' | 'random' | 'fixed' = 'min_moving_avg'
  let fixedNodeIndex = 0
  const pol = (g.policy || '').trim()
  if (pol.startsWith('fixed(') && pol.endsWith(')')) {
    policyMode = 'fixed'
    const idxStr = pol.slice(6, -1).trim()
    fixedNodeIndex = parseInt(idxStr, 10) || 0
  } else if (['min_moving_avg', 'min_avg10', 'min', 'random'].includes(pol)) {
    policyMode = pol as any
  }

  editingGroup.value = {
    name: g.name,
    sourceType,
    selectedSubTag: selectedSubTag || availableSubscriptions.value[0]?.tag || '',
    selectedNodeKeys,
    policyMode,
    fixedNodeIndex,
    customFilterText
  }
  nodeSearchQuery.value = ''
  showGroupModal.value = true
}

function saveGroupModal() {
  const name = editingGroup.value.name.trim()
  if (!name) {
    alert(t('config.group_name_req'))
    return
  }
  if (!/^[a-zA-Z0-9_\-]+$/.test(name)) {
    alert('策略组名称只能包含字母、数字、下划线或减号')
    return
  }
  if (groupModalMode.value === 'add') {
    if (configModel.value.groups.some(g => g.name === name)) {
      alert(`已存在名为 "${name}" 的策略组，请使用不同的名称`)
      return
    }
  }

  const filters = generatedFilters.value
  if (filters.length === 0) {
    alert('请至少选择一个节点订阅源或指定至少一个节点')
    return
  }

  const policy = generatedPolicyString.value
  const groupItem: ConfigGroupItem = {
    name,
    policy,
    filters
  }

  if (groupModalMode.value === 'add') {
    configModel.value.groups.push(groupItem)
  } else {
    const idx = configModel.value.groups.findIndex(g => g.name === name)
    if (idx >= 0) {
      configModel.value.groups[idx] = groupItem
    } else {
      configModel.value.groups.push(groupItem)
    }
  }
  showGroupModal.value = false
  isDirty.value = true
  syncModelToCode()
}

function deleteGroup(name: string) {
  if (!confirm(t('config.confirm_delete_group', { name }))) return
  configModel.value.groups = configModel.value.groups.filter(g => g.name !== name)
  isDirty.value = true
  syncModelToCode()
}

// Routing CRUD, Presets & Builder Helpers
function addBuilderItem() {
  builderItems.value.push({ not: false, type: 'domain', modifier: 'geosite', value: '' })
  updateConditionFromBuilder()
}

function removeBuilderItem(idx: number) {
  if (builderItems.value.length > 1) {
    builderItems.value.splice(idx, 1)
    updateConditionFromBuilder()
  }
}

function updateConditionFromBuilder() {
  const parts: string[] = []
  for (const item of builderItems.value) {
    const val = item.value.trim()
    if (!val) continue
    let sub = ''
    if (item.type === 'domain') {
      if (item.modifier && item.modifier !== 'none') {
        sub = `domain(${item.modifier}:${val})`
      } else {
        sub = `domain(${val})`
      }
    } else if (item.type === 'dip') {
      if (item.modifier === 'geoip') {
        sub = `dip(geoip:${val})`
      } else {
        sub = `dip(${val})`
      }
    } else if (item.type === 'dport') {
      sub = `dport(${val})`
    } else if (item.type === 'sip') {
      sub = `sip(${val})`
    } else if (item.type === 'sport') {
      sub = `sport(${val})`
    } else if (item.type === 'l4proto') {
      sub = `l4proto(${val})`
    } else if (item.type === 'ipversion') {
      sub = `ipversion(${val})`
    } else if (item.type === 'pname') {
      sub = `pname(${val})`
    } else if (item.type === 'mac') {
      sub = `mac('${val}')`
    }
    if (sub) {
      parts.push((item.not ? '!' : '') + sub)
    }
  }
  editingRoute.value.condition = parts.join(' && ')
}

function applyPreset(presetId: string) {
  selectedPresetId.value = presetId
  const p = ROUTING_PRESETS.find(x => x.id === presetId)
  if (!p) return
  editingRoute.value.condition = p.condition
  if (!editingRoute.value.target || editingRoute.value.target === 'proxy' || editingRoute.value.target === 'direct') {
    editingRoute.value.target = p.recommendedTarget
  }
}

function applyExampleToRoute(ex: { code: string; explain: string }, recTarget = 'direct') {
  editingRoute.value.condition = ex.code
  editingRoute.value.target = recTarget
  routeInputMode.value = 'custom'
  routeModalTab.value = 'form'
}

function openAddRoute() {
  routeModalMode.value = 'add'
  routeModalTab.value = 'form'
  routeInputMode.value = 'preset'
  selectedPresetId.value = 'lan-private'
  editingRoute.value = {
    condition: 'dip(geoip:private)',
    target: 'direct',
    customTarget: '',
    index: -1
  }
  builderItems.value = [
    { not: false, type: 'domain', modifier: 'geosite', value: 'cn' }
  ]
  showRouteModal.value = true
}

function openEditRoute(r: ConfigRouteRule, idx: number) {
  routeModalMode.value = 'edit'
  routeModalTab.value = 'form'
  const knownBuiltins = ['direct', 'must_direct', 'block', 'proxy']
  const groupNames = configModel.value.groups.map(g => g.name)
  let targetVal = r.target
  let customVal = ''

  if (!knownBuiltins.includes(r.target) && !groupNames.includes(r.target)) {
    targetVal = '__custom__'
    customVal = r.target
  }

  const matchedPreset = ROUTING_PRESETS.find(p => p.condition === r.condition)
  if (matchedPreset) {
    routeInputMode.value = 'preset'
    selectedPresetId.value = matchedPreset.id
  } else {
    routeInputMode.value = 'custom'
  }

  editingRoute.value = {
    condition: r.condition,
    target: targetVal,
    customTarget: customVal,
    index: idx
  }
  showRouteModal.value = true
}

function saveRouteModal() {
  if (routeInputMode.value === 'builder') {
    updateConditionFromBuilder()
  }

  let finalTarget = editingRoute.value.target
  if (finalTarget === '__custom__') {
    finalTarget = editingRoute.value.customTarget.trim()
  }
  if (!editingRoute.value.condition || !finalTarget) {
    alert(t('routing.condition_target_required'))
    return
  }

  const trimmedCond = editingRoute.value.condition.trim()
  const item: ConfigRouteRule = {
    condition: trimmedCond,
    target: finalTarget
  }
  if (routeModalMode.value === 'add') {
    configModel.value.routingRules.push(item)
  } else {
    if (editingRoute.value.index >= 0) {
      configModel.value.routingRules[editingRoute.value.index] = item
    }
  }
  showRouteModal.value = false
  isDirty.value = true
  syncModelToCode()
}

function deleteRoute(idx: number) {
  configModel.value.routingRules.splice(idx, 1)
  isDirty.value = true
  syncModelToCode()
}

function moveRouteUp(idx: number) {
  if (idx <= 0) return
  const rules = configModel.value.routingRules
  const temp = rules[idx]
  rules[idx] = rules[idx - 1]
  rules[idx - 1] = temp
  isDirty.value = true
  syncModelToCode()
}

function moveRouteDown(idx: number) {
  const rules = configModel.value.routingRules
  if (idx >= rules.length - 1) return
  const temp = rules[idx]
  rules[idx] = rules[idx + 1]
  rules[idx + 1] = temp
  isDirty.value = true
  syncModelToCode()
}

function duplicateRoute(idx: number) {
  const rule = configModel.value.routingRules[idx]
  configModel.value.routingRules.splice(idx + 1, 0, { ...rule })
  isDirty.value = true
  syncModelToCode()
}

function highlightRule(idx: number) {
  highlightedRuleIndex.value = idx
  setTimeout(() => {
    const el = document.getElementById(`route-row-${idx}`)
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
  isDirty.value = true
  syncModelToCode()
}

function fixFallbackOrder() {
  const rules = configModel.value.routingRules
  const fbIdx = rules.findIndex(r => r.condition.trim() === 'fallback')
  if (fbIdx >= 0 && fbIdx < rules.length - 1) {
    const [fbRule] = rules.splice(fbIdx, 1)
    rules.push(fbRule)
    isDirty.value = true
    syncModelToCode()
  }
}

// Subscriptions
function openAddSub() {
  editingSub.value = { tag: 'sub_' + (configModel.value.subscriptions.length + 1), url: '' }
  showSubModal.value = true
}

function saveSubModal() {
  if (!editingSub.value.tag || !editingSub.value.url) return
  configModel.value.subscriptions.push({ ...editingSub.value })
  showSubModal.value = false
  isDirty.value = true
  syncModelToCode()
}

function deleteSub(tag: string) {
  configModel.value.subscriptions = configModel.value.subscriptions.filter(s => s.tag !== tag)
  isDirty.value = true
  syncModelToCode()
}

// Backups & file ops
function exportConfig() {
  downloadProfile(currentSelectedConfig.value)
}

function triggerImport() {
  fileInput.value?.click()
}

async function handleFileSelect(e: Event) {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = async (event) => {
    const content = event.target?.result as string
    if (typeof content === 'string') {
      if (confirm(t('config.confirm_import_file', { name: file.name }))) {
        setEditorText(content)
        isDirty.value = true
        validateContent(content, true)
        // sync model
        const res = await fetch('/api/config/model')
        if (res.ok) configModel.value = await res.json()
      }
    }
    if (fileInput.value) fileInput.value.value = ''
  }
  reader.readAsText(file)
}

async function openBackupManager() {
  showBackupModal.value = true
  await fetchBackups()
}

async function fetchBackups() {
  try {
    const res = await fetch('/api/backups')
    const data = await res.json()
    if (res.ok) backups.value = data.backups || []
  } catch (e) {
    console.error(e)
  }
}

async function createManualBackup() {
  try {
    const res = await fetch('/api/backups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label: newBackupLabel.value })
    })
    if (res.ok) {
      newBackupLabel.value = ''
      await fetchBackups()
    }
  } catch (e) {
    alert(t('config.save_fail', { error: String(e), details: '' }))
  }
}

async function restoreBackup(name: string) {
  if (!confirm(t('config.confirm_rollback_backup', { name }))) return
  try {
    const res = await fetch('/api/backups/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    })
    const data = await res.json()
    if (res.ok) {
      alert(t('config.rollback_success'))
      showBackupModal.value = false
      loadConfig()
    } else {
      alert(t('config.rollback_fail', { error: data.error || '' }))
    }
  } catch (e: any) {
    alert(t('config.req_fail', { error: e.message }))
  }
}

async function deleteBackup(name: string) {
  if (!confirm(t('config.confirm_delete_backup', { name }))) return
  try {
    const res = await fetch(`/api/backups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    if (res.ok) await fetchBackups()
  } catch (e) {
    alert(t('config.delete_fail', { error: String(e) }))
  }
}

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatDate(iso: string) {
  if (!iso) return '-'
  return new Date(iso).toLocaleString('zh-CN', { hour12: false })
}

onMounted(async () => {
  await fetchConfigProfiles()
  if (configProfiles.value.length > 0) {
    const active = configProfiles.value.find(p => p.isActive)
    currentSelectedConfig.value = active ? active.name : configProfiles.value[0].name
  }
})
</script>

<template>
  <div class="config-view">
    <!-- LEVEL 1: Configuration Profiles List View (默认展示) -->
    <div v-if="viewLevel === 'list'" class="profiles-list-view">
      <header class="profiles-header">
        <div class="header-left">
          <div class="header-icon-wrap">
            <Files :size="24" class="text-primary" />
          </div>
          <div>
            <h2 class="profiles-title">{{ t('config.profiles_title') }}</h2>
            <p class="profiles-subtitle">{{ t('config.profiles_subtitle') }}</p>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn btn-primary" @click="openCreateProfileModal">
            <Plus :size="15" />
            <span>{{ t('config.btn_new_profile') }}</span>
          </button>
          <button
            class="btn btn-secondary"
            @click="triggerProfileImport"
            :disabled="importingProfile"
            :title="t('config.btn_import_profile')"
          >
            <Upload :size="15" />
            <span>{{ importingProfile ? t('config.importing') : t('config.btn_import_profile') }}</span>
          </button>
          <button
            class="btn btn-secondary"
            @click="toggleSelectAllProfiles"
            :title="isAllSelected ? t('config.btn_deselect_all') : t('config.btn_select_all')"
          >
            <CheckSquare :size="15" />
            <span>{{ isAllSelected ? t('config.btn_deselect_all') : t('config.btn_select_all') }}</span>
          </button>
          <button
            class="btn btn-secondary danger"
            :disabled="selectedProfileNames.length === 0 || batchDeleting"
            @click="batchDeleteProfiles"
            :title="t('config.btn_batch_delete')"
          >
            <Trash2 :size="15" />
            <span>{{ selectedProfileNames.length > 0 ? `${t('config.btn_batch_delete')} (${selectedProfileNames.length})` : t('config.btn_batch_delete') }}</span>
          </button>
          <button class="btn btn-secondary" @click="openBackupManager" :title="t('config.backup_btn')">
            <Archive :size="15" />
            <span>{{ t('config.backup_btn') }}</span>
          </button>
        </div>
        <input
          ref="profileFileInput"
          type="file"
          accept=".dae,.conf,.txt"
          class="hidden"
          @change="handleProfileFileImport"
        />
      </header>

      <div class="profiles-table-container">
        <table class="profiles-table">
          <thead>
            <tr>
              <th style="width: 44px; text-align: center">
                <input
                  type="checkbox"
                  :checked="isAllSelected"
                  :indeterminate="isIndeterminate"
                  @change="toggleSelectAllProfiles"
                  :title="t('config.btn_select_all')"
                />
              </th>
              <th style="width: 140px; min-width: 130px; white-space: nowrap">{{ t('config.col_profile_name') }}</th>
              <th style="min-width: 180px">{{ t('config.col_profile_note') }}</th>
              <th style="width: 120px; min-width: 110px; white-space: nowrap">{{ t('config.col_profile_rules') }}</th>
              <th style="width: 145px; min-width: 135px; white-space: nowrap">{{ t('config.col_profile_nodes') }}</th>
              <th style="width: 155px; min-width: 140px; white-space: nowrap">{{ t('config.col_profile_mod') }}</th>
              <th style="width: 125px; min-width: 115px; white-space: nowrap">{{ t('config.col_profile_status') }}</th>
              <th style="width: 290px; min-width: 290px; text-align: right; white-space: nowrap">{{ t('config.col_profile_action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in configProfiles"
              :key="p.name"
              :class="{ 'row-active': p.isActive, 'row-selected': selectedProfileNames.includes(p.name) }"
              @dblclick="openEditProfile(p.name)"
            >
              <td class="checkbox-col" @click.stop style="text-align: center; width: 44px">
                <input
                  type="checkbox"
                  :value="p.name"
                  v-model="selectedProfileNames"
                  :disabled="p.isActive"
                  :title="p.isActive ? t('config.cannot_delete_active') : ''"
                />
              </td>
              <td class="profile-name-cell">
                <div class="profile-name-wrap" :title="p.name">
                  <FileCode :size="15" class="text-info flex-shrink-0" />
                  <span class="profile-filename font-bold">{{ p.name }}</span>
                </div>
              </td>
              <td class="profile-note-cell">
                <span class="profile-note-text text-muted">{{ p.note || '-' }}</span>
              </td>
              <td>
                <span class="badge-count">{{ p.rulesCount || 0 }} 条</span>
              </td>
              <td>
                <span class="badge-count">{{ p.nodesCount || 0 }} 个</span>
              </td>
              <td class="text-muted text-xs mono">
                {{ formatDate(p.modTime) }}
              </td>
              <td>
                <span v-if="p.isActive" class="badge-active-prominent">
                  <CheckCircle2 :size="13" />
                  <span>{{ t('config.status_active') }}</span>
                </span>
                <span
                  v-else-if="p.isValid === false"
                  class="badge-invalid-clickable"
                  @click.stop="openErrorModal(p)"
                  :title="p.errorMsg || t('config.status_invalid')"
                >
                  <AlertTriangle :size="13" />
                  <span>{{ t('config.status_invalid') }}</span>
                </span>
                <span v-else class="badge-inactive">
                  {{ t('config.status_inactive') }}
                </span>
              </td>
              <td style="text-align: right" class="profile-action-cell">
                <button
                  class="btn btn-primary btn-sm btn-edit-profile"
                  @click="openEditProfile(p.name)"
                  :title="t('config.action_edit')"
                >
                  <Edit2 :size="13" />
                  <span>{{ t('config.action_edit') }}</span>
                </button>
                <button
                  v-if="!p.isActive"
                  class="btn btn-secondary btn-sm btn-activate-profile"
                  :disabled="activatingConfig"
                  @click="activateSelectedProfile(p.name)"
                  :title="t('config.action_activate')"
                >
                  <Check :size="13" />
                  <span>{{ t('config.action_activate') }}</span>
                </button>
                <button
                  class="btn-icon-subtle"
                  @click="downloadProfile(p.name)"
                  :title="t('config.btn_download_profile')"
                >
                  <Download :size="13" />
                </button>
                <button
                  class="btn-icon-subtle"
                  @click="currentSelectedConfig = p.name; openCopyProfileModal()"
                  :title="t('common.copy')"
                >
                  <Copy :size="13" />
                </button>
                <button
                  v-if="!p.isActive"
                  class="btn-icon-subtle danger"
                  @click="deleteProfile(p.name)"
                  :title="t('common.delete')"
                >
                  <Trash2 :size="13" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- LEVEL 2: Configuration Detail Editor View (配置文件各模块配置) -->
    <div v-else class="profile-detail-view">
      <!-- Top toolbar -->
      <header class="toolbar">
        <div class="toolbar-left">
          <button
            class="btn btn-secondary btn-sm btn-back"
            @click="backToProfilesList"
            :title="t('config.btn_back_to_profiles')"
          >
            <ArrowLeft :size="14" />
            <span>{{ t('config.btn_back_to_profiles') }}</span>
          </button>

          <div class="mode-switcher">
            <button
              class="mode-btn"
              :class="{ active: currentMode === 'form' }"
              @click="switchMode('form')"
            >
              <LayoutGrid :size="14" />
              <span>{{ t('config.mode.form') }}</span>
            </button>
            <button
              class="mode-btn"
              :class="{ active: currentMode === 'code', 'mode-btn-error': errorLineInfo !== null }"
              @click="switchMode('code')"
              :title="errorLineInfo ? ('代码存在语法错误' + (errorLineInfo.line ? ` (第 ${errorLineInfo.line} 行)` : '')) : t('config.mode.code')"
            >
              <Code :size="14" />
              <span>{{ t('config.mode.code') }}</span>
              <span v-if="errorLineInfo" class="code-error-badge-dot">●</span>
            </button>
          </div>

          <!-- Current Profile Info Pill -->
          <div class="profile-current-pill">
            <Files :size="14" class="text-primary mr-1" />
            <span class="profile-current-name">{{ currentSelectedConfig }}</span>
            <span v-if="currentProfileNote" class="profile-note-pill">{{ currentProfileNote }}</span>
            <span v-if="isDirty" class="dirty-dot" title="有未保存的改动">●</span>
          </div>

          <!-- Active badge or Activate button -->
          <span
            v-if="currentSelectedConfig === activeRunningConfig"
            class="badge-active-prominent ml-2"
          >
            <CheckCircle2 :size="13" />
            <span>{{ t('config.status_active') }}</span>
          </span>
          <button
            v-else
            class="btn btn-sm btn-activate ml-2"
            :disabled="activatingConfig"
            @click="activateSelectedProfile(currentSelectedConfig)"
            :title="t('config.activate_btn')"
          >
            <Check :size="13" />
            <span>{{ activatingConfig ? t('config.action_activating') : t('config.action_activate') }}</span>
          </button>
        </div>

        <div class="toolbar-actions">
          <!-- Validation trigger -->
          <button
            class="btn btn-secondary"
            :disabled="validating"
            @click="validateContent(configContent, true)"
            :title="t('config.validate_btn')"
          >
            <ShieldCheck class="icon" :size="15" />
            <span>{{ validating ? t('config.validating') : t('config.validate_btn') }}</span>
          </button>

          <!-- Import / Export -->
          <input
            ref="fileInput"
            type="file"
            accept=".dae,.conf,.txt"
            class="hidden"
            @change="handleFileSelect"
          />
          <button class="btn btn-secondary" @click="triggerImport" :title="t('common.restore')">
            <Upload class="icon" :size="15" />
            <span>{{ t('common.restore') }}</span>
          </button>

          <button class="btn btn-secondary" @click="exportConfig" :title="t('common.download')">
            <Download class="icon" :size="15" />
            <span>{{ t('common.download') }}</span>
          </button>

          <button class="btn btn-secondary" @click="openBackupManager" :title="t('config.backup_btn')">
            <Archive class="icon" :size="15" />
            <span>{{ t('config.backup_btn') }}</span>
          </button>

          <div class="divider"></div>

          <label class="checkbox-label" :title="t('config.reload_switch')">
            <input type="checkbox" v-model="reloadAfterSave" />
            <span>{{ t('config.reload_switch') }}</span>
          </label>

          <!-- Save Button -->
          <button class="btn btn-primary" :disabled="saving" @click="saveConfig">
            <Save class="icon" :size="15" />
            <span>{{ saving ? t('config.saving') : t('config.save_btn') }}</span>
          </button>
        </div>
      </header>

      <!-- Main Workspace Area -->
      <div class="workspace-area">
        <!-- 1. Visual Form Mode -->
        <div v-if="currentMode === 'form'" class="form-mode-container">
          <!-- Sub-navigation tabs: strictly following official example.dae order -->
          <div class="form-subnav">
            <!-- 1. Global -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'global', 'subnav-has-error': erroneousSection === 'global' }"
              @click="activeSection = 'global'"
              :title="erroneousSection === 'global' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <Sliders :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'global' }">{{ t('config.tab_global') }}</span>
              <span v-if="erroneousSection === 'global'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
            </button>

            <!-- 2. Subscriptions -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'subscriptions', 'subnav-has-error': erroneousSection === 'subscriptions' }"
              @click="activeSection = 'subscriptions'"
              :title="erroneousSection === 'subscriptions' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <Download :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'subscriptions' }">{{ t('config.tab_subscriptions') }}</span>
              <span v-if="erroneousSection === 'subscriptions'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
              <span v-else class="count-pill">{{ configModel.subscriptions.length }}</span>
            </button>

            <!-- 3. Nodes -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'nodes', 'subnav-has-error': erroneousSection === 'nodes' }"
              @click="activeSection = 'nodes'"
              :title="erroneousSection === 'nodes' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <Server :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'nodes' }">{{ t('config.tab_nodes') }}</span>
              <span v-if="erroneousSection === 'nodes'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
              <span v-else class="count-pill">{{ configModel.nodes.length }}</span>
            </button>

            <!-- 4. DNS Settings -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'dns', 'subnav-has-error': erroneousSection === 'dns' }"
              @click="activeSection = 'dns'"
              :title="erroneousSection === 'dns' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <Globe :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'dns' }">{{ t('config.tab_dns') }}</span>
              <span v-if="erroneousSection === 'dns'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
              <span v-else class="count-pill">{{ configModel.dns?.upstreams?.length || 0 }}</span>
            </button>

            <!-- 5. Groups -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'groups', 'subnav-has-error': erroneousSection === 'groups' }"
              @click="activeSection = 'groups'"
              :title="erroneousSection === 'groups' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <Layers :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'groups' }">{{ t('config.tab_groups') }}</span>
              <span v-if="erroneousSection === 'groups'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
              <span v-else class="count-pill">{{ configModel.groups.length }}</span>
            </button>

            <!-- 6. Routing Rules -->
            <button
              class="subnav-item"
              :class="{ active: activeSection === 'routing', 'subnav-has-error': erroneousSection === 'routing' }"
              @click="activeSection = 'routing'"
              :title="erroneousSection === 'routing' ? (validationStatus.message || '该模块存在语法错误') : ''"
            >
              <GitFork :size="15" />
              <span :class="{ 'text-danger font-bold': erroneousSection === 'routing' }">{{ t('config.tab_routing') }}</span>
              <span v-if="erroneousSection === 'routing'" class="subnav-error-badge">
                <AlertTriangle :size="11" /> 错误
              </span>
              <span v-else class="count-pill">{{ configModel.routingRules.length }}</span>
            </button>
          </div>

          <!-- Section 1: Global Parameters (按官方文档第一项) -->
          <div v-if="activeSection === 'global'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'global'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>

            <div class="section-title-bar">
              <div>
                <h3 :class="{ 'text-danger': erroneousSection === 'global' }">
                  {{ t('config.section.global') }}
                  <span v-if="erroneousSection === 'global'" class="section-error-pill">
                    <AlertTriangle :size="13" /> 存在语法错误
                  </span>
                </h3>
                <p>{{ t('config.global_desc') }}</p>
              </div>
            </div>

            <div class="form-grid">
              <!-- 1. tproxy_port -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_tproxy_port') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.tproxy_port')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['tproxy_port']" @input="isDirty = true" :placeholder="t('config.field.global_tproxy_port_ph')" />
                <span class="field-help-text">{{ t('config.help.tproxy_port') }}</span>
              </div>

              <!-- 2. tproxy_port_protect -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_tproxy_port_protect') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.tproxy_port_protect')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['tproxy_port_protect']" @change="isDirty = true">
                  <option value="true">true (推荐开启/防环路)</option>
                  <option value="false">false</option>
                </select>
                <span class="field-help-text">{{ t('config.help.tproxy_port_protect') }}</span>
              </div>

              <!-- 3. lan_interface -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_lan_iface') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.lan_interface')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['lan_interface']" @input="isDirty = true" :placeholder="t('config.field.global_lan_iface_ph')" />
                <span class="field-help-text">{{ t('config.help.lan_interface') }}</span>
              </div>

              <!-- 4. wan_interface -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_wan_iface') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.wan_interface')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['wan_interface']" @input="isDirty = true" :placeholder="t('config.field.global_wan_iface_ph')" />
                <span class="field-help-text">{{ t('config.help.wan_interface') }}</span>
              </div>

              <!-- 5. auto_config_kernel_parameter -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_auto_config_kernel') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.auto_config_kernel')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['auto_config_kernel_parameter']" @change="isDirty = true">
                  <option value="true">true (自动配置内核参数/推荐)</option>
                  <option value="false">false</option>
                </select>
                <span class="field-help-text">{{ t('config.help.auto_config_kernel') }}</span>
              </div>

              <!-- 6. dial_mode -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_dial_mode') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.dial_mode')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['dial_mode']" @change="isDirty = true">
                  <option value="domain">domain (保留域名优先/推荐)</option>
                  <option value="ip">ip (直连IP)</option>
                  <option value="domain+">domain+ (双栈退避)</option>
                  <option value="domain++">domain++ (严格域名)</option>
                </select>
                <span class="field-help-text">{{ t('config.help.dial_mode') }}</span>
              </div>

              <!-- 7. tcp_check_url -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_tcp_check_url') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.tcp_check_url')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['tcp_check_url']" @input="isDirty = true" :placeholder="t('config.field.global_tcp_check_url_ph')" />
                <span class="field-help-text">{{ t('config.help.tcp_check_url') }}</span>
              </div>

              <!-- 8. tcp_check_http_method -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_tcp_check_http_method') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.tcp_check_http_method')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['tcp_check_http_method']" @change="isDirty = true">
                  <option value="HEAD">HEAD (极速低流量/推荐)</option>
                  <option value="GET">GET</option>
                </select>
                <span class="field-help-text">{{ t('config.help.tcp_check_http_method') }}</span>
              </div>

              <!-- 9. check_interval -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_check_interval') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.check_interval')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['check_interval']" @input="isDirty = true" placeholder="30s" />
                <span class="field-help-text">{{ t('config.help.check_interval') }}</span>
              </div>

              <!-- 10. check_tolerance -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_check_tolerance') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.check_tolerance')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['check_tolerance']" @input="isDirty = true" placeholder="50ms" />
                <span class="field-help-text">{{ t('config.help.check_tolerance') }}</span>
              </div>

              <!-- 11. sniffing_timeout -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_sniffing_timeout') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.sniffing_timeout')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['sniffing_timeout']" @input="isDirty = true" placeholder="100ms" />
                <span class="field-help-text">{{ t('config.help.sniffing_timeout') }}</span>
              </div>

              <!-- 12. tls_implementation -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_tls_implementation') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.tls_implementation')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['tls_implementation']" @change="isDirty = true">
                  <option value="tls">tls (标准 Go 协议栈)</option>
                  <option value="utls">utls (指纹伪装增强)</option>
                </select>
                <span class="field-help-text">{{ t('config.help.tls_implementation') }}</span>
              </div>

              <!-- 13. utls -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_utls') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.utls')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <input v-model="configModel.globals['utls']" @input="isDirty = true" placeholder="chrome" />
                <span class="field-help-text">{{ t('config.help.utls') }}</span>
              </div>

              <!-- 14. allow_insecure -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_allow_insecure') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.allow_insecure')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['allow_insecure']" @change="isDirty = true">
                  <option value="false">false (生产推荐关闭)</option>
                  <option value="true">true</option>
                </select>
                <span class="field-help-text">{{ t('config.help.allow_insecure') }}</span>
              </div>

              <!-- 15. log_level -->
              <div class="form-group">
                <label class="form-label-with-help">
                  <span>{{ t('config.field.global_log_level') }}</span>
                  <span class="help-tooltip-trigger" :title="t('config.help.log_level')">
                    <HelpCircle :size="13" class="help-icon" />
                  </span>
                </label>
                <select v-model="configModel.globals['log_level']" @change="isDirty = true">
                  <option value="trace">trace</option>
                  <option value="debug">debug</option>
                  <option value="info">info (默认)</option>
                  <option value="warn">warn</option>
                  <option value="error">error</option>
                </select>
                <span class="field-help-text">{{ t('config.help.log_level') }}</span>
              </div>
            </div>
          </div>

          <!-- Section 2: Subscriptions (按官方文档第二项) -->
          <div v-else-if="activeSection === 'subscriptions'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'subscriptions'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>
            <div class="section-title-bar">
              <div>
                <h3>{{ t('config.section.subscriptions') }}</h3>
                <p>{{ t('config.sub_desc') }}</p>
              </div>
              <button class="btn btn-primary" @click="openAddSub">
                <Plus :size="14" /> {{ t('config.new_sub') }}
              </button>
            </div>

            <div v-if="configModel.subscriptions.length === 0" class="empty-state-box">
              <Download class="empty-icon" :size="36" />
              <div class="empty-text">{{ t('config.sub_empty') }}</div>
              <p class="empty-subtext">{{ t('config.sub_empty_desc') }}</p>
              <button class="btn btn-primary btn-sm" @click="openAddSub">
                <Plus :size="13" /> {{ t('config.new_sub') }}
              </button>
            </div>

            <div v-else class="cards-grid">
              <div v-for="sub in configModel.subscriptions" :key="sub.tag" class="item-card">
                <div class="card-top">
                  <span class="sub-badge">Sub</span>
                  <span class="card-key">{{ sub.tag }}</span>
                  <div class="card-actions">
                    <button class="btn-sub-fetch" @click="fetchSubNodes(sub)" :title="t('config.btn_fetch_nodes_tip')">
                      <ListFilter :size="13" /> {{ t('config.btn_fetch_nodes') }}
                    </button>
                    <button class="btn-icon delete" @click="deleteSub(sub.tag)" :title="t('common.delete')">
                      <Trash2 :size="13" />
                    </button>
                  </div>
                </div>
                <div class="card-info">
                  <div class="row">
                    <span class="lbl">URL:</span>
                    <span class="val mono truncate" :title="sub.url">{{ sub.url }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 3: Nodes (按官方文档第三项) -->
          <div v-else-if="activeSection === 'nodes'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'nodes'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>

            <div class="section-title-bar">
              <div>
                <h3 :class="{ 'text-danger': erroneousSection === 'nodes' }">
                  {{ t('config.section.nodes') }}
                  <span v-if="erroneousSection === 'nodes'" class="section-error-pill">
                    <AlertTriangle :size="13" /> 存在语法错误
                  </span>
                </h3>
                <p>{{ t('config.nodes_desc') }}</p>
              </div>
              <button class="btn btn-primary" @click="openAddNode">
                <Plus :size="14" /> {{ t('config.new_node') }}
              </button>
            </div>

            <div v-if="configModel.nodes.length === 0" class="empty-state-box">
              <Server class="empty-icon" :size="36" />
              <div class="empty-text">{{ t('config.nodes_empty') }}</div>
              <p class="empty-subtext">{{ t('config.nodes_empty_sub') }}</p>
              <button class="btn btn-primary btn-sm" @click="openAddNode">
                <Plus :size="13" /> {{ t('config.new_node') }}
              </button>
            </div>

            <div v-else class="cards-grid">
              <div v-for="node in configModel.nodes" :key="node.key" class="item-card">
                <div class="card-top">
                  <span class="proto-badge">{{ node.protocol || 'NODE' }}</span>
                  <span class="card-key">{{ node.key }}</span>
                  <div class="card-actions">
                    <button class="btn-icon" @click="openEditNode(node)" :title="t('common.edit')">
                      <Edit2 :size="13" />
                    </button>
                    <button class="btn-icon delete" @click="deleteNode(node.key)" :title="t('common.delete')">
                      <Trash2 :size="13" />
                    </button>
                  </div>
                </div>
                <div class="card-info">
                  <div class="row">
                    <span class="lbl">{{ t('config.node_alias') }}:</span>
                    <span class="val bold truncate" :title="node.name">{{ node.name || node.key }}</span>
                  </div>
                  <div class="row">
                    <span class="lbl">{{ t('config.node_target') }}:</span>
                    <span class="val mono truncate" :title="`${node.address}:${node.port}`">{{ node.address }}:{{ node.port }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 4: DNS Settings (官方文档第四项补全) -->
          <div v-else-if="activeSection === 'dns'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'dns'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>

            <div class="section-title-bar">
              <div>
                <h3 :class="{ 'text-danger': erroneousSection === 'dns' }">
                  {{ t('dns.title') }}
                  <span v-if="erroneousSection === 'dns'" class="section-error-pill">
                    <AlertTriangle :size="13" /> 存在语法错误
                  </span>
                </h3>
                <p>{{ t('dns.subtitle') }}</p>
              </div>
              <button class="btn btn-primary" @click="addDnsUpstream()">
                <Plus :size="14" /> {{ t('dns.btn_add_upstream') }}
              </button>
            </div>

            <!-- DNS Global Parameters -->
            <div class="dns-panel-box mb-4">
              <h4 class="dns-panel-title">
                <Sliders :size="14" class="text-primary mr-1" />
                {{ t('dns.options_title') }}
              </h4>
              <div class="form-grid-3">
                <div class="form-group">
                  <label>{{ t('dns.ipversion_prefer') }}</label>
                  <select v-model.number="configModel.dns.ipVersionPrefer" @change="isDirty = true">
                    <option :value="4">{{ t('dns.ipversion_v4') }}</option>
                    <option :value="6">{{ t('dns.ipversion_v6') }}</option>
                    <option :value="0">{{ t('dns.ipversion_auto') }}</option>
                  </select>
                </div>

                <div class="form-group">
                  <label>{{ t('dns.fixed_ttl') }}</label>
                  <input
                    type="number"
                    v-model.number="configModel.dns.fixedDomainTTL"
                    @input="isDirty = true"
                    :placeholder="t('dns.fixed_ttl_ph')"
                    min="0"
                  />
                </div>

                <div class="form-group">
                  <label>{{ t('dns.fallback_upstream') }}</label>
                  <select v-model="configModel.dns.fallback" @change="isDirty = true">
                    <option v-for="u in configModel.dns.upstreams" :key="u.tag" :value="u.tag">
                      {{ u.tag }} ({{ u.address }})
                    </option>
                  </select>
                  <span class="field-hint">{{ t('dns.fallback_desc') }}</span>
                </div>
              </div>

              <div class="optimistic-cache-row mt-2">
                <label class="checkbox-label" :title="t('dns.optimistic_cache_desc')">
                  <input type="checkbox" v-model="configModel.dns.optimisticCache" @change="isDirty = true" />
                  <span class="bold">{{ t('dns.optimistic_cache') }}</span>
                </label>
                <p class="text-xs text-muted mt-1">{{ t('dns.optimistic_cache_desc') }}</p>
              </div>
            </div>

            <!-- Upstream DNS Servers List -->
            <div class="dns-panel-box mb-4">
              <div class="panel-header-row mb-3">
                <h4 class="dns-panel-title">
                  <Server :size="14" class="text-info mr-1" />
                  {{ t('dns.upstreams_title') }} ({{ configModel.dns.upstreams.length }})
                </h4>
                <button class="btn btn-secondary btn-sm" @click="addDnsUpstream()">
                  <Plus :size="13" /> {{ t('dns.btn_add_upstream') }}
                </button>
              </div>

              <table class="dns-table">
                <thead>
                  <tr>
                    <th style="width: 180px">{{ t('dns.col_upstream_tag') }}</th>
                    <th>{{ t('dns.col_upstream_addr') }}</th>
                    <th style="width: 100px; text-align: right">{{ t('dns.col_upstream_action') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(u, uIdx) in configModel.dns.upstreams" :key="uIdx">
                    <td>
                      <input v-model="u.tag" class="form-control form-control-sm" placeholder="tag" @input="isDirty = true" />
                    </td>
                    <td>
                      <input v-model="u.address" class="form-control form-control-sm mono" placeholder="udp://223.5.5.5:53" @input="isDirty = true" />
                    </td>
                    <td style="text-align: right">
                      <button class="btn-icon danger" @click="removeDnsUpstream(uIdx)" :title="t('common.delete')">
                        <Trash2 :size="13" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <!-- DNS Request Routing Rules -->
            <div class="dns-panel-box">
              <div class="panel-header-row mb-3">
                <div>
                  <h4 class="dns-panel-title">
                    <GitFork :size="14" class="text-primary mr-1" />
                    {{ t('dns.req_routing_title') }} ({{ configModel.dns.requestRules.length }})
                  </h4>
                  <p class="text-xs text-muted">{{ t('dns.req_routing_desc') }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <button class="btn btn-secondary btn-sm" @click="showDnsSyntaxDoc = !showDnsSyntaxDoc">
                    <BookOpen :size="13" />
                    <span>{{ showDnsSyntaxDoc ? t('common.collapse') : t('dns.btn_toggle_syntax') }}</span>
                  </button>
                  <button class="btn btn-primary btn-sm" @click="addDnsRequestRule()">
                    <Plus :size="13" /> {{ t('dns.btn_add_rule') }}
                  </button>
                </div>
              </div>

              <!-- DNS Request Routing Syntax Guide Box -->
              <div v-if="showDnsSyntaxDoc" class="dns-syntax-guide-box mb-4">
                <div class="guide-header">
                  <div class="guide-title">
                    <BookOpen :size="15" class="text-primary mr-1" />
                    <strong>{{ t('dns.syntax_guide_title') }}</strong>
                  </div>
                  <p class="text-xs text-muted mt-1">{{ t('dns.syntax_guide_desc') }}</p>
                </div>

                <div class="guide-cards-grid mt-3">
                  <div class="syntax-card">
                    <div class="syntax-card-header">
                      <span class="syntax-tag">qname(...)</span>
                      <span class="syntax-name">目标域名特征匹配</span>
                    </div>
                    <p class="syntax-desc">匹配 DNS 待解析域名，支持 GeoSite 规则库、域名后缀、关键字及正则：</p>
                    <ul class="syntax-example-list">
                      <li><code>qname(geosite:cn)</code> <span>匹配国内主流网站与服务域名</span></li>
                      <li><code>qname(geosite:google, geosite:youtube)</code> <span>海外重点服务域名</span></li>
                      <li><code>qname(suffix: apple.com)</code> <span>匹配以 apple.com 结尾的所有域名及子域</span></li>
                      <li><code>qname(full: example.com)</code> <span>全域名精确匹配 (不含子域)</span></li>
                      <li><code>qname(keyword: aliyun)</code> <span>包含特定关键字</span></li>
                      <li><code>qname(regex: '^api\.')</code> <span>正则表达式匹配</span></li>
                    </ul>
                  </div>

                  <div class="syntax-card">
                    <div class="syntax-card-header">
                      <span class="syntax-tag">qtype(...) & inbound(...)</span>
                      <span class="syntax-name">解析类型与来源接口</span>
                    </div>
                    <p class="syntax-desc">匹配 DNS 请求查询的记录类型或发起客户端来源：</p>
                    <ul class="syntax-example-list">
                      <li><code>qtype(a, aaaa)</code> <span>匹配 IPv4 (A) 或 IPv6 (AAAA) 地址查询</span></li>
                      <li><code>qtype(https)</code> <span>匹配 HTTPS SVCB 服务绑定解析记录</span></li>
                      <li><code>qtype(txt)</code> <span>匹配 TXT 文本记录查询</span></li>
                      <li><code>inbound(lan)</code> <span>仅对局域网入站请求生效</span></li>
                    </ul>
                  </div>

                  <div class="syntax-card">
                    <div class="syntax-card-header">
                      <span class="syntax-tag">逻辑组合 & 兜底执行</span>
                      <span class="syntax-name">按序命中与 Fallback</span>
                    </div>
                    <p class="syntax-desc">规则判定与执行流程说明：</p>
                    <ul class="syntax-example-list">
                      <li><code>qname(geosite:cn) && !qtype(https)</code> <span>支持 && 组合与 ! 取反</span></li>
                      <li><strong>按序命中：</strong> <span>自上而下匹配，一旦首条规则命中立即交由目标上游解析并终止后续匹配</span></li>
                      <li><strong>Fallback 机制：</strong> <span>未命中任何自定义规则时，统一由指定的 Fallback 上游接管</span></li>
                    </ul>
                  </div>
                </div>

                <!-- Quick Presets Insertion -->
                <div class="quick-insert-bar mt-3">
                  <span class="quick-title text-xs text-muted mr-2">常用模板快速填入：</span>
                  <button class="quick-chip-btn" @click="quickInsertDnsRule('qname(geosite:cn)')">
                    + 国内域名直连 (geosite:cn)
                  </button>
                  <button class="quick-chip-btn" @click="quickInsertDnsRule('qname(geosite:geolocation-!cn)')">
                    + 境外域名走海外DNS (geolocation-!cn)
                  </button>
                  <button class="quick-chip-btn" @click="quickInsertDnsRule('qname(suffix: apple.com)')">
                    + 苹果服务优化 (suffix: apple.com)
                  </button>
                  <button class="quick-chip-btn" @click="quickInsertDnsRule('qtype(https)')">
                    + 过滤 SVCB (qtype: https)
                  </button>
                </div>
              </div>

              <table class="dns-table">
                <thead>
                  <tr>
                    <th style="width: 50px">#</th>
                    <th>{{ t('dns.col_rule_cond') }}</th>
                    <th style="width: 220px">{{ t('dns.col_rule_upstream') }}</th>
                    <th style="width: 140px; text-align: right">{{ t('routing.col_order') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(dr, drIdx) in configModel.dns.requestRules" :key="drIdx">
                    <td class="idx-cell">{{ drIdx + 1 }}</td>
                    <td>
                      <input v-model="dr.condition" class="form-control form-control-sm mono" placeholder="qname(geosite:cn)" @input="isDirty = true" />
                    </td>
                    <td>
                      <select v-model="dr.target" class="form-control form-control-sm" @change="isDirty = true">
                        <option v-for="u in configModel.dns.upstreams" :key="u.tag" :value="u.tag">{{ u.tag }}</option>
                      </select>
                    </td>
                    <td style="text-align: right" class="row-action-cell">
                      <button class="btn-icon" :disabled="drIdx === 0" @click="moveDnsRequestRule(drIdx, 'up')">
                        <ArrowUp :size="12" />
                      </button>
                      <button class="btn-icon" :disabled="drIdx === configModel.dns.requestRules.length - 1" @click="moveDnsRequestRule(drIdx, 'down')">
                        <ArrowDown :size="12" />
                      </button>
                      <button class="btn-icon delete" @click="removeDnsRequestRule(drIdx)">
                        <Trash2 :size="12" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Section 5: Groups (按官方文档第五项) -->
          <div v-else-if="activeSection === 'groups'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'groups'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>

            <div class="section-title-bar">
              <div>
                <h3 :class="{ 'text-danger': erroneousSection === 'groups' }">
                  {{ t('config.section.groups') }}
                  <span v-if="erroneousSection === 'groups'" class="section-error-pill">
                    <AlertTriangle :size="13" /> 存在语法错误
                  </span>
                </h3>
                <p>{{ t('config.groups_desc') }}</p>
              </div>
              <button class="btn btn-primary" @click="openAddGroup">
                <Plus :size="14" /> {{ t('config.new_group') }}
              </button>
            </div>

            <div v-if="configModel.groups.length === 0" class="empty-state-box">
              <Layers class="empty-icon" :size="36" />
              <div class="empty-text">{{ t('config.groups_empty') }}</div>
              <p class="empty-subtext">{{ t('config.groups_empty_sub') }}</p>
              <button class="btn btn-primary btn-sm" @click="openAddGroup">
                <Plus :size="13" /> {{ t('config.new_group') }}
              </button>
            </div>

            <div v-else class="cards-grid">
              <div v-for="group in configModel.groups" :key="group.name" class="item-card group-card">
                <div class="card-top">
                  <span class="group-badge">Group</span>
                  <span class="card-key">{{ group.name }}</span>
                  <div class="card-actions">
                    <button class="btn-icon" @click="openEditGroup(group)" :title="t('common.edit')">
                      <Edit2 :size="13" />
                    </button>
                    <button class="btn-icon delete" @click="deleteGroup(group.name)" :title="t('common.delete')">
                      <Trash2 :size="13" />
                    </button>
                  </div>
                </div>
                <div class="card-info">
                  <div class="row">
                    <span class="lbl">{{ t('config.field.group_policy') }}:</span>
                    <span class="val code-tag">{{ group.policy || 'default' }}</span>
                  </div>
                  <div class="filter-list">
                    <span class="lbl">{{ t('config.field.group_filters') }}:</span>
                    <div class="chips-wrap">
                      <span v-for="(f, i) in group.filters" :key="i" class="filter-chip">
                        {{ f }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Section 6: Routing (按官方文档第六项) -->
          <div v-else-if="activeSection === 'routing'" class="form-section-body">
            <!-- Erroneous Module Banner -->
            <div v-if="erroneousSection === 'routing'" class="module-syntax-error-banner mb-4">
              <div class="banner-icon-col">
                <AlertTriangle :size="20" class="text-danger flex-shrink-0" />
              </div>
              <div class="banner-body">
                <div class="banner-title text-danger">
                  <strong>{{ t('config.module_error_title') }}</strong>
                  <span v-if="errorLineInfo?.line" class="error-line-badge">第 {{ errorLineInfo.line }} 行</span>
                </div>
                <div class="banner-msg mono">{{ validationStatus.message || errorLineInfo?.message }}</div>
              </div>
              <button class="btn btn-sm btn-outline-danger btn-jump-code" @click="switchToCodeAndLocate(errorLineInfo?.line)">
                <Code :size="13" />
                <span>{{ t('config.btn_locate_in_code') }}</span>
              </button>
            </div>

            <div class="section-title-bar">
              <div>
                <div class="title-with-badge">
                  <h3 :class="{ 'text-danger': erroneousSection === 'routing' }">
                    {{ t('config.section.routing') }}
                    <span v-if="erroneousSection === 'routing'" class="section-error-pill">
                      <AlertTriangle :size="13" /> 存在语法错误
                    </span>
                  </h3>
                  <span
                    v-if="routingHealthReport.hasErrors"
                    class="health-badge danger"
                  >
                    <ShieldAlert :size="12" /> {{ routingHealthReport.conflicts.filter(c => c.severity === 'error').length }}
                  </span>
                  <span
                    v-else-if="routingHealthReport.hasWarnings"
                    class="health-badge warning"
                  >
                    <AlertTriangle :size="12" /> {{ routingHealthReport.conflicts.filter(c => c.severity === 'warning').length }}
                  </span>
                  <span
                    v-else-if="configModel.routingRules.length > 0"
                    class="health-badge success"
                  >
                    <ShieldCheck :size="12" />
                  </span>
                </div>
                <p>{{ t('config.rules_desc') }}</p>
              </div>
              <div class="title-actions">
                <button class="btn btn-primary" @click="openAddRoute">
                  <Plus :size="14" /> {{ t('config.new_rule') }}
                </button>
              </div>
            </div>

            <!-- Conflict & Health Diagnostic Panel -->
            <div v-if="configModel.routingRules.length > 0" class="conflict-panel-wrap">
              <!-- Case 1: Healthy banner -->
              <div v-if="!routingHealthReport.hasErrors && !routingHealthReport.hasWarnings && routingHealthReport.conflicts.length === 0" class="health-summary-banner success">
                <ShieldCheck class="banner-icon" :size="18" />
                <div class="banner-text">
                  <span class="banner-title">{{ t('routing.conflict_healthy') }}</span>
                  <span class="banner-sub">{{ t('routing.health_ok_desc', { count: configModel.routingRules.length }) }}</span>
                </div>
              </div>

              <!-- Case 2: Conflicts or optimization suggestions -->
              <div v-else class="conflict-card" :class="{ 'has-error': routingHealthReport.hasErrors }">
                <div class="conflict-card-header" @click="showConflictDetails = !showConflictDetails">
                  <div class="header-left">
                    <ShieldAlert v-if="routingHealthReport.hasErrors" class="status-icon danger" :size="20" />
                    <AlertTriangle v-else class="status-icon warning" :size="20" />
                    <div>
                      <h4 class="conflict-card-title">
                        {{ t('routing.conflict_report_title') }}
                        <span class="conflict-count-pill">{{ t('routing.conflict_items_count', { count: routingHealthReport.conflicts.length }) }}</span>
                      </h4>
                      <p class="conflict-card-subtitle">
                        {{ t('routing.conflict_report_desc') }}
                      </p>
                    </div>
                  </div>
                  <button class="btn-icon-subtle toggle-collapse-btn" :title="showConflictDetails ? t('common.collapse') : t('common.expand')">
                    <ChevronUp v-if="showConflictDetails" :size="16" />
                    <ChevronDown v-else :size="16" />
                  </button>
                </div>

                <div v-if="showConflictDetails" class="conflict-list">
                  <div
                    v-for="conflict in routingHealthReport.conflicts"
                    :key="conflict.id"
                    class="conflict-item"
                    :class="conflict.severity"
                  >
                    <div class="conflict-item-header">
                      <span class="sev-tag" :class="conflict.severity">
                        {{ conflict.severity === 'error' ? t('routing.diag_error') : (conflict.severity === 'warning' ? t('routing.diag_warning') : t('routing.diag_suggestion')) }}
                      </span>
                      <span class="conflict-name">{{ conflict.title }}</span>
                      <div v-if="conflict.ruleIndices.length > 0" class="involved-rules">
                        <span class="lbl-involved">{{ t('routing.diag_involved_lines') }}:</span>
                        <button
                          v-for="idx in conflict.ruleIndices"
                          :key="idx"
                          class="rule-link-chip"
                          @click="highlightRule(idx)"
                          :title="t('routing.diag_highlight_tip')"
                        >
                          #{{ idx + 1 }}
                        </button>
                      </div>
                    </div>
                    <div class="conflict-desc">{{ conflict.message }}</div>
                    <div class="conflict-suggestion">
                      <Lightbulb class="sugg-icon" :size="14" />
                      <div class="sugg-content">
                        <span class="sugg-label">{{ t('routing.diag_sugg_label') }}:</span>
                        <span>{{ conflict.suggestion }}</span>
                      </div>
                      <!-- Quick action buttons -->
                      <div class="quick-fix-actions">
                        <button
                          v-if="conflict.id === 'missing-private-bypass'"
                          class="btn btn-sm btn-quick-fix"
                          @click="fixMissingPrivateBypass"
                          :title="t('routing.diag_fix_private')"
                        >
                          <Wand2 :size="12" /> {{ t('routing.diag_fix_private') }}
                        </button>
                        <button
                          v-if="conflict.id === 'fallback-not-last'"
                          class="btn btn-sm btn-quick-fix"
                          @click="fixFallbackOrder"
                          :title="t('routing.diag_fix_fallback')"
                        >
                          <Wand2 :size="12" /> {{ t('routing.diag_fix_fallback') }}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="configModel.routingRules.length === 0" class="empty-state-box">
              <GitFork class="empty-icon" :size="36" />
              <div class="empty-text">{{ t('routing.empty_title') }}</div>
              <p class="empty-subtext">{{ t('routing.empty_desc') }}</p>
              <button class="btn btn-primary btn-sm" @click="openAddRoute">
                <Plus :size="13" /> {{ t('config.new_rule') }}
              </button>
            </div>

            <div v-else class="rule-table-wrap">
              <table class="rule-table">
                <thead>
                  <tr>
                    <th style="width: 60px">#</th>
                    <th>{{ t('routing.col_condition') }}</th>
                    <th style="width: 150px">{{ t('routing.col_action') }}</th>
                    <th style="width: 110px">{{ t('routing.col_status') }}</th>
                    <th style="width: 160px; text-align: right">{{ t('routing.col_order') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(rule, idx) in configModel.routingRules"
                    :key="idx"
                    :id="`route-row-${idx}`"
                    :class="{
                      'row-highlighted': highlightedRuleIndex === idx,
                      'row-error': routingHealthReport.ruleStatusMap[idx]?.severity === 'error',
                      'row-warning': routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'
                    }"
                  >
                    <td class="idx-cell">
                      <span class="idx-num">{{ idx + 1 }}</span>
                    </td>
                    <td class="mono cond-cell">
                      <span v-if="rule.condition === 'fallback'" class="fallback-tag">{{ t('routing.drag_tip_fallback') }}</span>
                      <span v-else class="rule-condition-text">{{ rule.condition }}</span>
                    </td>
                    <td>
                      <span class="target-tag" :class="rule.target">{{ rule.target }}</span>
                    </td>
                    <td>
                      <span
                        v-if="routingHealthReport.ruleStatusMap[idx]?.severity === 'error'"
                        class="rule-health-tag danger"
                        :title="routingHealthReport.ruleStatusMap[idx]?.messages.join('\n')"
                      >
                        <ShieldAlert :size="12" /> {{ t('routing.status_conflict') }}
                      </span>
                      <span
                        v-else-if="routingHealthReport.ruleStatusMap[idx]?.severity === 'warning'"
                        class="rule-health-tag warning"
                        :title="routingHealthReport.ruleStatusMap[idx]?.messages.join('\n')"
                      >
                        <AlertTriangle :size="12" /> {{ t('routing.status_shadow') }}
                      </span>
                      <span v-else class="rule-health-tag ok" :title="t('routing.status_ok')">
                        <Check :size="12" /> {{ t('routing.status_ok') }}
                      </span>
                    </td>
                    <td style="text-align: right" class="row-action-cell">
                      <button
                        class="btn-icon"
                        :disabled="idx === 0"
                        @click="moveRouteUp(idx)"
                        :title="t('routing.table_tip_priority')"
                      >
                        <ArrowUp :size="12" />
                      </button>
                      <button
                        class="btn-icon"
                        :disabled="idx === configModel.routingRules.length - 1"
                        @click="moveRouteDown(idx)"
                        :title="t('routing.table_tip_priority')"
                      >
                        <ArrowDown :size="12" />
                      </button>
                      <button
                        class="btn-icon"
                        @click="duplicateRoute(idx)"
                        :title="t('common.copy')"
                      >
                        <Copy :size="12" />
                      </button>
                      <button
                        class="btn-icon"
                        @click="openEditRoute(rule, idx)"
                        :title="t('common.edit')"
                      >
                        <Edit2 :size="12" />
                      </button>
                      <button
                        class="btn-icon delete"
                        @click="deleteRoute(idx)"
                        :title="t('common.delete')"
                      >
                        <Trash2 :size="12" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <!-- 2. Code Editor Mode -->
        <div v-show="currentMode === 'code'" class="editor-wrapper">
          <!-- Error banner in code mode -->
          <div v-if="errorLineInfo && errorLineInfo.line" class="code-editor-error-banner">
            <div class="banner-body-text">
              <AlertTriangle :size="16" class="text-danger flex-shrink-0" />
              <span class="font-bold text-danger">{{ t('config.code_error_line_tip', { line: errorLineInfo.line }) }}：</span>
              <span class="banner-error-msg mono truncate" :title="errorLineInfo.message">{{ errorLineInfo.message }}</span>
            </div>
            <button class="btn btn-xs btn-outline-danger btn-jump" @click="highlightErrorLineInEditor(errorLineInfo.line)">
              <Crosshair :size="13" />
              <span>{{ t('config.code_btn_jump_error') }}</span>
            </button>
          </div>
          <div ref="editorContainer" class="editor-host"></div>
        </div>
      </div>

      <!-- Status Bar Footer -->
      <footer class="status-bar">
        <div class="status-left">
          <div
            v-if="validationStatus.checked"
            class="val-indicator"
            :class="{ 'valid': validationStatus.valid, 'invalid': !validationStatus.valid }"
          >
            <CheckCircle2 v-if="validationStatus.valid" class="icon" :size="14" />
            <AlertCircle v-else class="icon" :size="14" />
            <span class="val-msg">{{ validationStatus.message }}</span>
          </div>
          <div v-else class="val-indicator neutral">
            <span class="val-msg">{{ t('config.val_not_checked') }}</span>
          </div>
        </div>

        <div class="status-right">
          <span>{{ t('config.last_modified', { time: formatDate(lastModTime) }) }}</span>
          <button class="btn-text" @click="() => loadConfig()" :title="t('common.reload_disk')">
            <RefreshCw class="icon" :size="12" /> {{ t('common.reload') }}
          </button>
        </div>
      </footer>
    </div>

    <!-- ==================== TOP-LEVEL MODALS ==================== -->

    <!-- Node Modal -->
    <div v-if="showNodeModal" class="modal-backdrop" @click.self="showNodeModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{{ nodeModalMode === 'add' ? t('config.modal_add_node') : t('config.modal_edit_node') }}</h3>
          <button class="close-btn" @click="showNodeModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>{{ t('config.field.node_key') }}</label>
            <input v-model="editingNode.key" :disabled="nodeModalMode === 'edit'" :placeholder="t('config.field.node_key_ph')" />
          </div>
          <div class="field">
            <label>{{ t('config.field.node_link') }}</label>
            <textarea v-model="editingNode.url" rows="3" :placeholder="t('config.field.node_link_ph')"></textarea>
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showNodeModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="saveNodeModal">{{ t('common.save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Group Modal (Interactive with Subscriptions, Nodes and Auto Fixed(N) Code) -->
    <div v-if="showGroupModal" class="modal-backdrop" @click.self="showGroupModal = false">
      <div class="modal-card modal-lg group-editor-modal">
        <div class="modal-header">
          <div class="modal-title-wrap">
            <Layers class="modal-title-icon" :size="20" />
            <div>
              <h3>{{ groupModalMode === 'add' ? '新增出站策略分组' : `编辑策略分组: ${editingGroup.name}` }}</h3>
              <span class="modal-subtitle">选择订阅源或节点联动范围，并按需自动生成 fixed(N) 固定节点绑定代码</span>
            </div>
          </div>
          <button class="close-btn" @click="showGroupModal = false">&times;</button>
        </div>

        <div class="modal-body form-modal group-modal-body">
          <!-- Step 1: 分组名称 -->
          <div class="form-step-card">
            <div class="step-card-header">
              <span class="step-badge">1</span>
              <h4>分组基本标识</h4>
            </div>
            <div class="field mb-2">
              <label>{{ t('config.field.group_name') }} <span class="text-danger">*</span></label>
              <input
                v-model="editingGroup.name"
                :disabled="groupModalMode === 'edit'"
                :placeholder="t('config.field.group_name_ph')"
                class="form-control"
              />
            </div>
            <!-- Quick preset names -->
            <div v-if="groupModalMode === 'add'" class="quick-names-row">
              <span class="quick-names-lbl">{{ t('config.group.quick_names') }}:</span>
              <button
                v-for="qn in ['proxy', 'streaming', 'japan', 'game', 'backup', 'auto']"
                :key="qn"
                type="button"
                class="quick-name-btn"
                :class="{ active: editingGroup.name === qn }"
                @click="editingGroup.name = qn"
              >
                {{ qn }}
              </button>
            </div>
          </div>

          <!-- Step 2: 节点来源联动 -->
          <div class="form-step-card">
            <div class="step-card-header">
              <span class="step-badge">2</span>
              <h4>{{ t('config.group.step1_title') }}</h4>
            </div>

            <!-- Source Type Switcher -->
            <div class="source-type-switcher">
              <button
                type="button"
                class="source-tab-btn"
                :class="{ active: editingGroup.sourceType === 'sub' }"
                @click="editingGroup.sourceType = 'sub'"
              >
                <Radio class="tab-icon" :size="14" />
                <span>{{ t('config.group.source_sub') }}</span>
              </button>
              <button
                type="button"
                class="source-tab-btn"
                :class="{ active: editingGroup.sourceType === 'nodes' }"
                @click="editingGroup.sourceType = 'nodes'"
              >
                <Server class="tab-icon" :size="14" />
                <span>{{ t('config.group.source_nodes') }}</span>
              </button>
              <button
                type="button"
                class="source-tab-btn"
                :class="{ active: editingGroup.sourceType === 'custom' }"
                @click="editingGroup.sourceType = 'custom'"
              >
                <Code class="tab-icon" :size="14" />
                <span>{{ t('config.group.source_custom') }}</span>
              </button>
            </div>

            <!-- Option A: Subscription linkage -->
            <div v-if="editingGroup.sourceType === 'sub'" class="source-content-box">
              <div v-if="availableSubscriptions.length === 0" class="empty-hint-box">
                <AlertCircle :size="16" class="text-amber" />
                <span>{{ t('config.group.no_subs') }}</span>
              </div>
              <div v-else class="field">
                <label>{{ t('config.group.select_sub') }}:</label>
                <select v-model="editingGroup.selectedSubTag" class="form-control">
                  <option
                    v-for="s in availableSubscriptions"
                    :key="s.tag"
                    :value="s.tag"
                  >
                    📡 {{ s.tag }} ({{ s.nodeCount > 0 ? `共 ${s.nodeCount} 个节点` : '订阅源' }})
                  </option>
                </select>
                <div class="sub-matched-stats">
                  <span class="stat-pill">
                    {{ t('config.group.candidate_count', { count: groupCandidateNodes.length }) }}
                  </span>
                  <span class="filter-code-preview mono">生成语法: subtag({{ editingGroup.selectedSubTag }})</span>
                </div>
              </div>
            </div>

            <!-- Option B: Specific Nodes selection -->
            <div v-else-if="editingGroup.sourceType === 'nodes'" class="source-content-box">
              <div class="nodes-select-toolbar">
                <div class="search-input-wrap">
                  <Search :size="14" class="search-icon" />
                  <input
                    v-model="nodeSearchQuery"
                    class="search-mini"
                    placeholder="搜索节点名称 / 协议 / IP..."
                  />
                </div>
                <div class="nodes-select-actions">
                  <button type="button" class="btn-xs" @click="selectAllFilteredNodes">全选匹配</button>
                  <button type="button" class="btn-xs" @click="clearSelectedNodes">清空</button>
                  <span class="selected-count-badge">已选 {{ editingGroup.selectedNodeKeys.length }} 个节点</span>
                </div>
              </div>

              <div class="nodes-checkbox-list">
                <label
                  v-for="n in filteredAllNodes"
                  :key="n.key"
                  class="node-check-item"
                  :class="{ selected: editingGroup.selectedNodeKeys.includes(n.key) }"
                >
                  <input
                    type="checkbox"
                    :checked="editingGroup.selectedNodeKeys.includes(n.key)"
                    @change="toggleSelectNodeKey(n.key)"
                  />
                  <span class="node-flag">{{ n.flag || '🛡️' }}</span>
                  <span class="node-name truncate">{{ n.name || n.key }}</span>
                  <span class="node-proto-badge">{{ n.protocol }}</span>
                  <span class="node-addr truncate">{{ n.address }}:{{ n.port }}</span>
                </label>
              </div>
            </div>

            <!-- Option C: Custom filter syntax -->
            <div v-else class="source-content-box">
              <label>{{ t('config.field.group_filters') }}</label>
              <textarea
                v-model="editingGroup.customFilterText"
                rows="3"
                class="form-control mono"
                :placeholder="t('config.field.group_filters_ph')"
              ></textarea>
            </div>
          </div>

          <!-- Step 3: 选择调度模式与固定节点联动 -->
          <div class="form-step-card">
            <div class="step-card-header">
              <span class="step-badge">3</span>
              <h4>{{ t('config.group.step2_title') }}</h4>
            </div>

            <div class="policy-modes-grid">
              <label
                class="policy-mode-card"
                :class="{ active: editingGroup.policyMode === 'min_moving_avg' }"
              >
                <input type="radio" value="min_moving_avg" v-model="editingGroup.policyMode" />
                <div class="mode-info">
                  <strong class="mode-title">min_moving_avg</strong>
                  <span class="mode-desc">最小移动平均延迟 (推荐, 动态防抖最佳选路)</span>
                </div>
              </label>

              <label
                class="policy-mode-card"
                :class="{ active: editingGroup.policyMode === 'min_avg10' }"
              >
                <input type="radio" value="min_avg10" v-model="editingGroup.policyMode" />
                <div class="mode-info">
                  <strong class="mode-title">min_avg10</strong>
                  <span class="mode-desc">最近 10 次平均延迟最小</span>
                </div>
              </label>

              <label
                class="policy-mode-card"
                :class="{ active: editingGroup.policyMode === 'min' }"
              >
                <input type="radio" value="min" v-model="editingGroup.policyMode" />
                <div class="mode-info">
                  <strong class="mode-title">min</strong>
                  <span class="mode-desc">绝对最低瞬时延迟</span>
                </div>
              </label>

              <label
                class="policy-mode-card"
                :class="{ active: editingGroup.policyMode === 'random' }"
              >
                <input type="radio" value="random" v-model="editingGroup.policyMode" />
                <div class="mode-info">
                  <strong class="mode-title">random</strong>
                  <span class="mode-desc">随机均衡调度节点</span>
                </div>
              </label>

              <label
                class="policy-mode-card highlight-fixed"
                :class="{ active: editingGroup.policyMode === 'fixed' }"
              >
                <input type="radio" value="fixed" v-model="editingGroup.policyMode" />
                <div class="mode-info">
                  <strong class="mode-title">fixed(N) 固定节点模式 🎯</strong>
                  <span class="mode-desc">选择固定某个节点，自动生成数字序号代码</span>
                </div>
              </label>
            </div>

            <!-- 固定节点联动选择区 (如果选择 fix() 固定节点模式，选择某个节点，自动生成对应数字代码) -->
            <div v-if="editingGroup.policyMode === 'fixed'" class="fixed-node-picker-box">
              <div class="picker-header">
                <Crosshair :size="16" class="text-primary" />
                <strong>{{ t('config.group.step3_title') }}</strong>
                <span class="text-muted">(点击下方候选节点，自动生成数字序号代码)</span>
              </div>

              <div v-if="groupCandidateNodes.length === 0" class="no-candidate-warn">
                <AlertCircle :size="16" class="text-amber" />
                <span>当前节点来源暂无候选节点，请检查上方节点来源设置</span>
              </div>

              <div v-else class="candidate-nodes-list">
                <div
                  v-for="(node, idx) in groupCandidateNodes"
                  :key="node.key"
                  class="candidate-node-row"
                  :class="{ active: editingGroup.fixedNodeIndex === idx }"
                  @click="editingGroup.fixedNodeIndex = idx"
                >
                  <div class="row-left">
                    <span class="index-badge">[{{ idx }}]</span>
                    <span class="flag-icon">{{ node.flag || '🛡️' }}</span>
                    <strong class="node-name truncate">{{ node.name || node.key }}</strong>
                    <span class="proto-tag">{{ node.protocol }}</span>
                  </div>
                  <div class="row-right">
                    <span class="node-addr truncate">{{ node.address }}:{{ node.port }}</span>
                    <span v-if="node.latency" class="latency-pill">{{ node.latency }}ms</span>
                    <span v-if="editingGroup.fixedNodeIndex === idx" class="bound-tag">✓ 已绑定 [{{ idx }}]</span>
                  </div>
                </div>
              </div>

              <!-- Fixed Code Auto-generation Confirmation Banner -->
              <div class="fixed-code-banner">
                <Zap :size="16" class="banner-icon" />
                <div class="banner-content">
                  <strong>{{ t('config.group.fixed_hint', { code: `fixed(${editingGroup.fixedNodeIndex})`, index: editingGroup.fixedNodeIndex, name: groupCandidateNodes[editingGroup.fixedNodeIndex]?.name || groupCandidateNodes[editingGroup.fixedNodeIndex]?.key || '节点' }) }}</strong>
                </div>
              </div>
            </div>
          </div>

          <!-- Step 4: 配置代码实时预览 -->
          <div class="form-step-card preview-card">
            <div class="step-card-header">
              <Code class="step-icon" :size="15" />
              <h4>{{ t('config.group.preview_title') }}</h4>
            </div>
            <pre class="dae-preview-code mono">{{ groupDaePreview }}</pre>
          </div>

          <!-- Actions -->
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showGroupModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="saveGroupModal">{{ t('common.save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Route Rule Modal -->
    <div v-if="showRouteModal" class="modal-backdrop" @click.self="showRouteModal = false">
      <div class="modal-card modal-lg">
        <div class="modal-header">
          <div class="modal-title-with-tabs">
            <h3>{{ routeModalMode === 'add' ? t('routing.editor_title_add') : t('routing.editor_title_edit') }}</h3>
            <div class="modal-subtabs">
              <button
                class="subtab-btn"
                :class="{ active: routeModalTab === 'form' }"
                @click="routeModalTab = 'form'"
              >
                <Sliders :size="13" />
                <span>{{ t('config.route_tab_config') }}</span>
              </button>
              <button
                class="subtab-btn"
                :class="{ active: routeModalTab === 'guide' }"
                @click="routeModalTab = 'guide'"
              >
                <BookOpen :size="13" />
                <span>{{ t('config.route_tab_guide') }}</span>
              </button>
            </div>
          </div>
          <button class="close-btn" @click="showRouteModal = false">&times;</button>
        </div>

        <!-- Tab 1: Form Configuration -->
        <div v-if="routeModalTab === 'form'" class="modal-body form-modal">
          <!-- Input Mode Switcher -->
          <div class="route-input-tabs">
            <button
              class="tab-pill"
              :class="{ active: routeInputMode === 'preset' }"
              @click="routeInputMode = 'preset'"
            >
              <Sparkles :size="12" /> {{ t('routing.editor_mode_preset') }}
            </button>
            <button
              class="tab-pill"
              :class="{ active: routeInputMode === 'builder' }"
              @click="routeInputMode = 'builder'"
            >
              <Sliders :size="12" /> {{ t('routing.editor_mode_builder') }}
            </button>
            <button
              class="tab-pill"
              :class="{ active: routeInputMode === 'custom' }"
              @click="routeInputMode = 'custom'"
            >
              <FileCode :size="12" /> {{ t('routing.editor_mode_custom') }}
            </button>
          </div>

          <!-- Mode A: Presets -->
          <div v-if="routeInputMode === 'preset'" class="preset-section">
            <div class="field">
              <label>{{ t('routing.select_preset_label') }}</label>
              <select v-model="selectedPresetId" @change="applyPreset(selectedPresetId)">
                <option v-for="p in ROUTING_PRESETS" :key="p.id" :value="p.id">
                  {{ p.name }} ({{ p.description }})
                </option>
              </select>
              <div class="preset-tip-box" v-if="ROUTING_PRESETS.find(x => x.id === selectedPresetId)?.tip">
                <Info :size="13" />
                <span>{{ ROUTING_PRESETS.find(x => x.id === selectedPresetId)?.tip }}</span>
              </div>
            </div>
          </div>

          <!-- Mode B: Visual Condition Builder -->
          <div v-else-if="routeInputMode === 'builder'" class="builder-section">
            <div class="builder-items-list">
              <div
                v-for="(item, bIdx) in builderItems"
                :key="bIdx"
                class="builder-row"
              >
                <!-- Negate check -->
                <label class="builder-not-check" :title="t('routing.builder_not')">
                  <input
                    type="checkbox"
                    v-model="item.not"
                    @change="updateConditionFromBuilder"
                  />
                  <span>{{ t('routing.builder_not') }}</span>
                </label>

                <!-- Type select -->
                <select
                  v-model="item.type"
                  class="builder-type-select"
                  @change="updateConditionFromBuilder"
                >
                  <option value="domain">{{ t('routing.type_domain') }}</option>
                  <option value="dip">{{ t('routing.type_dip') }}</option>
                  <option value="dport">{{ t('routing.type_dport') }}</option>
                  <option value="sip">{{ t('routing.type_sip') }}</option>
                  <option value="sport">{{ t('routing.type_sport') }}</option>
                  <option value="l4proto">{{ t('routing.type_l4proto') }}</option>
                  <option value="ipversion">{{ t('routing.type_ipver_opt') }}</option>
                  <option value="pname">{{ t('routing.type_pname') }}</option>
                  <option value="mac">{{ t('routing.type_mac') }}</option>
                </select>

                <!-- Domain modifiers -->
                <select
                  v-if="item.type === 'domain'"
                  v-model="item.modifier"
                  class="builder-mod-select"
                  @change="updateConditionFromBuilder"
                >
                  <option value="geosite">{{ t('routing.sub_geosite_opt') }}</option>
                  <option value="suffix">{{ t('routing.sub_suffix_opt') }}</option>
                  <option value="keyword">{{ t('routing.sub_keyword_opt') }}</option>
                  <option value="full">{{ t('routing.sub_full_opt') }}</option>
                  <option value="regex">{{ t('routing.sub_regex_opt') }}</option>
                  <option value="none">{{ t('routing.sub_domain_default_opt') }}</option>
                </select>

                <!-- Dip modifiers -->
                <select
                  v-else-if="item.type === 'dip'"
                  v-model="item.modifier"
                  class="builder-mod-select"
                  @change="updateConditionFromBuilder"
                >
                  <option value="geoip">{{ t('routing.sub_geoip_opt') }}</option>
                  <option value="none">{{ t('routing.sub_ip_cidr_opt') }}</option>
                </select>

                <!-- Parameter input -->
                <div class="builder-val-box">
                  <input
                    type="text"
                    v-model="item.value"
                    @input="updateConditionFromBuilder"
                    :placeholder="
                      item.type === 'domain' ? (item.modifier === 'geosite' ? t('routing.ph_domain_geosite') : t('routing.ph_domain_suffix')) :
                      item.type === 'dip' ? (item.modifier === 'geoip' ? t('routing.ph_dip_geoip') : t('routing.ph_dip_cidr')) :
                      item.type === 'dport' ? t('routing.ph_dport') :
                      item.type === 'sip' ? t('routing.ph_sip') :
                      item.type === 'pname' ? t('routing.ph_pname') :
                      item.type === 'mac' ? t('routing.ph_mac') : '...'
                    "
                    class="builder-val-input"
                  />
                  <!-- Quick chips for fast completion -->
                  <div class="quick-chips">
                    <template v-if="item.type === 'domain' && item.modifier === 'geosite'">
                      <span class="chip" @click="item.value = 'cn'; updateConditionFromBuilder()">cn</span>
                      <span class="chip" @click="item.value = 'google'; updateConditionFromBuilder()">google</span>
                      <span class="chip" @click="item.value = 'category-ads-all'; updateConditionFromBuilder()">ads-all</span>
                      <span class="chip" @click="item.value = 'github'; updateConditionFromBuilder()">github</span>
                    </template>
                    <template v-else-if="item.type === 'dip' && item.modifier === 'geoip'">
                      <span class="chip" @click="item.value = 'private'; updateConditionFromBuilder()">private</span>
                      <span class="chip" @click="item.value = 'cn'; updateConditionFromBuilder()">cn</span>
                      <span class="chip" @click="item.value = 'us'; updateConditionFromBuilder()">us</span>
                    </template>
                    <template v-else-if="item.type === 'dport'">
                      <span class="chip" @click="item.value = '80, 443'; updateConditionFromBuilder()">80, 443</span>
                      <span class="chip" @click="item.value = '53'; updateConditionFromBuilder()">53 (DNS)</span>
                      <span class="chip" @click="item.value = '22'; updateConditionFromBuilder()">22 (SSH)</span>
                    </template>
                  </div>
                </div>

                <!-- Delete row button -->
                <button
                  type="button"
                  class="btn-icon delete"
                  :disabled="builderItems.length <= 1"
                  @click="removeBuilderItem(bIdx)"
                  :title="t('routing.btn_del_condition')"
                >
                  <Trash2 :size="13" />
                </button>
              </div>

              <button
                type="button"
                class="btn btn-secondary btn-sm btn-add-cond"
                @click="addBuilderItem"
              >
                <Plus :size="12" /> {{ t('routing.builder_btn_add_and') }}
              </button>
            </div>
          </div>

          <!-- Mode C: Custom Raw Code -->
          <div v-else class="custom-section">
            <div class="field">
              <label>{{ t('routing.custom_expr_label') }}</label>
              <input
                v-model="editingRoute.condition"
                :placeholder="t('routing.custom_expr_ph')"
                class="mono-input"
              />
              <span class="help-text">{{ t('routing.custom_expr_hint') }}</span>
            </div>
          </div>

          <!-- Live dae syntax preview -->
          <div class="syntax-preview-box">
            <span class="preview-label">{{ t('routing.stmt_title') }}:</span>
            <code class="preview-code">
              {{ editingRoute.condition.trim() === 'fallback' ? `fallback: ${editingRoute.target === '__custom__' ? (editingRoute.customTarget || '[target]') : editingRoute.target}` : `${editingRoute.condition || '[condition]'} -> ${editingRoute.target === '__custom__' ? (editingRoute.customTarget || '[target]') : editingRoute.target}` }}
            </code>
          </div>

          <!-- Target Selection -->
          <div class="field">
            <label>{{ t('routing.target_action_label') }}</label>
            <select v-model="editingRoute.target" class="target-select">
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
          </div>

          <div v-if="editingRoute.target === '__custom__'" class="field">
            <label>{{ t('routing.custom_target_label') }}</label>
            <input v-model="editingRoute.customTarget" :placeholder="t('routing.custom_target_ph')" />
          </div>

          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showRouteModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="saveRouteModal">{{ t('routing.btn_save_rule') }}</button>
          </div>
        </div>

        <!-- Tab 2: Syntax Guide & Examples -->
        <div v-else class="modal-body guide-modal">
          <div class="guide-banner">
            <div class="banner-title"><BookOpen :size="15" /> {{ t('routing.guide_title') }}</div>
            <p>{{ t('routing.guide_desc') }}</p>
          </div>

          <div class="syntax-docs-grid">
            <div
              v-for="(sec, sIdx) in ROUTING_SYNTAX_GUIDE"
              :key="sIdx"
              class="syntax-card"
            >
              <div class="syntax-card-head">
                <h4>{{ sec.title }}</h4>
                <code class="syntax-tag">{{ sec.syntax }}</code>
              </div>
              <p class="syntax-summary">{{ sec.summary }}</p>
              <div class="syntax-examples-wrap">
                <span class="ex-lbl">{{ t('routing.fast_tags_hint') }}:</span>
                <div
                  v-for="(ex, eIdx) in sec.examples"
                  :key="eIdx"
                  class="syntax-example-row"
                >
                  <code class="ex-code">{{ ex.code }}</code>
                  <span class="ex-explain">{{ ex.explain }}</span>
                  <button
                    class="btn-text-xs"
                    @click="applyExampleToRoute(ex)"
                    :title="t('routing.guide_apply_tip')"
                  >
                    <Copy :size="10" /> {{ t('routing.guide_btn_apply') }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-actions">
            <button class="btn btn-primary" @click="routeModalTab = 'form'">{{ t('common.back') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Subscription Modal -->
    <div v-if="showSubModal" class="modal-backdrop" @click.self="showSubModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{{ t('config.new_sub') }}</h3>
          <button class="close-btn" @click="showSubModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>{{ t('config.field.sub_tag') }}</label>
            <input v-model="editingSub.tag" :placeholder="t('config.field.sub_tag_ph')" />
          </div>
          <div class="field">
            <label>{{ t('config.field.sub_url') }}</label>
            <input v-model="editingSub.url" :placeholder="t('config.field.sub_url_ph')" />
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showSubModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="saveSubModal">{{ t('common.save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Backup Modal -->
    <div v-if="showBackupModal" class="modal-backdrop" @click.self="showBackupModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3><Archive class="icon" :size="18" /> {{ t('config.backup_modal_title') }}</h3>
          <button class="close-btn" @click="showBackupModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="create-backup-bar">
            <input
              type="text"
              v-model="newBackupLabel"
              :placeholder="t('config.backup_label_ph')"
              @keydown.enter="createManualBackup"
            />
            <button class="btn btn-primary" @click="createManualBackup">{{ t('config.create_backup_btn') }}</button>
          </div>

          <div class="backup-list-box">
            <div v-if="backups.length === 0" class="empty-state">{{ t('config.no_backups') }}</div>
            <table v-else class="backup-table">
              <thead>
                <tr>
                  <th>{{ t('config.backup_col_name') }}</th>
                  <th>{{ t('config.backup_col_time') }}</th>
                  <th>{{ t('config.backup_col_size') }}</th>
                  <th style="text-align: right">{{ t('common.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="b in backups" :key="b.name">
                  <td class="name-cell">{{ b.name }}</td>
                  <td>{{ formatDate(b.modTime) }}</td>
                  <td>{{ formatBytes(b.size) }}</td>
                  <td class="action-cell">
                    <a :href="`/api/backups/${encodeURIComponent(b.name)}/download`" target="_blank" class="btn-icon">
                      <Download :size="14" />
                    </a>
                    <button class="btn-icon restore" @click="restoreBackup(b.name)"><RotateCcw :size="14" /></button>
                    <button class="btn-icon delete" @click="deleteBackup(b.name)"><Trash2 :size="14" /></button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- New Profile Modal (Name + Note Inputs) -->
    <div v-if="showNewProfileModal" class="modal-backdrop" @click.self="showNewProfileModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3><Files class="icon" :size="18" /> {{ t('config.new_modal_title') }}</h3>
          <button class="close-btn" @click="showNewProfileModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>{{ t('config.new_modal_name') }}</label>
            <input
              v-model="newProfileName"
              :placeholder="t('config.new_modal_name_ph')"
              class="form-control"
            />
            <span class="field-hint">{{ t('config.new_profile_hint') }}</span>
          </div>
          <div class="field mt-3">
            <label>{{ t('config.new_modal_note') }}</label>
            <input
              v-model="newProfileNote"
              :placeholder="t('config.new_modal_note_ph')"
              class="form-control"
              @keydown.enter="createNewProfile"
            />
          </div>
          <div class="modal-actions mt-4">
            <button class="btn btn-secondary" @click="showNewProfileModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="createNewProfile">{{ t('config.new_modal_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Copy Profile Modal -->
    <div v-if="showCopyProfileModal" class="modal-backdrop" @click.self="showCopyProfileModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3><Copy class="icon" :size="18" /> {{ t('config.copy_profile_title') }}</h3>
          <button class="close-btn" @click="showCopyProfileModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>{{ t('config.copy_profile_label') }}</label>
            <input
              v-model="copyProfileName"
              :placeholder="t('config.copy_profile_ph')"
              @keydown.enter="copyCurrentProfile"
            />
            <span class="field-hint">{{ t('config.copy_profile_hint', { name: currentSelectedConfig }) }}</span>
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showCopyProfileModal = false">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" @click="copyCurrentProfile">{{ t('config.btn_copy_profile') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Subscription Nodes Modal -->
    <div v-if="showSubNodesModal" class="modal-backdrop" @click.self="showSubNodesModal = false">
      <div class="modal-card modal-large">
        <div class="modal-header">
          <h3><ListFilter class="icon" :size="18" /> {{ t('config.sub_nodes_modal_title', { tag: currentSubTag }) }}</h3>
          <button class="close-btn" @click="showSubNodesModal = false">&times;</button>
        </div>
        <div class="modal-body sub-nodes-modal">
          <div v-if="fetchingSubNodes" class="sub-loading-box">
            <RefreshCw class="icon spin" :size="24" />
            <span>{{ t('config.sub_nodes_loading') }}</span>
          </div>
          <div v-else-if="subFetchedNodes.length === 0" class="empty-state">
            {{ t('config.sub_nodes_empty') }}
          </div>
          <div v-else class="sub-nodes-container">
            <div class="sub-nodes-toolbar">
              <div class="sub-summary">
                {{ t('config.sub_nodes_stats', { total: subFetchedNodes.length, selected: selectedSubNodesCount }) }}
              </div>
              <button class="btn-text" @click="toggleSelectAllSubNodes">
                <CheckSquare :size="14" /> {{ selectAllSubNodes ? t('common.deselect_all') : t('common.select_all') }}
              </button>
            </div>

            <div class="sub-nodes-table-wrap">
              <table class="sub-nodes-table">
                <thead>
                  <tr>
                    <th style="width: 38px"></th>
                    <th style="width: 80px">{{ t('common.protocol') }}</th>
                    <th>{{ t('common.node_name') }}</th>
                    <th>{{ t('common.server_address') }}</th>
                    <th style="width: 90px">{{ t('common.region') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(node, idx) in subFetchedNodes"
                    :key="idx"
                    :class="{ 'row-selected': node.selected }"
                    @click="node.selected = !node.selected"
                  >
                    <td class="checkbox-col" @click.stop>
                      <input type="checkbox" v-model="node.selected" />
                    </td>
                    <td>
                      <span class="proto-badge-sm">{{ node.protocol || 'NODE' }}</span>
                    </td>
                    <td class="name-cell bold">
                      <span class="truncate" :title="node.name">{{ node.name }}</span>
                    </td>
                    <td class="mono address-cell">
                      {{ node.address }}:{{ node.port }}
                    </td>
                    <td>
                      <span class="country-tag">{{ node.flag }} {{ node.country }}</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="modal-actions" style="margin-top: 14px">
              <button class="btn btn-secondary" @click="showSubNodesModal = false">{{ t('common.cancel') }}</button>
              <button
                class="btn btn-primary"
                :disabled="selectedSubNodesCount === 0"
                @click="importSelectedSubNodes"
              >
                <Plus :size="14" />
                <span>{{ t('config.btn_import_selected_nodes', { count: selectedSubNodesCount }) }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>



    <!-- Syntax Error Details Modal -->
    <div v-if="showSyntaxErrorModal && currentErrorProfile" class="modal-backdrop" @click.self="showSyntaxErrorModal = false">
      <div class="modal-card modal-md">
        <div class="modal-header">
          <div class="modal-title-wrap">
            <AlertTriangle class="text-danger" :size="18" />
            <h3>{{ t('config.modal_syntax_error_title') }} - {{ currentErrorProfile.name }}</h3>
          </div>
          <button class="close-btn" @click="showSyntaxErrorModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="alert-box-danger mb-3">
            <strong>{{ t('config.status_invalid') }}</strong>: 该配置文件未能通过 dae 核心引擎有效性校验，无法作为生效配置运行。请先修复语法错误。
          </div>
          <label class="form-label font-semibold mb-1">dae validate 语法错误信息:</label>
          <div class="syntax-error-console mono">
            <pre>{{ currentErrorProfile.errorMsg || '未能获取详细错误信息' }}</pre>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showSyntaxErrorModal = false">{{ t('common.close') }}</button>
          <button class="btn btn-secondary" @click="showSyntaxErrorModal = false; openEditProfile(currentErrorProfile.name)">
            <Edit2 :size="14" />
            <span>{{ t('config.action_edit') }}修复</span>
          </button>
          <button class="btn btn-primary" @click="showSyntaxErrorModal = false; openEditProfile(currentErrorProfile.name, undefined, true)">
            <Code :size="14" />
            <span>源码定位修复</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.config-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  transition: background-color 0.2s, color 0.2s;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--bg-card, #0f172a);
  border-bottom: 1px solid var(--border-color, #1e293b);
  gap: 12px;
  transition: background-color 0.2s, border-color 0.2s;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mode-switcher {
  display: flex;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  padding: 2px;
  gap: 2px;
}

.mode-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-btn.active {
  background: var(--bg-card, #1e293b);
  color: #0284c7;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.profile-selector-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.profile-select-wrapper {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  padding: 3px 8px;
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
  padding-right: 4px;
}

.profile-select option {
  background: var(--bg-card, #0f172a);
  color: var(--text-main, #e2e8f0);
}

.badge-active {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  border-radius: 4px;
  color: #10b981;
  font-size: 11px;
  font-weight: 500;
  user-select: none;
}

.active-indicator {
  width: 6px;
  height: 6px;
  background: #10b981;
  border-radius: 50%;
  box-shadow: 0 0 6px #10b981;
}

.btn-activate {
  background: #059669;
  color: #fff;
  border: none;
  padding: 4px 8px;
  font-size: 11px;
  border-radius: 4px;
}
.btn-activate:hover:not(:disabled) {
  background: #047857;
}

.btn-icon-subtle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 4px;
  border: 1px solid var(--border-color, #334155);
  background: var(--bg-input, #1e293b);
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-icon-subtle:hover {
  background: var(--border-color, #334155);
  color: var(--text-main, #f1f5f9);
}
.btn-icon-subtle.danger:hover {
  background: rgba(239, 68, 68, 0.2);
  border-color: rgba(239, 68, 68, 0.4);
  color: #ef4444;
}

.dirty-dot {
  color: #f59e0b;
  font-size: 10px;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.divider {
  width: 1px;
  height: 20px;
  background: var(--border-color, #334155);
  margin: 0 4px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  user-select: none;
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
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.btn-primary {
  background: #0284c7;
  color: #fff;
}
.btn-primary:hover:not(:disabled) {
  background: #0369a1;
}

.btn-secondary {
  background: var(--bg-input, #1e293b);
  border-color: var(--border-color, #334155);
  color: var(--text-main, #cbd5e1);
}
.btn-secondary:hover:not(:disabled) {
  background: var(--border-color, #334155);
  color: var(--text-main, #fff);
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.workspace-area {
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-main, #090d16);
}

/* Form Mode Layout */
.form-mode-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-main, #090d16);
}

.form-subnav {
  display: flex;
  gap: 4px;
  padding: 8px 16px;
  background: var(--bg-card, #0b1120);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.subnav-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--text-muted, #94a3b8);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.subnav-item:hover {
  color: var(--text-main, #f1f5f9);
  background: var(--nav-hover, rgba(30, 41, 59, 0.4));
}

.subnav-item.active {
  background: var(--bg-input, #1e293b);
  color: #0284c7;
  border-color: var(--border-color, #334155);
}

.count-pill {
  background: var(--bg-input, #090d16);
  color: var(--text-muted, #94a3b8);
  padding: 1px 6px;
  border-radius: 10px;
  font-size: 11px;
}

.form-section-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  scrollbar-gutter: stable;
}

.section-title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-title-bar h3 {
  margin: 0;
  font-size: 16px;
  color: var(--text-main, #f1f5f9);
}

.section-title-bar p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.item-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: background-color 0.2s, border-color 0.2s;
}

.card-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.proto-badge {
  font-size: 10px;
  font-weight: 600;
  background: #0284c722;
  color: #0284c7;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}

.group-badge {
  font-size: 10px;
  font-weight: 600;
  background: #7e22ce22;
  color: #9333ea;
  padding: 2px 6px;
  border-radius: 4px;
}

.sub-badge {
  font-size: 10px;
  font-weight: 600;
  background: #16a34a22;
  color: #16a34a;
  padding: 2px 6px;
  border-radius: 4px;
}

.card-key {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main, #e2e8f0);
  font-family: monospace;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-actions {
  display: flex;
  gap: 4px;
}

.card-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.card-info .row {
  display: flex;
  gap: 6px;
}

.card-info .lbl {
  color: var(--text-muted, #64748b);
  width: 36px;
  flex-shrink: 0;
}

.card-info .val {
  color: var(--text-main, #cbd5e1);
}

.code-tag {
  background: var(--bg-input, #1e293b);
  color: #0284c7;
  padding: 1px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 11px;
}

.chips-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}

.filter-chip {
  background: var(--bg-input, #1e293b);
  color: var(--text-main, #cbd5e1);
  font-size: 11px;
  font-family: monospace;
  padding: 2px 6px;
  border-radius: 4px;
}

/* Rule table */
.rule-table-wrap {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  overflow: hidden;
}

.rule-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.rule-table th {
  text-align: left;
  padding: 10px 14px;
  background: var(--bg-input, #0b1120);
  color: var(--text-muted, #64748b);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.rule-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-color, #1e293b55);
  color: var(--text-main, #cbd5e1);
}

.fallback-tag {
  color: #f59e0b;
  font-weight: 600;
}

.target-tag {
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  display: inline-block;
  background: var(--bg-input, #1e293b);
  color: #0284c7;
}
.target-tag.direct {
  background: #064e3b22;
  color: #10b981;
}
.target-tag.block {
  background: #7f1d1d22;
  color: #ef4444;
}

/* Form grid */
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  background: var(--bg-card, #0f172a);
  padding: 20px;
  border-radius: 8px;
  border: 1px solid var(--border-color, #1e293b);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
}

.form-group input, .form-group select {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #f1f5f9);
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
}

.form-group input:focus, .form-group select:focus {
  border-color: #0284c7;
}

/* Editor Wrapper */
.editor-wrapper {
  flex: 1;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  position: relative;
  overflow: hidden;
  background: var(--bg-card, #0f172a);
}

.editor-host {
  flex: 1;
  min-height: 0;
  height: 100%;
  position: relative;
  overflow: hidden;
}

.editor-host :deep(.cm-editor) {
  height: 100%;
  width: 100%;
  outline: none;
}

.editor-host :deep(.cm-scroller) {
  height: 100%;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Status Bar */
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px;
  background: var(--bg-card, #090d16);
  border-top: 1px solid var(--border-color, #1e293b);
  font-size: 12px;
  color: var(--text-muted, #64748b);
}

.status-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.val-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  border-radius: 4px;
}

.val-indicator.valid {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.val-indicator.invalid {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.val-indicator.neutral {
  color: var(--text-muted, #64748b);
}

.val-msg {
  max-width: 600px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.btn-text {
  background: none;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0;
}
.btn-text:hover {
  color: #0284c7;
}

/* Modals */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-card {
  width: 580px;
  max-height: 85vh;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3);
  color: var(--text-main, #f8fafc);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.modal-header h3 {
  margin: 0;
  font-size: 15px;
  color: var(--text-main, #e2e8f0);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted, #94a3b8);
  font-size: 20px;
  cursor: pointer;
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
}

.form-modal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-modal .field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-modal .field label {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.form-modal input, .form-modal textarea, .form-modal select {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--text-main, #f1f5f9);
  font-size: 13px;
  outline: none;
}

.form-modal input:focus, .form-modal textarea:focus, .form-modal select:focus {
  border-color: #0284c7;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 10px;
}

.create-backup-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.create-backup-bar input {
  flex: 1;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 6px;
  padding: 6px 12px;
  color: var(--text-main, #f1f5f9);
  font-size: 13px;
  outline: none;
}

.backup-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.backup-table th {
  text-align: left;
  padding: 8px;
  color: var(--text-muted, #64748b);
  background: var(--bg-input, #0b1120);
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.backup-table td {
  padding: 10px 8px;
  border-bottom: 1px solid var(--border-color, #1e293b77);
  color: var(--text-main, #cbd5e1);
}

.name-cell {
  font-family: monospace;
  color: #0284c7;
}

.btn-icon {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-muted, #94a3b8);
  border-radius: 4px;
  padding: 4px 6px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}
.btn-icon:hover {
  color: var(--text-main, #fff);
  background: var(--border-color, #334155);
}
.btn-icon.restore:hover {
  color: #0284c7;
  border-color: #0284c7;
}
.btn-icon.delete:hover {
  color: #ef4444;
  border-color: #ef4444;
}

.empty-state {
  text-align: center;
  padding: 30px;
  color: var(--text-muted, #64748b);
  font-size: 13px;
}

.empty-state-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  background: var(--bg-input, rgba(15, 23, 42, 0.4));
  border: 1px dashed var(--border-color, #334155);
  border-radius: 8px;
  text-align: center;
  margin: 12px 0;
}

.empty-icon {
  color: var(--text-muted, #64748b);
  margin-bottom: 12px;
}

.empty-text {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-main, #cbd5e1);
  margin-bottom: 6px;
}

.empty-subtext {
  font-size: 12px;
  color: var(--text-muted, #64748b);
  margin-bottom: 16px;
  max-width: 360px;
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: monospace;
}

.hidden {
  display: none;
}

.modal-large {
  max-width: 820px;
  width: 90%;
}

.btn-sub-fetch {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  font-size: 11px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: #0284c7;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-sub-fetch:hover {
  background: var(--border-color, #334155);
  color: #0369a1;
  border-color: #0284c7;
}

.sub-loading-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 40px;
  color: var(--text-muted, #94a3b8);
  font-size: 13px;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.sub-nodes-container {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sub-nodes-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: var(--bg-input, #1e293b);
  border-radius: 6px;
  font-size: 12px;
  color: var(--text-main, #cbd5e1);
}

.sub-nodes-toolbar strong {
  color: #0284c7;
}

.sub-nodes-table-wrap {
  max-height: 400px;
  overflow-y: auto;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  background: var(--bg-card, #090d16);
}

.sub-nodes-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.sub-nodes-table th {
  position: sticky;
  top: 0;
  background: var(--bg-input, #0f172a);
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-color, #1e293b);
  z-index: 1;
}

.sub-nodes-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  color: var(--text-main, #cbd5e1);
}

.sub-nodes-table tr {
  cursor: pointer;
  transition: background 0.15s;
}

.sub-nodes-table tr:hover {
  background: var(--nav-hover, rgba(30, 41, 59, 0.5));
}

.sub-nodes-table tr.row-selected {
  background: rgba(2, 132, 199, 0.08);
}

.checkbox-col {
  text-align: center;
}

.checkbox-col input {
  cursor: pointer;
}

.proto-badge-sm {
  display: inline-block;
  padding: 2px 6px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  color: #0284c7;
}

.country-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

/* =========================================================
   Group Editor Modal: Linkage with Subs, Nodes & Fixed(N)
   ========================================================= */

.group-editor-modal {
  width: 820px;
  max-width: 95vw;
}

.modal-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.modal-title-icon {
  color: #0284c7;
}

.modal-subtitle {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.group-modal-body {
  overflow-y: auto;
  max-height: calc(85vh - 120px);
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-step-card {
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px 16px;
}

.step-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.step-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #0284c7;
  color: #ffffff;
  font-size: 11px;
  font-weight: 700;
}

.step-card-header h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.quick-names-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}

.quick-names-lbl {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.quick-name-btn {
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--bg-card, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}

.quick-name-btn:hover {
  color: var(--text-main, #f8fafc);
  border-color: #0284c7;
}

.quick-name-btn.active {
  background: rgba(2, 132, 199, 0.2);
  color: #38bdf8;
  border-color: #0284c7;
  font-weight: 600;
}

.source-type-switcher {
  display: flex;
  gap: 6px;
  margin-bottom: 12px;
  background: var(--bg-card, #0f172a);
  padding: 3px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
}

.source-tab-btn {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: var(--text-muted, #94a3b8);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}

.source-tab-btn.active {
  background: var(--bg-input, #1e293b);
  color: #0284c7;
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.source-content-box {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  padding: 12px;
}

.sub-matched-stats {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
  font-size: 11px;
}

.stat-pill {
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  padding: 2px 8px;
  border-radius: 10px;
}

.filter-code-preview {
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
}

.nodes-select-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.search-input-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  padding: 2px 8px;
  flex: 1;
  min-width: 180px;
}

.search-input-wrap .search-mini {
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  width: 100%;
}

.nodes-select-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-xs {
  padding: 2px 8px;
  font-size: 11px;
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
}

.selected-count-badge {
  font-size: 11px;
  color: #0284c7;
  font-weight: 600;
}

.nodes-checkbox-list {
  max-height: 180px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 4px;
  padding: 6px;
  background: var(--bg-input, #090d16);
}

.node-check-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: background 0.15s;
}

.node-check-item:hover {
  background: rgba(30, 41, 59, 0.4);
}

.node-check-item.selected {
  background: rgba(2, 132, 199, 0.15);
}

.node-proto-badge {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(99, 102, 241, 0.15);
  color: #818cf8;
  font-weight: 600;
}

.node-addr {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  margin-left: auto;
}

.policy-modes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px;
}

.policy-mode-card {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}

.policy-mode-card:hover {
  border-color: #334155;
}

.policy-mode-card.active {
  border-color: #0284c7;
  background: rgba(2, 132, 199, 0.12);
}

.policy-mode-card.highlight-fixed {
  border-color: rgba(245, 158, 11, 0.4);
}

.policy-mode-card.highlight-fixed.active {
  border-color: #f59e0b;
  background: rgba(245, 158, 11, 0.12);
}

.mode-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.mode-title {
  font-size: 12px;
  color: var(--text-main, #f8fafc);
}

.mode-desc {
  font-size: 10px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.3;
}

.fixed-node-picker-box {
  margin-top: 12px;
  background: var(--bg-card, #0f172a);
  border: 1px solid rgba(245, 158, 11, 0.4);
  border-radius: 6px;
  padding: 12px;
}

.picker-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  margin-bottom: 8px;
  color: var(--text-main, #f8fafc);
}

.candidate-nodes-list {
  max-height: 200px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.candidate-node-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 10px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 5px;
  cursor: pointer;
  transition: all 0.15s;
}

.candidate-node-row:hover {
  border-color: #0284c7;
}

.candidate-node-row.active {
  border-color: #10b981;
  background: rgba(16, 185, 129, 0.12);
}

.row-left {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.index-badge {
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
  color: #38bdf8;
  background: rgba(2, 132, 199, 0.2);
  padding: 1px 6px;
  border-radius: 4px;
}

.proto-tag {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(148, 163, 184, 0.15);
  color: var(--text-muted, #94a3b8);
}

.row-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.latency-pill {
  font-size: 11px;
  color: #10b981;
  font-family: monospace;
}

.bound-tag {
  font-size: 11px;
  font-weight: 600;
  color: #10b981;
  background: rgba(16, 185, 129, 0.2);
  padding: 2px 8px;
  border-radius: 10px;
}

.fixed-code-banner {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  border-radius: 5px;
  color: #10b981;
  font-size: 12px;
}

.preview-card {
  background: var(--bg-card, #0f172a);
}

.dae-preview-code {
  margin: 0;
  padding: 10px 12px;
  background: var(--bg-main, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 5px;
  color: #38bdf8;
  font-size: 12px;
  line-height: 1.4;
  overflow-x: auto;
}

/* =========================================================
   Routing Rules Upgrade: Conflict Diagnostic Panel & Builder
   ========================================================= */

.modal-card.modal-lg {
  width: 820px;
  max-width: 95vw;
}

.modal-title-with-tabs {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.modal-title-with-tabs h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.modal-subtabs {
  display: flex;
  gap: 4px;
  background: var(--bg-input, #090d16);
  padding: 2px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
}

.subtab-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: transparent;
  border: none;
  border-radius: 4px;
  color: var(--text-muted, #94a3b8);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.subtab-btn:hover {
  color: var(--text-main, #f8fafc);
}

.subtab-btn.active {
  background: var(--bg-card, #1e293b);
  color: #0284c7;
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.title-with-badge {
  display: flex;
  align-items: center;
  gap: 10px;
}

.health-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
}

.health-badge.success {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.25);
}

.health-badge.warning {
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.25);
}

.health-badge.danger {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.25);
}

/* Conflict & Health Diagnostic Panel */
.conflict-panel-wrap {
  margin-bottom: 16px;
}

.health-summary-banner {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 8px;
  background: rgba(16, 185, 129, 0.08);
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.health-summary-banner.success .banner-icon {
  color: #10b981;
  flex-shrink: 0;
  margin-top: 2px;
}

.banner-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.banner-title {
  font-size: 13px;
  font-weight: 600;
  color: #10b981;
}

.banner-sub {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.4;
}

.conflict-card {
  border-radius: 8px;
  background: var(--bg-card, #0f172a);
  border: 1px solid rgba(245, 158, 11, 0.3);
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.conflict-card.has-error {
  border-color: rgba(239, 68, 68, 0.35);
}

.conflict-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--bg-input, #090d16);
  cursor: pointer;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.conflict-card-header .header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.conflict-card-header .status-icon.warning {
  color: #f59e0b;
}

.conflict-card-header .status-icon.danger {
  color: #ef4444;
}

.conflict-card-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  display: flex;
  align-items: center;
  gap: 8px;
}

.conflict-count-pill {
  font-size: 11px;
  font-weight: 500;
  padding: 1px 7px;
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
  border-radius: 10px;
}

.conflict-card.has-error .conflict-count-pill {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.conflict-card-subtitle {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
}

.toggle-collapse-btn {
  color: var(--text-muted, #94a3b8);
}

.conflict-list {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 320px;
  overflow-y: auto;
}

.conflict-item {
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--bg-input, #090d16);
  border-left: 3px solid #0284c7;
}

.conflict-item.error {
  border-left-color: #ef4444;
  background: rgba(239, 68, 68, 0.04);
}

.conflict-item.warning {
  border-left-color: #f59e0b;
  background: rgba(245, 158, 11, 0.04);
}

.conflict-item.info {
  border-left-color: #0284c7;
  background: rgba(2, 132, 199, 0.04);
}

.conflict-item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.sev-tag {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.sev-tag.error {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.sev-tag.warning {
  background: rgba(245, 158, 11, 0.15);
  color: #f59e0b;
}

.sev-tag.info {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.conflict-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.involved-rules {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}

.lbl-involved {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.rule-link-chip {
  padding: 1px 5px;
  background: var(--bg-card, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  color: #0284c7;
  cursor: pointer;
  transition: all 0.15s;
}

.rule-link-chip:hover {
  background: #0284c7;
  color: #fff;
  border-color: #0284c7;
}

.conflict-desc {
  font-size: 12px;
  color: var(--text-main, #e2e8f0);
  line-height: 1.45;
  margin-bottom: 6px;
}

.conflict-suggestion {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 10px;
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

.sugg-content {
  flex: 1;
}

.sugg-label {
  font-weight: 600;
  color: #f59e0b;
  margin-right: 4px;
}

.quick-fix-actions {
  margin-top: 4px;
  width: 100%;
  display: flex;
  justify-content: flex-end;
}

.btn-quick-fix {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
  border: 1px solid rgba(2, 132, 199, 0.3);
  gap: 4px;
  padding: 3px 8px;
  font-size: 11px;
  font-weight: 500;
}

.btn-quick-fix:hover {
  background: #0284c7;
  color: #fff;
}

/* Table row highlight & status badges */
.rule-table tr.row-highlighted {
  background: rgba(2, 132, 199, 0.18) !important;
  box-shadow: inset 3px 0 0 #0284c7;
  animation: pulseHighlight 1.5s ease-in-out infinite alternate;
}

@keyframes pulseHighlight {
  from {
    background: rgba(2, 132, 199, 0.12);
  }
  to {
    background: rgba(2, 132, 199, 0.28);
  }
}

.rule-table tr.row-error {
  border-left: 2px solid #ef4444;
}

.rule-table tr.row-warning {
  border-left: 2px solid #f59e0b;
}

.rule-condition-text {
  color: var(--text-main, #f8fafc);
  font-size: 12px;
}

.rule-health-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.rule-health-tag.ok {
  color: #10b981;
  background: rgba(16, 185, 129, 0.08);
}

.rule-health-tag.warning {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.12);
}

.rule-health-tag.danger {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.12);
}

.row-action-cell {
  white-space: nowrap;
}

/* Modal Input Mode Pills */
.mode-pill-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.mode-pill-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.pill-group {
  display: flex;
  gap: 6px;
  background: var(--bg-input, #090d16);
  padding: 3px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
}

.pill-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  background: transparent;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.pill-btn:hover {
  color: var(--text-main, #f8fafc);
}

.pill-btn.active {
  background: var(--bg-card, #1e293b);
  color: #0284c7;
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

/* Presets Section */
.preset-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 14px;
}

.preset-select {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  font-size: 13px;
}

.preset-card-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  font-size: 12px;
}

.preset-desc-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--text-main, #e2e8f0);
}

.preset-tip-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: #f59e0b;
  font-weight: 500;
}

/* Builder Section */
.builder-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 14px;
}

.builder-items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.builder-row {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--bg-input, #090d16);
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--border-color, #1e293b);
  flex-wrap: wrap;
}

.builder-not-check {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  color: #ef4444;
  cursor: pointer;
}

.builder-type-select {
  padding: 6px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  color: var(--text-main, #f8fafc);
  font-size: 12px;
  font-weight: 500;
}

.builder-mod-select {
  padding: 6px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  color: #0284c7;
  font-size: 12px;
  font-weight: 500;
}

.builder-val-box {
  flex: 1;
  min-width: 200px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.builder-val-input {
  width: 100%;
  padding: 6px 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #334155);
  border-radius: 4px;
  color: var(--text-main, #f8fafc);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.quick-chips {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.quick-chips .chip {
  padding: 1px 5px;
  background: var(--bg-card, #1e293b);
  border: 1px solid var(--border-color, #334155);
  border-radius: 3px;
  font-size: 10px;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  transition: all 0.15s;
}

.quick-chips .chip:hover {
  background: #0284c7;
  color: #fff;
  border-color: #0284c7;
}

.btn-add-cond {
  align-self: flex-start;
  gap: 6px;
  font-size: 11px;
}

/* Custom Raw Input */
.custom-section {
  margin-bottom: 14px;
}

.mono-input {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
}

/* Syntax Live Preview */
.syntax-preview-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  background: var(--bg-input, #090d16);
  border-left: 3px solid #0284c7;
  border-radius: 4px;
  margin-bottom: 14px;
}

.preview-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  font-weight: 500;
}

.preview-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  font-weight: 600;
  color: #0284c7;
  word-break: break-all;
}

/* Guide Tab */
.guide-modal {
  max-height: 65vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 20px;
}

.guide-banner {
  padding: 10px 14px;
  border-radius: 6px;
  background: rgba(2, 132, 199, 0.08);
  border: 1px solid rgba(2, 132, 199, 0.2);
}

.guide-banner .banner-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #0284c7;
  margin-bottom: 4px;
}

.guide-banner p {
  margin: 0;
  font-size: 12px;
  color: var(--text-main, #cbd5e1);
  line-height: 1.5;
}

.syntax-docs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 12px;
}

.syntax-card {
  padding: 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.syntax-card-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.syntax-card-head h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.syntax-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg-card, #1e293b);
  color: #0284c7;
}

.syntax-summary {
  margin: 0;
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.4;
}

.syntax-examples-wrap {
  margin-top: 4px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ex-lbl {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
  text-transform: uppercase;
}

.syntax-example-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 6px;
  background: var(--bg-card, #0f172a);
  border-radius: 4px;
  font-size: 11px;
}

.ex-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #10b981;
  font-weight: 500;
}

.ex-explain {
  flex: 1;
  color: var(--text-muted, #94a3b8);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.btn-text-xs {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 6px;
  background: rgba(2, 132, 199, 0.12);
  border: 1px solid rgba(2, 132, 199, 0.25);
  border-radius: 3px;
  color: #0284c7;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}

.btn-text-xs:hover {
  background: #0284c7;
  color: #fff;
}

/* Routing Modes Styles */
.badge-active-mode {
  background: rgba(16, 185, 129, 0.2) !important;
  color: #34d399 !important;
  font-size: 11px !important;
  font-weight: 600 !important;
}

.modes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(460px, 1fr));
  gap: 16px;
}

.mode-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: all 0.2s ease;
  position: relative;
}

.mode-card:hover {
  border-color: #334155;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.mode-card.mode-active {
  border-color: #0284c7;
  background: linear-gradient(180deg, rgba(2, 132, 199, 0.05) 0%, rgba(15, 23, 42, 0.8) 100%);
  box-shadow: 0 0 0 1px rgba(2, 132, 199, 0.3), 0 4px 16px rgba(2, 132, 199, 0.1);
}

.mode-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.mode-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mode-icon-avatar {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #1e293b;
  color: #94a3b8;
  flex-shrink: 0;
}

.mode-icon-avatar.gfwlist {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
}

.mode-icon-avatar.bypass_cn {
  background: rgba(2, 132, 199, 0.15);
  color: #38bdf8;
}

.mode-icon-avatar.only_cn {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
}

.mode-icon-avatar.global {
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
}

.mode-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mode-name-row h4 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
}

.mode-tag {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted, #94a3b8);
}

.badge-active-glow {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #10b981;
  font-weight: 600;
  margin-top: 4px;
}

.mode-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-apply-mode {
  padding: 5px 12px;
  font-size: 12px;
}

.mode-desc {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--text-muted, #94a3b8);
}

.mode-meta-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 6px;
}

.rules-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.rules-count-pill {
  font-size: 11px;
  color: #cbd5e1;
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.btn-text-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: transparent;
  border: none;
  color: #0284c7;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}

.btn-text-link:hover {
  text-decoration: underline;
}

.btn-fast-test {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

/* Fast Test Results Panel */
.mode-test-panel {
  border-top: 1px solid var(--border-color, #1e293b);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.test-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.test-panel-title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--text-main, #f1f5f9);
  display: flex;
  align-items: center;
  gap: 4px;
}

.test-hint {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.test-items-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: rgba(0, 0, 0, 0.15);
  border-radius: 6px;
  padding: 6px;
}

.test-item-row {
  display: grid;
  grid-template-columns: 100px 80px 1fr 60px;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 11.5px;
  transition: background 0.15s;
}

.test-item-row:hover {
  background: rgba(255, 255, 255, 0.03);
}

.target-col {
  display: flex;
  flex-direction: column;
}

.target-name {
  color: #f1f5f9;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.target-cat {
  font-size: 10px;
  color: #64748b;
}

.badge-out {
  font-size: 10.5px;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: 600;
  white-space: nowrap;
}

.badge-out.out-direct {
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
}

.badge-out.out-proxy {
  background: rgba(2, 132, 199, 0.15);
  color: #0284c7;
}

.badge-out.out-block {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.rule-col code {
  font-size: 10.5px;
  color: #94a3b8;
  background: transparent;
  padding: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  display: block;
}

.latency-col {
  text-align: right;
}

.latency-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 4px;
  border-radius: 3px;
}

.latency-badge.fast {
  color: #10b981;
  background: rgba(16, 185, 129, 0.1);
}

.latency-badge.medium {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.1);
}

.latency-badge.slow {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.1);
}

.detail-col {
  display: none;
}

/* Rules Table in Modal */
.rules-table-container {
  max-height: 55vh;
  overflow-y: auto;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
}

.rules-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.rules-table th {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.3);
  text-align: left;
  font-weight: 600;
  color: #cbd5e1;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.rules-table td {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.rule-idx {
  color: #64748b;
  font-size: 11px;
}

.rule-cond {
  color: #f1f5f9;
}

/* ==================== Profiles List View Styles ==================== */
.profiles-list-view {
  display: flex;
  flex-direction: column;
  padding: 24px 32px;
  height: 100%;
  overflow-y: auto;
  background: var(--bg-main, #090d16);
}

.profiles-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  gap: 20px;
}

.header-icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  background: rgba(14, 165, 233, 0.12);
  border: 1px solid rgba(14, 165, 233, 0.25);
  border-radius: 10px;
  margin-right: 16px;
  flex-shrink: 0;
}

.profiles-header .header-left {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
}

.profiles-title {
  margin: 0 0 4px 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-main, #f8fafc);
}

.profiles-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
}

.profiles-table-container {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.profiles-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.profiles-table th {
  padding: 12px 14px;
  background: rgba(0, 0, 0, 0.2);
  text-align: left;
  font-weight: 600;
  color: #94a3b8;
  border-bottom: 1px solid var(--border-color, #1e293b);
  white-space: nowrap;
}

.profiles-table td {
  padding: 14px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.profiles-table tr:hover {
  background: rgba(255, 255, 255, 0.02);
}

.profiles-table tr.row-active {
  background: rgba(16, 185, 129, 0.04);
}

.profile-name-cell {
  width: 140px;
  max-width: 160px;
  white-space: nowrap;
}

.profile-name-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 155px;
}

.profile-filename {
  font-size: 13px;
  color: var(--text-main, #f8fafc);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
  max-width: 120px;
}

.profile-note-text {
  font-size: 13px;
  color: #94a3b8;
}

.badge-count {
  display: inline-block;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 4px;
  background: var(--bg-input, #1e293b);
  color: #cbd5e1;
  border: 1px solid var(--border-color, #334155);
  white-space: nowrap;
}

/* Prominent Active Badge */
.badge-active-prominent {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.35);
  border-radius: 9999px;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.15);
  white-space: nowrap;
}

.badge-inactive {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  font-size: 12px;
  color: #64748b;
  background: rgba(100, 116, 139, 0.1);
  border: 1px solid rgba(100, 116, 139, 0.2);
  border-radius: 9999px;
  white-space: nowrap;
}

/* Invalid / Failed Validation Status Badge */
.badge-invalid-clickable {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  font-size: 11px;
  font-weight: 600;
  color: #f87171;
  background: rgba(239, 68, 68, 0.14);
  border: 1px solid rgba(239, 68, 68, 0.4);
  border-radius: 9999px;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  transition: all 0.15s ease;
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.15);
}

.badge-invalid-clickable:hover {
  background: rgba(239, 68, 68, 0.28);
  border-color: #ef4444;
  color: #ffffff;
  transform: translateY(-1px);
}

.syntax-error-console {
  background: #020617;
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 6px;
  padding: 12px 14px;
  max-height: 260px;
  overflow-y: auto;
  font-size: 12px;
  line-height: 1.5;
  color: #fca5a5;
}

.syntax-error-console pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.alert-box-danger {
  padding: 10px 14px;
  background: rgba(239, 68, 68, 0.1);
  border-left: 3px solid #ef4444;
  border-radius: 0 6px 6px 0;
  font-size: 13px;
  color: #fecaca;
}

/* Sub-nav Tab Error Indicators */
.subnav-item.subnav-has-error {
  border-color: rgba(239, 68, 68, 0.45) !important;
  color: #f87171 !important;
  background: rgba(239, 68, 68, 0.08) !important;
}

.subnav-error-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  background: rgba(239, 68, 68, 0.22);
  color: #fca5a5;
  border: 1px solid rgba(239, 68, 68, 0.5);
  font-size: 10px;
  font-weight: 600;
  padding: 1px 5px;
  border-radius: 9999px;
  margin-left: 4px;
}

.section-error-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  color: #f87171;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.4);
  padding: 2px 8px;
  border-radius: 9999px;
  margin-left: 10px;
  vertical-align: middle;
}

/* Module Syntax Error Banner */
.module-syntax-error-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.35);
  border-left: 4px solid #ef4444;
  border-radius: 8px;
  padding: 12px 16px;
}

.module-syntax-error-banner .banner-icon-col {
  flex-shrink: 0;
}

.module-syntax-error-banner .banner-body {
  flex: 1;
  min-width: 0;
}

.module-syntax-error-banner .banner-title {
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.module-syntax-error-banner .error-line-badge {
  background: rgba(239, 68, 68, 0.25);
  color: #fca5a5;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.module-syntax-error-banner .banner-msg {
  font-size: 12px;
  color: #fca5a5;
  margin-top: 3px;
  line-height: 1.4;
  word-break: break-all;
}

.module-syntax-error-banner .btn-jump-code {
  flex-shrink: 0;
  border-color: rgba(239, 68, 68, 0.5);
  color: #f87171;
  background: rgba(239, 68, 68, 0.1);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.module-syntax-error-banner .btn-jump-code:hover {
  background: #ef4444;
  color: #ffffff;
}

/* Form Group with Help Tips */
.form-label-with-help {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  margin-bottom: 5px;
  font-weight: 500;
  font-size: 13px;
}

.help-tooltip-trigger {
  color: #64748b;
  cursor: help;
  display: inline-flex;
  align-items: center;
  transition: color 0.15s ease;
}

.help-tooltip-trigger:hover {
  color: var(--primary, #0284c7);
}

.field-help-text {
  display: block;
  font-size: 11px;
  line-height: 1.35;
  color: #64748b;
  margin-top: 4px;
}

/* DNS Request Routing Syntax Guide Box */
.dns-syntax-guide-box {
  background: rgba(15, 23, 42, 0.75);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 14px 16px;
}

.guide-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.syntax-card {
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 12px 14px;
}

.syntax-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.syntax-tag {
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.syntax-name {
  font-size: 12px;
  font-weight: 600;
  color: #f1f5f9;
}

.syntax-desc {
  font-size: 11px;
  color: #94a3b8;
  margin-bottom: 6px;
  line-height: 1.4;
}

.syntax-example-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.syntax-example-list li {
  display: flex;
  flex-direction: column;
  gap: 1px;
  font-size: 11px;
}

.syntax-example-list code {
  color: #7dd3fc;
  background: rgba(14, 165, 233, 0.08);
  padding: 1px 4px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  width: fit-content;
}

.syntax-example-list span {
  color: #94a3b8;
  font-size: 11px;
}

.quick-insert-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.quick-chip-btn {
  background: rgba(14, 165, 233, 0.1);
  border: 1px solid rgba(14, 165, 233, 0.3);
  color: #38bdf8;
  border-radius: 9999px;
  padding: 3px 10px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.quick-chip-btn:hover {
  background: rgba(14, 165, 233, 0.25);
  border-color: #38bdf8;
  color: #ffffff;
}

/* Code Mode Toolbar Error Indicators */
.mode-btn-error {
  border-color: rgba(239, 68, 68, 0.4) !important;
  color: #f87171 !important;
}

.code-error-badge-dot {
  color: #ef4444;
  font-size: 9px;
  margin-left: 3px;
}

/* Code Editor Error Banner */
.code-editor-error-banner {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #380a0a;
  border-bottom: 2px solid #ef4444;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.4);
  font-size: 13px;
  gap: 14px;
  z-index: 30;
  position: relative;
}

.code-editor-error-banner .banner-body-text {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.code-editor-error-banner .banner-error-msg {
  color: #fca5a5;
  font-size: 12px;
  max-width: 800px;
  word-break: break-all;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.code-editor-error-banner .btn-jump {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid #ef4444;
  background: rgba(239, 68, 68, 0.25);
  color: #fecaca;
  padding: 4px 10px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.code-editor-error-banner .btn-jump:hover {
  background: #ef4444;
  color: #ffffff;
}

/* CodeMirror Error Line Highlighting */
:deep(.cm-error-line) {
  background-color: rgba(239, 68, 68, 0.24) !important;
  border-left: 4px solid #ef4444 !important;
}

:deep(.cm-error-line::after) {
  content: ' ⚠ 语法错误定位在此行';
  color: #fca5a5;
  font-size: 11px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  padding-left: 14px;
  font-weight: 600;
  pointer-events: none;
}

.profile-action-cell {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  white-space: nowrap;
}

.btn-edit-profile {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 12px;
  padding: 5px 0;
  width: 90px;
  min-width: 90px;
  max-width: 90px;
  box-sizing: border-box;
  flex-shrink: 0;
}

.btn-activate-profile {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 12px;
  padding: 5px 0;
  width: 90px;
  min-width: 90px;
  max-width: 90px;
  box-sizing: border-box;
  flex-shrink: 0;
}

.checkbox-col {
  padding: 14px 8px !important;
}

.checkbox-col input[type="checkbox"],
.profiles-table th:first-child input[type="checkbox"] {
  cursor: pointer;
  accent-color: #0284c7;
  width: 15px;
  height: 15px;
  margin: 0;
  vertical-align: middle;
}

.profiles-table tr.row-selected {
  background: rgba(14, 165, 233, 0.08) !important;
}

.header-actions .btn.danger {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.header-actions .btn.danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.22);
  border-color: #ef4444;
}

.header-actions .btn.danger:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  border-color: rgba(255, 255, 255, 0.1);
  color: #64748b;
  background: rgba(255, 255, 255, 0.03);
}

/* ==================== Profile Detail Header Styles ==================== */
.profile-detail-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.btn-back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-right: 8px;
}

.profile-current-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  font-size: 13px;
}

.profile-current-name {
  font-weight: 600;
  color: #38bdf8;
}

.profile-note-pill {
  font-size: 12px;
  color: #94a3b8;
  padding-left: 6px;
  border-left: 1px solid #334155;
}

/* ==================== DNS Section Styles ==================== */
.dns-panel-box {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 16px 20px;
}

.dns-panel-title {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  display: flex;
  align-items: center;
}

.form-grid-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.optimistic-cache-row {
  padding-top: 10px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.dns-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.dns-table th {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.2);
  text-align: left;
  font-weight: 600;
  color: #94a3b8;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.dns-table td {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.panel-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* Modal Z-Index Overhaul & Styling */
.modal-backdrop {
  z-index: 9999 !important;
  background: rgba(0, 0, 0, 0.7) !important;
  backdrop-filter: blur(4px);
}

.modal-scroll-body {
  max-height: 70vh;
  overflow-y: auto;
  padding: 18px 22px;
}

.flex-align-center {
  display: flex;
  align-items: center;
}

.tag-pill {
  display: inline-block;
  padding: 2px 8px;
  background: rgba(14, 165, 233, 0.15);
  border: 1px solid rgba(14, 165, 233, 0.3);
  border-radius: 4px;
  color: #38bdf8;
  font-size: 11px;
}

.tag-pill-subtle {
  display: inline-block;
  padding: 2px 8px;
  background: rgba(148, 163, 184, 0.12);
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 4px;
  color: #cbd5e1;
  font-size: 11px;
}

.form-grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.flex-gap {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* =========================================================
   Comprehensive Polish: Utilities, Form Controls & Tables
   ========================================================= */

/* Typography & Colors */
.text-primary { color: #0284c7 !important; }
.text-info { color: #38bdf8 !important; }
.text-warning { color: #f59e0b !important; }
.text-muted { color: var(--text-muted, #94a3b8) !important; }
.text-xs { font-size: 11px !important; }
.text-sm { font-size: 13px !important; }
.font-bold { font-weight: 700 !important; }
.font-semibold { font-weight: 600 !important; }
.bold { font-weight: 600 !important; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace !important; }

/* Spacing & Layout Utilities */
.inline-block { display: inline-block !important; }
.block { display: block !important; }
.flex-1 { flex: 1 !important; }
.w-full { width: 100% !important; }
.w-1\/3 { width: 33.333% !important; }
.flex-gap { display: flex; align-items: center; gap: 8px; }
.flex-align-center { display: flex; align-items: center; }

.mb-1 { margin-bottom: 4px !important; }
.mb-2 { margin-bottom: 8px !important; }
.mb-3 { margin-bottom: 12px !important; }
.mb-4 { margin-bottom: 16px !important; }
.mr-1 { margin-right: 4px !important; }
.mr-2 { margin-right: 8px !important; }
.ml-2 { margin-left: 8px !important; }
.mt-1 { margin-top: 4px !important; }
.mt-2 { margin-top: 8px !important; }
.mt-3 { margin-top: 12px !important; }
.mt-4 { margin-top: 16px !important; }
.p-3 { padding: 12px !important; }

.bg-subtle { background: rgba(255, 255, 255, 0.03) !important; }
.border { border: 1px solid var(--border-color, #1e293b) !important; }
.rounded { border-radius: 8px !important; }

/* Form Controls (Inputs, Selects, Labels) */
.form-control {
  width: 100%;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  padding: 8px 12px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
  box-sizing: border-box;
}

.form-control:focus {
  border-color: #0284c7;
  box-shadow: 0 0 0 2px rgba(2, 132, 199, 0.2);
}

.form-control-sm {
  padding: 5px 9px !important;
  font-size: 12px !important;
  border-radius: 5px !important;
}

.form-control-xs {
  padding: 3px 7px !important;
  font-size: 11px !important;
  border-radius: 4px !important;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: #94a3b8;
  margin-bottom: 5px;
}

.form-label.required::after {
  content: ' *';
  color: #ef4444;
  font-weight: bold;
}

.field-hint, .help-text {
  display: block;
  font-size: 11px;
  color: #64748b;
  margin-top: 5px;
  line-height: 1.4;
}

/* Profiles Header Actions */
.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.header-actions .btn {
  white-space: nowrap !important;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  height: 38px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
}

.header-actions .btn span {
  white-space: nowrap !important;
}

/* Status Badges */
.badge-active-prominent {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  font-size: 12px;
  font-weight: 600;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.4);
  border-radius: 9999px;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.15);
  white-space: nowrap;
}

.badge-inactive {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  font-size: 11px;
  color: #64748b;
  background: rgba(100, 116, 139, 0.1);
  border: 1px solid rgba(100, 116, 139, 0.25);
  border-radius: 9999px;
  white-space: nowrap;
}

/* Sub-nav Tab Active Badge */
.badge-active-mode {
  background: rgba(16, 185, 129, 0.2) !important;
  color: #34d399 !important;
  border: 1px solid rgba(16, 185, 129, 0.4) !important;
  font-size: 11px !important;
  font-weight: 600 !important;
  border-radius: 9999px !important;
  padding: 1px 7px !important;
}

/* DNS Table Inputs Styling */
.dns-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.dns-table th {
  padding: 10px 14px;
  background: rgba(0, 0, 0, 0.25);
  text-align: left;
  font-weight: 600;
  color: #94a3b8;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.dns-table td {
  padding: 8px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  vertical-align: middle;
}

.dns-table td .form-control {
  background: var(--bg-input, #090d16);
  border-color: #334155;
}

.dns-table td .form-control:focus {
  border-color: #0284c7;
}

.dns-panel-box {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 18px 22px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.dns-panel-title {
  margin: 0 0 14px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main, #f8fafc);
  display: flex;
  align-items: center;
}

/* Modals & Backdrop */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75) !important;
  backdrop-filter: blur(5px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999 !important;
}

.modal-notice-text {
  font-size: 12px;
  color: #94a3b8;
  line-height: 1.5;
  margin: 0 0 16px 0;
  padding: 8px 12px;
  background: rgba(2, 132, 199, 0.08);
  border-left: 3px solid #0284c7;
  border-radius: 0 6px 6px 0;
}

.mode-config-section {
  background: rgba(0, 0, 0, 0.25) !important;
  border: 1px solid var(--border-color, #1e293b) !important;
  border-radius: 8px;
}

.mode-dns-preview-card {
  background: rgba(0, 0, 0, 0.25) !important;
  border: 1px solid var(--border-color, #1e293b) !important;
  border-radius: 8px;
}

/* Sub-nodes modal table */
.sub-summary {
  font-size: 12px;
  color: #94a3b8;
}

.sub-nodes-modal {
  padding: 16px 20px;
}

/* Target select in route modal */
.target-select {
  width: 100%;
  background: var(--bg-input, #090d16);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 6px;
  color: var(--text-main, #f8fafc);
  padding: 8px 12px;
  font-size: 13px;
  outline: none;
}



/* Additional Controls, Tabs & Table Cells */
.btn-sm {
  padding: 5px 12px !important;
  font-size: 12px !important;
  font-weight: 500 !important;
  border-radius: 6px !important;
}

.btn-xs {
  padding: 3px 8px !important;
  font-size: 11px !important;
  font-weight: 500 !important;
  border-radius: 4px !important;
}

.modal-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.idx-cell {
  width: 50px;
  text-align: center;
  color: #64748b;
}

.idx-num {
  display: inline-block;
  font-size: 11px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.cond-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.rule-target {
  padding: 8px 12px;
}

.profile-name-cell {
  font-weight: 600;
  color: #f8fafc;
}

.profile-note-cell {
  color: #94a3b8;
  font-size: 12px;
}

.action-cell {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.address-cell {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  color: #94a3b8;
}

.backup-list-box {
  max-height: 400px;
  overflow-y: auto;
  margin-top: 12px;
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
}

.route-input-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  background: rgba(0, 0, 0, 0.25);
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--border-color, #1e293b);
}

.tab-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  color: #94a3b8;
  background: transparent;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-pill.active {
  background: var(--bg-card, #1e293b);
  color: #0284c7;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.preset-tip-box {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(14, 165, 233, 0.1);
  border: 1px solid rgba(14, 165, 233, 0.25);
  border-radius: 6px;
  font-size: 12px;
  color: #38bdf8;
}

.preview-header {
  display: flex;
  align-items: center;
}

.preview-content {
  padding: 8px 0;
}

.dns-preview-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.detail-text {
  font-size: 11px;
  color: #64748b;
}

</style>
