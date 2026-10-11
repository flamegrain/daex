<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Wand2,
  Network,
  Server,
  Compass,
  ShieldCheck,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  ArrowLeft,
  Check,
  RefreshCw,
  Zap,
  Eye,
  Plus,
  Trash2,
  HelpCircle,
  Activity,
  Globe,
  Sliders,
  X,
  Radio,
  Sparkles,
  Link2,
  Layers,
  ChevronDown,
  ChevronUp,
  Shield,
  Flag,
  FileCode,
  XCircle,
  FileText,
  RotateCcw
} from 'lucide-vue-next'
import { t, lang } from '../composables/useSettings'

const emit = defineEmits<{
  (e: 'navigate-tab', tab: string): void
}>()

// Stepper
const currentStep = ref<1 | 2 | 3 | 4>(1)

// --- Step 1: Network & Global ---
interface IfaceInfo {
  name: string
  ips: string[]
  isUp: boolean
  isLoop: boolean
  isDefault: boolean
}
const interfaces = ref<IfaceInfo[]>([])
const loadingIfaces = ref(false)

const networkScenario = ref<'gateway' | 'single'>('gateway')
const lanInterface = ref('eth0')
const wanInterface = ref('auto')
const dialMode = ref<'domain' | 'ip'>('domain')
const autoKernel = ref(true)
const logLevel = ref('info')

async function fetchInterfaces() {
  loadingIfaces.value = true
  try {
    const res = await fetch('/api/system/interfaces')
    if (res.ok) {
      interfaces.value = await res.json()
      const def = interfaces.value.find(i => i.isDefault && !i.isLoop && i.isUp)
      if (def && !lanInterface.value) {
        lanInterface.value = def.name
      }
    }
  } catch (err) {
    console.error('Fetch interfaces error:', err)
  } finally {
    loadingIfaces.value = false
  }
}

function selectScenario(scenario: 'gateway' | 'single') {
  networkScenario.value = scenario
  if (scenario === 'gateway') {
    const def = interfaces.value.find(i => i.isDefault)
    lanInterface.value = def ? def.name : 'eth0'
    wanInterface.value = 'auto'
  } else {
    lanInterface.value = 'docker0'
    wanInterface.value = 'auto'
  }
}

// --- Step 2: Nodes & Subscriptions ---
interface NodeItem {
  key: string
  protocol: string
  name: string
  address: string
  port: string
  url: string
}
const existingNodes = ref<NodeItem[]>([])
const systemExistingNodes = ref<NodeItem[]>([])
const loadingNodes = ref(false)
const subUrl = ref('')
const fetchingSub = ref(false)
const subMessage = ref('')
const singleNodeLink = ref('')

async function fetchCurrentGlobals() {
  loadingNodes.value = true
  try {
    const res = await fetch('/api/config/model')
    if (res.ok) {
      const data = await res.json()
      // Store current system nodes for optional reuse, but keep existingNodes initially empty for fresh setup
      systemExistingNodes.value = data.nodes || []
      if (data.globals) {
        if (data.globals.lan_interface) lanInterface.value = data.globals.lan_interface
        if (data.globals.wan_interface) wanInterface.value = data.globals.wan_interface
        if (data.globals.dial_mode) dialMode.value = data.globals.dial_mode
        if (data.globals.log_level) logLevel.value = data.globals.log_level
      }
    }
  } catch (err) {
    console.error('Fetch config globals error:', err)
  } finally {
    loadingNodes.value = false
  }
}

function reuseSystemNodes() {
  if (systemExistingNodes.value.length === 0) return
  existingNodes.value = [...systemExistingNodes.value]
  subMessage.value = t('quick.msg_reused_nodes', { count: systemExistingNodes.value.length })
}

async function handleFetchSub() {
  if (!subUrl.value.trim()) {
    alert(t('quick.msg_invalid_sub_url'))
    return
  }
  fetchingSub.value = true
  subMessage.value = ''
  try {
    const res = await fetch('/api/subscription/nodes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tag: 'quick_sub', url: subUrl.value.trim() })
    })
    const data = await res.json()
    if (res.ok && data.nodes && data.nodes.length > 0) {
      let added = 0
      for (const n of data.nodes) {
        if (!existingNodes.value.some(ex => ex.key === n.key || ex.name === n.name)) {
          existingNodes.value.push({
            key: n.key || `node_${existingNodes.value.length + 1}`,
            protocol: n.protocol || 'VLESS',
            name: n.name || n.key,
            address: n.address || '',
            port: String(n.port || '443'),
            url: n.url || ''
          })
          added++
        }
      }
      subMessage.value = t('quick.msg_sub_fetched', { count: data.nodes.length, added })
      subUrl.value = ''
    } else {
      alert(data.error || t('quick.msg_no_sub_nodes'))
    }
  } catch (err: any) {
    alert(t('quick.msg_sub_failed') + err.message)
  } finally {
    fetchingSub.value = false
  }
}

function handleAddSingleNode() {
  const text = singleNodeLink.value.trim()
  if (!text) return
  const lines = text.split('\n').map(l => l.trim()).filter(Boolean)
  let added = 0
  for (const link of lines) {
    try {
      const u = new URL(link)
      const proto = u.protocol.replace(':', '').toUpperCase()
      const name = decodeURIComponent(u.hash ? u.hash.slice(1) : '') || `node_${existingNodes.value.length + 1}`
      const addr = u.hostname
      const port = u.port || '443'
      existingNodes.value.push({
        key: `custom_${Date.now()}_${added}`,
        protocol: proto,
        name,
        address: addr,
        port,
        url: link
      })
      added++
    } catch (e) {
      console.warn('Invalid link format:', link)
    }
  }
  if (added > 0) {
    singleNodeLink.value = ''
    alert(t('quick.msg_nodes_added', { count: added }))
  } else {
    alert(t('quick.msg_invalid_node_link'))
  }
}

function removeNode(idx: number) {
  existingNodes.value.splice(idx, 1)
}

// --- Step 3: Routing Modes ---
interface RoutingModeRule {
  condition: string
  target: string
}
interface FastTestItem {
  id: string
  name: string
  url: string
  category: string
  expectedOut: string
  matchedRule: string
  status: string
  latency: number
  httpCode: number
  message: string
}
interface RoutingModeItem {
  id: string
  name: string
  description: string
  badge: string
  isCustom: boolean
  dnsSettings: any
  rules: RoutingModeRule[]
  testResults: FastTestItem[]
}

const modesList = ref<RoutingModeItem[]>([])
const selectedModeId = ref('bypass_cn')
const loadingModes = ref(false)
const viewingMode = ref<RoutingModeItem | null>(null)
const showModeRulesModal = ref(false)

// Custom mode creation
const showNewModeModal = ref(false)
const createModeStep = ref<1 | 2>(1)
const newModeForm = ref({
  id: '',
  name: '',
  description: '',
  badge: '自定义',
  dnsSettings: {
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
  rulesText: 'pname(NetworkManager, systemd-resolved, dnsmasq) -> must_direct\ndip(224.0.0.0/3, "ff00::/8") -> direct\ndip(geoip:private) -> direct\ndomain(geosite:category-ads-all) -> block\ndomain(geosite:cn) -> direct\ndip(geoip:cn) -> direct\nfallback: proxy'
})
const savingNewMode = ref(false)

async function fetchModes() {
  loadingModes.value = true
  try {
    const res = await fetch('/api/config/modes')
    if (res.ok) {
      const data = await res.json()
      modesList.value = data.modes || []
      if (data.activeModeId) {
        selectedModeId.value = data.activeModeId
      }
    }
  } catch (err) {
    console.error('Fetch modes error:', err)
  } finally {
    loadingModes.value = false
  }
}

function openViewRules(mode: RoutingModeItem) {
  viewingMode.value = mode
  showModeRulesModal.value = true
}

function openCreateMode() {
  createModeStep.value = 1
  newModeForm.value = {
    id: '',
    name: '',
    description: '',
    badge: '自定义',
    dnsSettings: {
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
    rulesText: 'pname(NetworkManager, systemd-resolved, dnsmasq) -> must_direct\ndip(224.0.0.0/3, "ff00::/8") -> direct\ndip(geoip:private) -> direct\ndomain(geosite:category-ads-all) -> block\ndomain(geosite:cn) -> direct\ndip(geoip:cn) -> direct\nfallback: proxy'
  }
  showNewModeModal.value = true
}

async function saveCustomMode() {
  if (!newModeForm.value.name.trim()) {
    alert(t('quick.msg_mode_name_required'))
    return
  }
  const rawLines = newModeForm.value.rulesText.split('\n')
  const rules: RoutingModeRule[] = []
  for (const l of rawLines) {
    const trimmed = l.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    if (trimmed.includes('->')) {
      const [c, t] = trimmed.split('->')
      rules.push({ condition: c.trim(), target: t.trim() })
    } else if (trimmed.startsWith('fallback:') || trimmed.startsWith('fallback :')) {
      const parts = trimmed.split(':')
      rules.push({ condition: 'fallback', target: parts[1].trim() })
    }
  }
  if (rules.length === 0) {
    alert(t('quick.msg_rule_required'))
    return
  }

  savingNewMode.value = true
  try {
    const res = await fetch('/api/config/modes/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id: newModeForm.value.id || undefined,
        name: newModeForm.value.name.trim(),
        description: newModeForm.value.description.trim(),
        badge: newModeForm.value.badge.trim() || t('quick.badge_custom'),
        dnsSettings: newModeForm.value.dnsSettings,
        rules
      })
    })
    const data = await res.json()
    if (res.ok && data.ok) {
      showNewModeModal.value = false
      await fetchModes()
      selectedModeId.value = data.id || newModeForm.value.name.trim()
      alert(t('quick.msg_mode_created'))
    } else {
      alert(data.error || t('quick.msg_mode_create_failed'))
    }
  } catch (err: any) {
    alert(err.message || 'Error')
  } finally {
    savingNewMode.value = false
  }
}

async function deleteCustomMode(mode: RoutingModeItem) {
  if (!confirm(t('quick.msg_confirm_delete_mode', { name: mode.name }))) return
  try {
    const res = await fetch(`/api/config/modes/${mode.id}`, { method: 'DELETE' })
    if (res.ok) {
      await fetchModes()
      if (selectedModeId.value === mode.id) {
        selectedModeId.value = 'bypass_cn'
      }
    }
  } catch (err) {
    console.error('Delete mode error:', err)
  }
}

// --- Step 4: Connectivity Test & One-Click Apply ---
const testingMode = ref(false)
const testResults = ref<FastTestItem[]>([
  {
    id: 'aliyun',
    name: '国内直连检测 (Aliyun)',
    url: 'https://www.aliyun.com',
    category: '国内骨干网',
    expectedOut: '直连 direct',
    matchedRule: 'domain(geosite:cn) -> direct',
    status: 'pending',
    latency: 0,
    httpCode: 0,
    message: '等待测试'
  },
  {
    id: 'google',
    name: '海外代理检测 (Google)',
    url: 'https://www.google.com/generate_204',
    category: '全球搜索',
    expectedOut: '代理 proxy',
    matchedRule: 'fallback: proxy',
    status: 'pending',
    latency: 0,
    httpCode: 0,
    message: '等待测试'
  },
  {
    id: 'github',
    name: '开发生态检测 (GitHub)',
    url: 'https://github.com',
    category: '全球开发者',
    expectedOut: '代理 proxy',
    matchedRule: 'fallback: proxy',
    status: 'pending',
    latency: 0,
    httpCode: 0,
    message: '等待测试'
  },
  {
    id: 'youtube',
    name: '流媒体服务检测 (YouTube)',
    url: 'https://www.youtube.com/generate_204',
    category: '高清流媒体',
    expectedOut: '代理 proxy',
    matchedRule: 'fallback: proxy',
    status: 'pending',
    latency: 0,
    httpCode: 0,
    message: '等待测试'
  }
])

async function runStep4Diagnostics() {
  testingMode.value = true
  try {
    const res = await fetch('/api/config/modes/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ modeId: selectedModeId.value })
    })
    if (res.ok) {
      const data = await res.json()
      if (data.testResults && data.testResults.length > 0) {
        testResults.value = data.testResults
      }
    }
  } catch (err) {
    console.error('Diagnostics test error:', err)
  } finally {
    testingMode.value = false
  }
}

const applying = ref(false)
const applySuccess = ref(false)
const saveProfileName = ref('config.quick.dae')

function getLocalizedMode(mode: RoutingModeItem | undefined) {
  if (!mode) return { name: '', badge: '', description: '' }
  if (mode.id === 'bypass_cn') {
    return {
      name: t('modes.mode_bypass_cn_name'),
      badge: t('quick.badge_bypass_cn'),
      description: t('modes.mode_bypass_cn_desc')
    }
  }
  if (mode.id === 'gfwlist') {
    return {
      name: t('modes.mode_gfwlist_name'),
      badge: t('quick.badge_gfwlist'),
      description: t('modes.mode_gfwlist_desc')
    }
  }
  if (mode.id === 'only_cn') {
    return {
      name: t('modes.mode_only_cn_name'),
      badge: t('quick.badge_only_cn'),
      description: t('modes.mode_only_cn_desc')
    }
  }
  if (mode.id === 'global') {
    return {
      name: t('modes.mode_global_name'),
      badge: t('quick.badge_global'),
      description: t('modes.mode_global_desc')
    }
  }
  return {
    name: mode.name,
    badge: mode.badge,
    description: mode.description
  }
}

function getTestItemName(item: FastTestItem) {
  if (lang.value === 'en-US') {
    if (item.id === 'aliyun') return 'Domestic Direct (Aliyun)'
    if (item.id === 'google') return 'Overseas Proxy (Google)'
    if (item.id === 'github') return 'Dev Ecosystem (GitHub)'
    if (item.id === 'youtube') return 'Streaming Media (YouTube)'
    if (item.id === 'baidu') return 'Domestic Search (Baidu)'
    if (item.id === 'bilibili') return 'Domestic Video (Bilibili)'
    if (item.id === 'openai') return 'AI Service (OpenAI)'
    if (item.id === 'lan') return 'LAN Gateway Management'
  }
  return item.name
}

function getTestItemExpected(item: FastTestItem) {
  if (lang.value === 'en-US') {
    const exp = (item.expectedOut || '').toLowerCase()
    if (exp.includes('直连') || exp.includes('direct')) {
      if (exp.includes('局域网') || exp.includes('内网') || exp.includes('lan')) return 'LAN Direct'
      return 'Direct'
    }
    if (exp.includes('代理') || exp.includes('proxy')) {
      return 'Proxy'
    }
  }
  return item.expectedOut
}

function getTestItemMessage(item: FastTestItem) {
  if (lang.value === 'en-US') {
    if (item.status === 'pending') return 'Waiting for test'
    if (item.id === 'aliyun') return 'Domestic backbone fiber direct connection'
    if (item.id === 'google') return 'Overseas target routed via proxy endpoint'
    if (item.id === 'github') return 'Code hosting & releases channel connected'
    if (item.id === 'youtube') return 'Global video CDN nodes reachable'
    if (item.id === 'lan') return 'LAN private network isolated direct connection'
  }
  return item.message
}

function getSpeedQuality(item: FastTestItem) {
  if (item.status === 'pending') {
    return { level: 'pending', text: t('quick.quality_pending'), badgeClass: 'quality-pending' }
  }
  if (item.status === 'error' || item.status === 'timeout' || (item.latency <= 0 && item.status !== 'ok') || item.latency >= 500) {
    return { level: 'fail', text: t('quick.quality_fail'), badgeClass: 'quality-fail' }
  }
  if (item.latency >= 120) {
    return { level: 'slow', text: t('quick.quality_slow', { latency: item.latency }), badgeClass: 'quality-slow' }
  }
  return { level: 'fast', text: t('quick.quality_fast', { latency: item.latency }), badgeClass: 'quality-fast' }
}

async function applyAndActivate() {
  if (existingNodes.value.length === 0) {
    alert(t('quick.msg_no_nodes_error'))
    currentStep.value = 2
    return
  }

  applying.value = true
  applySuccess.value = false
  try {
    // 1. Apply selected routing mode (links DNS & Routing)
    const modeRes = await fetch('/api/config/modes/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ modeId: selectedModeId.value })
    })
    const modeData = await modeRes.json()
    if (!modeRes.ok || !modeData.ok) {
      throw new Error(modeData.error || t('quick.msg_apply_mode_failed'))
    }

    // 2. Fetch fresh model, sync globals & nodes
    const modelRes = await fetch('/api/config/model')
    if (modelRes.ok) {
      const model = await modelRes.json()
      model.globals = {
        ...model.globals,
        lan_interface: lanInterface.value,
        wan_interface: wanInterface.value,
        dial_mode: dialMode.value,
        auto_config_kernel_parameter: autoKernel.value ? 'true' : 'false',
        log_level: logLevel.value
      }
      model.nodes = existingNodes.value.map(n => ({
        key: n.key,
        protocol: n.protocol,
        name: n.name,
        address: n.address,
        port: n.port,
        url: n.url
      }))

      // Serialize and save
      const serRes = await fetch('/api/config/serialize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(model)
      })
      const serData = await serRes.json()
      if (serRes.ok && serData.raw) {
        let profileName = saveProfileName.value.trim() || 'config.quick.dae'
        if (!profileName.endsWith('.dae')) {
          profileName += '.dae'
        }
        saveProfileName.value = profileName
        const contentWithNote = `# note: 快速设置向导生成配置\n` + serData.raw

        // 2a. Save named profile in configs/
        const saveRes = await fetch('/api/config', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: profileName,
            content: contentWithNote,
            autoBackup: true,
            reloadConfig: true
          })
        })
        const saveData = await saveRes.json()
        if (!saveRes.ok || saveData.error) {
          throw new Error(saveData.error || saveData.details || t('quick.msg_save_config_failed'))
        }

        // 2b. Activate this profile so it is live in dae and prominent in Config Management
        await fetch('/api/configs/activate', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: profileName })
        })
      }
    }

    // 3. Run quick test
    await runStep4Diagnostics()
    applySuccess.value = true
  } catch (err: any) {
    alert(t('quick.msg_apply_failed') + err.message)
  } finally {
    applying.value = false
  }
}

const selectedMode = computed(() => {
  return modesList.value.find(m => m.id === selectedModeId.value) || modesList.value[0]
})

onMounted(async () => {
  await Promise.all([
    fetchInterfaces(),
    fetchCurrentGlobals(),
    fetchModes()
  ])
})
</script>

<template>
  <div class="quick-setup-view">
    <!-- Header -->
    <header class="wizard-header">
      <div class="header-left">
        <div class="header-icon-box">
          <Wand2 :size="24" class="text-primary" />
        </div>
        <div>
          <div class="header-title-row">
            <h2>{{ t('quick.title') }}</h2>
            <span class="badge-rec">{{ t('quick.badge') }}</span>
          </div>
          <p class="header-subtitle">{{ t('quick.subtitle') }}</p>
        </div>
      </div>
      <div class="header-right">
        <button class="btn btn-secondary btn-sm" @click="emit('navigate-tab', 'home')">
          <ArrowLeft :size="13" />
          <span>{{ t('quick.btn_back_home') }}</span>
        </button>
      </div>
    </header>

    <!-- Stepper Indicator -->
    <div class="stepper-bar">
      <div
        class="step-item"
        :class="{ active: currentStep === 1, completed: currentStep > 1 }"
        @click="currentStep = 1"
      >
        <div class="step-num">
          <Check v-if="currentStep > 1" :size="14" />
          <span v-else>1</span>
        </div>
        <div class="step-info">
          <div class="step-label">{{ t('quick.step1_label') }}</div>
          <div class="step-sub">{{ t('quick.step1_sub') }}</div>
        </div>
      </div>

      <div class="step-divider" :class="{ completed: currentStep > 1 }"></div>

      <div
        class="step-item"
        :class="{ active: currentStep === 2, completed: currentStep > 2 }"
        @click="currentStep = 2"
      >
        <div class="step-num">
          <Check v-if="currentStep > 2" :size="14" />
          <span v-else>2</span>
        </div>
        <div class="step-info">
          <div class="step-label">{{ t('quick.step2_label') }}</div>
          <div class="step-sub">{{ t('quick.step2_sub') }}</div>
        </div>
      </div>

      <div class="step-divider" :class="{ completed: currentStep > 2 }"></div>

      <div
        class="step-item"
        :class="{ active: currentStep === 3, completed: currentStep > 3 }"
        @click="currentStep = 3"
      >
        <div class="step-num">
          <Check v-if="currentStep > 3" :size="14" />
          <span v-else>3</span>
        </div>
        <div class="step-info">
          <div class="step-label">{{ t('quick.step3_label') }}</div>
          <div class="step-sub">{{ t('quick.step3_sub') }}</div>
        </div>
      </div>

      <div class="step-divider" :class="{ completed: currentStep > 3 }"></div>

      <div
        class="step-item"
        :class="{ active: currentStep === 4, completed: applySuccess }"
        @click="currentStep = 4"
      >
        <div class="step-num">
          <Check v-if="applySuccess" :size="14" />
          <span v-else>4</span>
        </div>
        <div class="step-info">
          <div class="step-label">{{ t('quick.step4_label') }}</div>
          <div class="step-sub">{{ t('quick.step4_sub') }}</div>
        </div>
      </div>
    </div>

    <!-- Wizard Body -->
    <div class="wizard-container">
      <!-- STEP 1: Network & Global -->
      <div v-if="currentStep === 1" class="step-card">
        <div class="step-head">
          <div class="step-badge">{{ t('quick.step_badge', { current: 1, total: 4 }) }}</div>
          <h3>{{ t('quick.step1_title') }}</h3>
          <p class="step-desc">{{ t('quick.step1_desc') }}</p>
        </div>

        <!-- Scenario Pickers -->
        <div class="scenario-grid">
          <div
            class="scenario-card"
            :class="{ active: networkScenario === 'gateway' }"
            @click="selectScenario('gateway')"
          >
            <div class="sc-top">
              <div class="sc-icon"><Network :size="20" /></div>
              <div class="sc-title">{{ t('quick.scenario_gateway_title') }}</div>
              <span class="badge-rec">{{ t('quick.badge_recommended') }}</span>
            </div>
            <p class="sc-desc">{{ t('quick.scenario_gateway_desc') }}</p>
            <div class="sc-tags">
              <span>{{ t('quick.tag_lan_physical') }}</span>
              <span>{{ t('quick.tag_wan_auto') }}</span>
            </div>
          </div>

          <div
            class="scenario-card"
            :class="{ active: networkScenario === 'single' }"
            @click="selectScenario('single')"
          >
            <div class="sc-top">
              <div class="sc-icon"><Sliders :size="20" /></div>
              <div class="sc-title">{{ t('quick.scenario_single_title') }}</div>
            </div>
            <p class="sc-desc">{{ t('quick.scenario_single_desc') }}</p>
            <div class="sc-tags">
              <span>{{ t('quick.tag_lan_docker') }}</span>
              <span>{{ t('quick.tag_wan_auto') }}</span>
            </div>
          </div>
        </div>

        <!-- Interfaces Form -->
        <div class="form-box mt-4">
          <div class="form-row-2">
            <div class="form-group">
              <label class="form-label">
                <span>{{ t('quick.field_lan') }}</span>
                <span class="tip-pill" :title="t('quick.tip_lan')">?</span>
              </label>
              <div class="input-with-chips">
                <input v-model="lanInterface" class="form-control" :placeholder="t('quick.placeholder_lan')" />
                <div v-if="interfaces.length > 0" class="chip-row">
                  <span class="chip-label">{{ t('quick.detected_nics') }}</span>
                  <button
                    v-for="i in interfaces.filter(item => !item.isLoop)"
                    :key="i.name"
                    class="chip-btn"
                    :class="{ active: lanInterface === i.name }"
                    @click="lanInterface = i.name"
                  >
                    {{ i.name }} <span v-if="i.isDefault" class="chip-def">{{ t('quick.nic_default') }}</span>
                  </button>
                </div>
              </div>
              <div class="form-help">{{ t('quick.help_lan') }}</div>
            </div>

            <div class="form-group">
              <label class="form-label">
                <span>{{ t('quick.field_wan') }}</span>
                <span class="tip-pill" :title="t('quick.tip_wan')">?</span>
              </label>
              <input v-model="wanInterface" class="form-control" :placeholder="t('quick.placeholder_wan')" />
              <div class="form-help">{{ t('quick.help_wan') }}</div>
            </div>
          </div>

          <!-- Recommended Core Params -->
          <div class="recommend-params-box mt-3">
            <div class="rec-title">
              <Sparkles :size="15" class="text-warning mr-1" />
              <span>{{ t('quick.rec_params_title') }}</span>
            </div>
            <div class="rec-grid">
              <div class="rec-item">
                <span class="k">{{ t('quick.param_dial_mode') }}</span>
                <span class="v"><code>domain</code>{{ t('quick.param_dial_mode_desc') }}</span>
              </div>
              <div class="rec-item">
                <span class="k">{{ t('quick.param_kernel') }}</span>
                <span class="v"><code>true</code>{{ t('quick.param_kernel_desc') }}</span>
              </div>
              <div class="rec-item">
                <span class="k">{{ t('quick.param_log_level') }}</span>
                <span class="v"><code>info</code>{{ t('quick.param_log_level_desc') }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="step-footer">
          <div></div>
          <button class="btn btn-primary" @click="currentStep = 2">
            <span>{{ t('quick.btn_next_to_nodes') }}</span>
            <ArrowRight :size="15" />
          </button>
        </div>
      </div>

      <!-- STEP 2: Nodes & Subscriptions -->
      <div v-if="currentStep === 2" class="step-card">
        <div class="step-head">
          <div class="step-badge">{{ t('quick.step_badge', { current: 2, total: 4 }) }}</div>
          <h3>{{ t('quick.step2_title') }}</h3>
          <p class="step-desc">{{ t('quick.step2_desc') }}</p>
        </div>

        <!-- Node Add Methods -->
        <div class="add-node-grid">
          <!-- 1. Subscription URL Import -->
          <div class="import-card">
            <div class="import-card-head">
              <Link2 :size="18" class="text-primary mr-2" />
              <strong>{{ t('quick.method1_title') }}</strong>
            </div>
            <p class="text-xs text-muted mb-2">{{ t('quick.method1_desc') }}</p>
            <div class="import-input-row">
              <input
                v-model="subUrl"
                class="form-control full-width-input"
                :placeholder="t('quick.sub_placeholder')"
              />
              <button
                class="btn btn-primary btn-fetch"
                :disabled="fetchingSub || !subUrl.trim()"
                @click="handleFetchSub"
              >
                <RefreshCw v-if="fetchingSub" :size="14" class="spin" />
                <Plus v-else :size="14" />
                <span>{{ fetchingSub ? t('quick.fetching_sub') : t('quick.btn_fetch_sub') }}</span>
              </button>
            </div>
            <div v-if="subMessage" class="sub-alert success mt-2">
              <CheckCircle2 :size="14" />
              <span>{{ subMessage }}</span>
            </div>
          </div>

          <!-- 2. Single Node Link Paste -->
          <div class="import-card">
            <div class="import-card-head">
              <Server :size="18" class="text-info mr-2" />
              <strong>{{ t('quick.method2_title') }}</strong>
            </div>
            <p class="text-xs text-muted mb-2">{{ t('quick.method2_desc') }}</p>
            <div class="import-textarea-block">
              <textarea
                v-model="singleNodeLink"
                class="form-control mono text-xs full-width-textarea mb-2"
                rows="3"
                :placeholder="t('quick.node_placeholder')"
              ></textarea>
              <div class="textarea-btn-row">
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="!singleNodeLink.trim()"
                  @click="handleAddSingleNode"
                >
                  <Plus :size="14" />
                  <span>{{ t('quick.btn_parse_add') }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Node List Preview -->
        <div class="nodes-list-section mt-4">
          <div class="section-subhead">
            <div class="flex-align-center gap-2">
              <Layers :size="16" class="text-primary" />
              <strong>{{ t('quick.ready_nodes_title') }}</strong>
              <span class="count-pill">{{ t('quick.unit_nodes', { count: existingNodes.length }) }}</span>
            </div>
            <div class="flex-align-center gap-2">
              <button
                v-if="systemExistingNodes.length > 0 && existingNodes.length === 0"
                class="btn btn-secondary btn-xs btn-reuse-nodes"
                @click="reuseSystemNodes"
                :title="t('quick.reuse_nodes_tip')"
              >
                <RotateCcw :size="12" />
                <span>{{ t('quick.btn_reuse_nodes', { count: systemExistingNodes.length }) }}</span>
              </button>
              <span v-if="existingNodes.length > 0" class="text-xs text-success flex-align-center gap-1">
                <CheckCircle2 :size="13" /> {{ t('quick.nodes_ready_tip') }}
              </span>
            </div>
          </div>

          <div v-if="existingNodes.length === 0" class="empty-nodes-box">
            <AlertTriangle :size="24" class="text-warning mb-2" />
            <div class="font-bold">{{ t('quick.empty_nodes_title') }}</div>
            <p class="text-xs text-muted">{{ t('quick.empty_nodes_desc') }}</p>
          </div>

          <div v-else class="ready-nodes-grid">
            <div v-for="(n, idx) in existingNodes" :key="idx" class="node-chip-card">
              <div class="node-proto-tag">{{ n.protocol }}</div>
              <div class="node-info-text">
                <div class="node-name-text truncate" :title="n.name">{{ n.name }}</div>
                <div class="node-addr-text mono text-muted truncate">{{ n.address }}:{{ n.port }}</div>
              </div>
              <button class="btn-del-node" @click="removeNode(idx)" :title="t('quick.btn_del_node')">
                <Trash2 :size="12" />
              </button>
            </div>
          </div>
        </div>

        <div class="step-footer">
          <button class="btn btn-secondary" @click="currentStep = 1">
            <ArrowLeft :size="15" />
            <span>{{ t('quick.btn_prev_step') }}</span>
          </button>
          <button class="btn btn-primary" :disabled="existingNodes.length === 0" @click="currentStep = 3">
            <span>{{ t('quick.btn_next_to_modes') }}</span>
            <ArrowRight :size="15" />
          </button>
        </div>
      </div>

      <!-- STEP 3: Routing Modes -->
      <div v-if="currentStep === 3" class="step-card">
        <div class="step-head">
          <div class="step-badge">{{ t('quick.step_badge', { current: 3, total: 4 }) }}</div>
          <h3>{{ t('quick.step3_title') }}</h3>
          <p class="step-desc">{{ t('quick.step3_desc') }}</p>
        </div>

        <!-- Mode Cards Grid -->
        <div class="mode-select-grid">
          <div
            v-for="mode in modesList"
            :key="mode.id"
            class="mode-pick-card"
            :class="{ active: mode.id === selectedModeId }"
            @click="selectedModeId = mode.id"
          >
            <div class="mp-head">
              <div class="mp-icon" :class="mode.id">
                <Compass v-if="mode.id === 'bypass_cn'" :size="22" />
                <Shield v-else-if="mode.id === 'gfwlist'" :size="22" />
                <Flag v-else-if="mode.id === 'only_cn'" :size="22" />
                <Globe v-else-if="mode.id === 'global'" :size="22" />
                <Sliders v-else :size="22" />
              </div>
              <div class="mp-title-box">
                <div class="mp-title-row">
                  <h4>{{ getLocalizedMode(mode).name }}</h4>
                  <span class="mp-badge">{{ getLocalizedMode(mode).badge }}</span>
                </div>
              </div>
              <div class="mp-radio">
                <div class="radio-circle" :class="{ checked: mode.id === selectedModeId }"></div>
              </div>
            </div>

            <p class="mp-desc">{{ getLocalizedMode(mode).description }}</p>

            <div class="mp-bottom">
              <span class="mp-rules-count">{{ t('quick.unit_rules', { count: mode.rules.length }) }}</span>
              <button class="btn-text-rules" @click.stop="openViewRules(mode)">
                <Eye :size="12" />
                <span>{{ t('quick.btn_view_rules') }}</span>
              </button>
              <button
                v-if="mode.isCustom"
                class="btn-del-custom"
                @click.stop="deleteCustomMode(mode)"
                :title="t('quick.del_custom_mode')"
              >
                <Trash2 :size="12" />
              </button>
            </div>
          </div>
        </div>

        <!-- Create Custom Mode Trigger -->
        <div class="custom-mode-trigger-bar mt-3">
          <span>{{ t('quick.custom_mode_prompt') }}</span>
          <button class="btn btn-secondary btn-sm" @click="openCreateMode">
            <Plus :size="13" />
            <span>{{ t('quick.btn_new_custom_mode') }}</span>
          </button>
        </div>

        <div class="step-footer">
          <button class="btn btn-secondary" @click="currentStep = 2">
            <ArrowLeft :size="15" />
            <span>{{ t('quick.btn_prev_step') }}</span>
          </button>
          <button class="btn btn-primary" @click="currentStep = 4; runStep4Diagnostics()">
            <span>{{ t('quick.btn_next_to_test') }}</span>
            <ArrowRight :size="15" />
          </button>
        </div>
      </div>

      <!-- STEP 4: Diagnostics & Apply -->
      <div v-if="currentStep === 4" class="step-card">
        <div class="step-head">
          <div class="step-badge">{{ t('quick.step_badge', { current: 4, total: 4 }) }}</div>
          <h3>{{ t('quick.step4_title') }}</h3>
          <p class="step-desc">{{ t('quick.step4_desc') }}</p>
        </div>

        <!-- Configuration Summary Review Box -->
        <div class="summary-review-box">
          <div class="sr-item">
            <span class="sr-label">{{ t('quick.review_interfaces') }}</span>
            <span class="sr-val">LAN: <b>{{ lanInterface }}</b> | WAN: <b>{{ wanInterface }}</b></span>
          </div>
          <div class="sr-item">
            <span class="sr-label">{{ t('quick.review_nodes') }}</span>
            <span class="sr-val">{{ t('quick.review_nodes_count', { count: existingNodes.length }) }}</span>
          </div>
          <div class="sr-item">
            <span class="sr-label">{{ t('quick.review_mode') }}</span>
            <span class="sr-val highlight">{{ getLocalizedMode(selectedMode).name }} ({{ getLocalizedMode(selectedMode).badge }})</span>
          </div>
          <div class="sr-item">
            <span class="sr-label">{{ t('quick.review_dns') }}</span>
            <span class="sr-val">{{ t('quick.review_dns_val', { domestic: 'alidns', overseas: 'googledns' }) }}</span>
          </div>
          <div class="sr-item">
            <span class="sr-label">{{ t('quick.review_save_profile') }}</span>
            <div class="flex-align-center gap-1">
              <input
                v-model="saveProfileName"
                class="form-control form-control-xs font-mono"
                placeholder="config.quick.dae"
                :title="t('quick.review_save_profile_title')"
              />
            </div>
            <span class="text-xs text-muted" style="font-size: 11px;">{{ t('quick.review_profile_tip') }}</span>
          </div>
        </div>

        <!-- Fast Connectivity Test Row -->
        <div class="diagnostics-panel mt-4">
          <div class="diag-head">
            <div class="flex-align-center gap-2">
              <Zap :size="16" class="text-warning" />
              <strong>{{ t('quick.diag_test_title') }}</strong>
            </div>
            <button
              class="btn btn-secondary btn-xs"
              :disabled="testingMode"
              @click="runStep4Diagnostics"
            >
              <RefreshCw :size="12" :class="{ spin: testingMode }" />
              <span>{{ testingMode ? t('quick.testing') : t('quick.btn_retest') }}</span>
            </button>
          </div>

          <div class="diag-items-grid">
            <div v-for="t in testResults" :key="t.id" class="diag-item-card">
              <div class="diag-item-top">
                <span class="diag-name">{{ getTestItemName(t) }}</span>
                <span class="diag-badge-out" :class="{ direct: t.expectedOut.includes('直连') || t.expectedOut.includes('direct'), proxy: t.expectedOut.includes('代理') || t.expectedOut.includes('proxy') }">
                  {{ getTestItemExpected(t) }}
                </span>
              </div>
              <div class="diag-item-status mt-3">
                <span :class="['quality-badge', getSpeedQuality(t).badgeClass]">
                  <CheckCircle2 v-if="getSpeedQuality(t).level === 'fast'" :size="13" />
                  <AlertTriangle v-else-if="getSpeedQuality(t).level === 'slow'" :size="13" />
                  <XCircle v-else-if="getSpeedQuality(t).level === 'fail'" :size="13" />
                  <RefreshCw v-else :size="13" class="spin" />
                  <span>{{ getSpeedQuality(t).text }}</span>
                </span>
              </div>
              <div class="diag-msg text-xs text-muted mt-2" :title="getTestItemMessage(t)">
                {{ getTestItemMessage(t) }}
              </div>
            </div>
          </div>
        </div>

        <!-- Success Celebration Alert -->
        <div v-if="applySuccess" class="success-celebration-banner mt-4">
          <div class="sc-icon-circle"><CheckCircle2 :size="28" /></div>
          <div style="flex: 1;">
            <h4 class="text-success font-bold text-base">{{ t('quick.success_title') }}</h4>
            <p class="text-sm text-muted mt-1">{{ t('quick.success_desc', { name: saveProfileName || 'config.quick.dae' }) }}</p>
          </div>
          <div class="flex-align-center gap-2">
            <button class="btn btn-secondary btn-sm" @click="emit('navigate-tab', 'config')">
              <FileCode :size="14" class="mr-1" />
              <span>{{ t('quick.btn_goto_config') }}</span>
            </button>
            <button class="btn btn-success btn-sm" @click="emit('navigate-tab', 'home')">
              <Activity :size="14" class="mr-1" />
              <span>{{ t('quick.btn_goto_home') }}</span>
            </button>
          </div>
        </div>

        <div class="step-footer mt-4">
          <button class="btn btn-secondary" @click="currentStep = 3">
            <ArrowLeft :size="15" />
            <span>{{ t('quick.btn_prev_step') }}</span>
          </button>
          <div class="flex-align-center gap-2">
            <button
              v-if="!applySuccess"
              class="btn btn-primary btn-lg btn-activate-final"
              :disabled="applying"
              @click="applyAndActivate"
            >
              <RefreshCw v-if="applying" :size="16" class="spin mr-1" />
              <CheckCircle2 v-else :size="16" class="mr-1" />
              <span>{{ applying ? t('quick.saving_activating') : t('quick.btn_save_activate') }}</span>
            </button>
            <template v-else>
              <button
                class="btn btn-secondary btn-lg"
                @click="emit('navigate-tab', 'config')"
              >
                <FileCode :size="16" class="mr-1" />
                <span>{{ t('quick.btn_goto_config') }}</span>
              </button>
              <button
                class="btn btn-success btn-lg"
                @click="emit('navigate-tab', 'home')"
              >
                <Activity :size="16" class="mr-1" />
                <span>{{ t('quick.btn_goto_dashboard') }}</span>
              </button>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- View Mode Rules Modal -->
    <div v-if="showModeRulesModal && viewingMode" class="modal-backdrop" @click.self="showModeRulesModal = false">
      <div class="modal-card modal-lg">
        <div class="modal-header">
          <div class="modal-title-wrap">
            <Compass :size="18" class="text-primary mr-2" />
            <h3>{{ getLocalizedMode(viewingMode).name }} - {{ t('quick.modal_rules_detail') }}</h3>
          </div>
          <button class="btn-icon-subtle" @click="showModeRulesModal = false"><X :size="16" /></button>
        </div>
        <div class="modal-body modal-scroll-body">
          <p class="text-xs text-muted mb-3">{{ getLocalizedMode(viewingMode).description }}</p>

          <!-- DNS Rules preview -->
          <div class="rule-group-box mb-3">
            <h5 class="text-xs font-bold text-primary mb-2 flex-align-center gap-1">
              <Globe :size="14" /> {{ t('quick.modal_dns_rules') }}
            </h5>
            <div class="rules-table-box">
              <table class="simple-rules-table">
                <thead>
                  <tr><th>{{ t('quick.modal_col_condition') }}</th><th>{{ t('quick.modal_col_upstream') }}</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(r, idx) in viewingMode.dnsSettings.requestRules" :key="idx">
                    <td><code>{{ r.condition }}</code></td>
                    <td><span class="badge-target">{{ r.target }}</span></td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Routing Rules preview -->
          <div class="rule-group-box">
            <h5 class="text-xs font-bold text-primary mb-2 flex-align-center gap-1">
              <Sliders :size="14" /> {{ t('quick.modal_routing_rules') }}
            </h5>
            <div class="rules-table-box">
              <table class="simple-rules-table">
                <thead>
                  <tr><th>{{ t('quick.modal_col_condition') }}</th><th>{{ t('quick.modal_col_outbound') }}</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(r, idx) in viewingMode.rules" :key="idx">
                    <td><code>{{ r.condition }}</code></td>
                    <td>
                      <span class="badge-target" :class="{ direct: r.target.includes('direct'), proxy: r.target.includes('proxy') || r.target.includes('streaming'), block: r.target.includes('block') }">
                        {{ r.target }}
                      </span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showModeRulesModal = false">{{ t('common.close') }}</button>
          <button
            class="btn btn-primary"
            @click="selectedModeId = viewingMode.id; showModeRulesModal = false"
          >
            <Check :size="14" />
            <span>{{ t('quick.btn_use_mode') }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Create Custom Mode Modal -->
    <div v-if="showNewModeModal" class="modal-backdrop" @click.self="showNewModeModal = false">
      <div class="modal-card modal-lg">
        <div class="modal-header">
          <div class="modal-title-wrap">
            <Plus :size="18" class="text-primary mr-2" />
            <h3 v-if="createModeStep === 1">{{ t('quick.modal_new_mode_basic') }}</h3>
            <h3 v-else>{{ t('quick.modal_new_mode_rules') }}</h3>
          </div>
          <button class="btn-icon-subtle" @click="showNewModeModal = false"><X :size="16" /></button>
        </div>

        <div v-if="createModeStep === 1" class="modal-body">
          <div class="form-group">
            <label class="form-label required">{{ t('quick.form_mode_name') }}</label>
            <input v-model="newModeForm.name" class="form-control" :placeholder="t('quick.form_mode_name_ph')" />
          </div>
          <div class="form-group mt-3">
            <label class="form-label">{{ t('quick.form_mode_badge') }}</label>
            <input v-model="newModeForm.badge" class="form-control" :placeholder="t('quick.form_mode_badge_ph')" />
          </div>
          <div class="form-group mt-3">
            <label class="form-label">{{ t('quick.form_mode_desc') }}</label>
            <textarea v-model="newModeForm.description" class="form-control" rows="3" :placeholder="t('quick.form_mode_desc_ph')"></textarea>
          </div>
        </div>

        <div v-else class="modal-body modal-scroll-body">
          <p class="modal-notice-text">{{ t('quick.modal_notice') }}</p>
          <div class="mode-config-section mb-4 p-3 bg-subtle rounded border">
            <div class="preview-header mb-2 flex-align-center">
              <Globe :size="15" class="text-info mr-2" />
              <strong class="text-sm">{{ t('quick.modal_link_dns') }}</strong>
            </div>
            <div class="text-xs text-muted mb-1 font-semibold">{{ t('quick.modal_upstream_dns') }}</div>
            <div v-for="(u, idx) in newModeForm.dnsSettings.upstreams" :key="idx" class="flex-gap mb-1">
              <input v-model="u.tag" class="form-control form-control-xs w-1/3" placeholder="tag" />
              <input v-model="u.address" class="form-control form-control-xs flex-1 mono" placeholder="address" />
              <button class="btn-icon-subtle danger" @click="newModeForm.dnsSettings.upstreams.splice(idx, 1)"><Trash2 :size="12" /></button>
            </div>
          </div>

          <div class="mode-config-section p-3 bg-subtle rounded border">
            <div class="preview-header mb-2 flex-align-center">
              <Sliders :size="15" class="text-primary mr-2" />
              <strong class="text-sm">{{ t('quick.modal_routing_lines') }}</strong>
            </div>
            <textarea v-model="newModeForm.rulesText" class="form-control mono text-xs" rows="8"></textarea>
          </div>
        </div>

        <div class="modal-actions">
          <button v-if="createModeStep === 1" class="btn btn-secondary" @click="showNewModeModal = false">{{ t('common.cancel') }}</button>
          <button v-if="createModeStep === 1" class="btn btn-primary" @click="createModeStep = 2">{{ t('quick.btn_next_step') }} &rarr;</button>

          <button v-if="createModeStep === 2" class="btn btn-secondary" @click="createModeStep = 1">{{ t('quick.btn_prev_step') }}</button>
          <button v-if="createModeStep === 2" class="btn btn-primary" :disabled="savingNewMode" @click="saveCustomMode">
            <Check :size="14" /> {{ t('quick.btn_save_custom_mode') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.quick-setup-view {
  height: 100%;
  overflow-y: auto;
  padding: 24px;
  background: var(--bg-main, #090d16);
  color: var(--text-main, #f8fafc);
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Header */
.wizard-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--bg-card, #0f172a);
  padding: 18px 24px;
  border-radius: 12px;
  border: 1px solid var(--border-color, #1e293b);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-icon-box {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  background: rgba(14, 165, 233, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-title-row h2 {
  font-size: 18px;
  font-weight: 700;
  margin: 0;
}

.badge-rec {
  background: linear-gradient(135deg, #0284c7, #0ea5e9);
  color: #fff;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 12px;
  font-weight: 600;
}

.header-subtitle {
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
  margin: 4px 0 0 0;
}

/* Stepper */
.stepper-bar {
  display: flex;
  align-items: center;
  background: var(--bg-card, #0f172a);
  padding: 16px 24px;
  border-radius: 12px;
  border: 1px solid var(--border-color, #1e293b);
  gap: 12px;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.step-item:hover {
  background: rgba(255, 255, 255, 0.04);
}

.step-num {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--bg-input, #1e293b);
  color: var(--text-muted, #94a3b8);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 700;
  transition: all 0.2s ease;
}

.step-item.active .step-num {
  background: #0284c7;
  color: #fff;
  box-shadow: 0 0 10px rgba(2, 132, 199, 0.5);
}

.step-item.completed .step-num {
  background: #10b981;
  color: #fff;
}

.step-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
}

.step-item.active .step-label {
  color: #38bdf8;
}

.step-item.completed .step-label {
  color: var(--text-main, #f8fafc);
}

.step-sub {
  font-size: 11px;
  color: var(--text-muted, #64748b);
}

.step-divider {
  flex: 1;
  height: 2px;
  background: var(--border-color, #1e293b);
  transition: all 0.2s ease;
}

.step-divider.completed {
  background: #10b981;
}

/* Wizard Body */
.wizard-container {
  flex: 1;
}

.step-card {
  background: var(--bg-card, #0f172a);
  border-radius: 12px;
  border: 1px solid var(--border-color, #1e293b);
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
}

.step-head {
  margin-bottom: 20px;
}

.step-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 700;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  padding: 2px 8px;
  border-radius: 6px;
  margin-bottom: 6px;
}

.step-head h3 {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
}

.step-desc {
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
  margin: 6px 0 0 0;
}

/* Step 1: Scenarios */
.scenario-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.scenario-card {
  background: rgba(255, 255, 255, 0.02);
  border: 2px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 18px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.scenario-card:hover {
  border-color: #38bdf8;
  background: rgba(56, 189, 248, 0.04);
}

.scenario-card.active {
  border-color: #0284c7;
  background: rgba(2, 132, 199, 0.08);
  box-shadow: 0 4px 12px rgba(2, 132, 199, 0.15);
}

.sc-top {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.sc-icon {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.06);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #38bdf8;
}

.sc-title {
  font-size: 15px;
  font-weight: 700;
  flex: 1;
}

.sc-desc {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.5;
  margin-bottom: 12px;
}

.sc-tags {
  display: flex;
  gap: 8px;
  font-size: 11px;
}

.sc-tags span {
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 8px;
  border-radius: 4px;
  color: var(--text-muted, #cbd5e1);
}

/* Form Box */
.form-box {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 18px;
}

.form-row-2 {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
}

.tip-pill {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  cursor: help;
  color: #38bdf8;
}

.input-with-chips {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chip-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.chip-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.chip-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #e2e8f0);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  font-family: monospace;
}

.chip-btn:hover {
  border-color: #38bdf8;
}

.chip-btn.active {
  background: #0284c7;
  border-color: #0284c7;
  color: #fff;
}

.form-help {
  font-size: 11px;
  color: var(--text-muted, #64748b);
  line-height: 1.4;
}

.recommend-params-box {
  background: rgba(245, 158, 11, 0.05);
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 8px;
  padding: 12px 16px;
}

.rec-title {
  font-size: 12px;
  font-weight: 600;
  color: #fbbf24;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
}

.rec-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.rec-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rec-item .k {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.rec-item .v {
  font-size: 12px;
}

/* Step 2: Nodes */
.add-node-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.full-width-input {
  flex: 1;
  width: 100%;
}

.full-width-textarea {
  width: 100%;
  resize: vertical;
}

.import-textarea-block {
  display: flex;
  flex-direction: column;
}

.textarea-btn-row {
  display: flex;
  justify-content: flex-end;
}

.import-card {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
}

.import-card-head {
  display: flex;
  align-items: center;
  font-size: 14px;
  margin-bottom: 6px;
}

.import-input-row {
  display: flex;
  gap: 8px;
}

.sub-alert {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  padding: 6px 10px;
  border-radius: 6px;
}

.sub-alert.success {
  background: rgba(16, 185, 129, 0.1);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.nodes-list-section {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
}

.section-subhead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.count-pill {
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
}

.empty-nodes-box {
  text-align: center;
  padding: 30px;
  color: var(--text-muted, #94a3b8);
}

.ready-nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 10px;
  max-height: 240px;
  overflow-y: auto;
}

.node-chip-card {
  display: flex;
  align-items: center;
  gap: 10px;
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 8px 12px;
}

.node-proto-tag {
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
}

.node-info-text {
  flex: 1;
  min-width: 0;
}

.node-name-text {
  font-size: 12px;
  font-weight: 600;
}

.node-addr-text {
  font-size: 11px;
}

.btn-del-node {
  background: transparent;
  border: none;
  color: #64748b;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
}

.btn-del-node:hover {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.1);
}

/* Step 3: Modes */
.mode-select-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.mode-pick-card {
  background: rgba(255, 255, 255, 0.02);
  border: 2px solid var(--border-color, #1e293b);
  border-radius: 12px;
  padding: 18px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  transition: all 0.2s ease;
}

.mode-pick-card:hover {
  border-color: #38bdf8;
  background: rgba(56, 189, 248, 0.04);
}

.mode-pick-card.active {
  border-color: #0284c7;
  background: rgba(2, 132, 199, 0.08);
  box-shadow: 0 4px 16px rgba(2, 132, 199, 0.18);
}

.mp-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.mp-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #38bdf8;
}

.mp-icon.bypass_cn { color: #10b981; background: rgba(16, 185, 129, 0.12); }
.mp-icon.gfwlist { color: #3b82f6; background: rgba(59, 130, 246, 0.12); }
.mp-icon.only_cn { color: #f59e0b; background: rgba(245, 158, 11, 0.12); }
.mp-icon.global { color: #ec4899; background: rgba(236, 72, 153, 0.12); }

.mp-title-box {
  flex: 1;
}

.mp-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mp-title-row h4 {
  font-size: 16px;
  font-weight: 700;
  margin: 0;
}

.mp-badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
}

.radio-circle {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid var(--border-color, #475569);
  position: relative;
  transition: all 0.2s ease;
}

.radio-circle.checked {
  border-color: #0284c7;
  background: #0284c7;
}

.radio-circle.checked::after {
  content: '';
  position: absolute;
  top: 5px;
  left: 5px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #fff;
}

.mp-desc {
  font-size: 12px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.5;
  flex: 1;
  margin: 0 0 14px 0;
}

.mp-bottom {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  padding-top: 10px;
}

.mp-rules-count {
  color: var(--text-muted, #94a3b8);
}

.btn-text-rules {
  background: transparent;
  border: none;
  color: #38bdf8;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  font-size: 11px;
  padding: 0;
}

.btn-text-rules:hover {
  text-decoration: underline;
}

.btn-del-custom {
  margin-left: auto;
  background: transparent;
  border: none;
  color: #64748b;
  cursor: pointer;
}

.btn-del-custom:hover {
  color: #ef4444;
}

.custom-mode-trigger-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(255, 255, 255, 0.02);
  border: 1px dashed var(--border-color, #1e293b);
  padding: 12px 18px;
  border-radius: 8px;
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
}

/* Step 4: Diagnostics & Apply */
.summary-review-box {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
}

.sr-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sr-label {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
}

.sr-val {
  font-size: 13px;
}

.sr-val.highlight {
  color: #38bdf8;
  font-weight: 700;
}

.diagnostics-panel {
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 10px;
  padding: 16px;
}

.diag-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.diag-items-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.diag-item-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 8px;
  padding: 12px;
}

.diag-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.diag-name {
  font-size: 12px;
  font-weight: 600;
}

.diag-badge-out {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.diag-badge-out.direct {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
}

.diag-badge-out.proxy {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
}

.diag-item-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.quality-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 6px;
  letter-spacing: 0.2px;
}

.quality-fast {
  background: rgba(16, 185, 129, 0.2);
  color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.4);
}

.quality-slow {
  background: rgba(245, 158, 11, 0.2);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.4);
}

.quality-fail {
  background: rgba(239, 68, 68, 0.2);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.4);
}

.quality-pending {
  background: rgba(148, 163, 184, 0.15);
  color: #94a3b8;
  border: 1px solid rgba(148, 163, 184, 0.25);
}

.success-celebration-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 18px 20px;
  border-radius: 10px;
}

.sc-icon-circle {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* Step Footer */
.step-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 24px;
  padding-top: 18px;
  border-top: 1px solid var(--border-color, #1e293b);
}

.btn-activate-final {
  padding: 10px 24px;
  font-size: 15px;
  font-weight: 700;
  box-shadow: 0 4px 14px rgba(2, 132, 199, 0.4);
}

/* Simple Rules Table */
.simple-rules-table {
  width: 100%;
  font-size: 12px;
  border-collapse: collapse;
}

.simple-rules-table th, .simple-rules-table td {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color, #1e293b);
  text-align: left;
}

.simple-rules-table th {
  color: var(--text-muted, #94a3b8);
  font-weight: 600;
}

.badge-target {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.08);
}

.badge-target.direct { color: #34d399; background: rgba(16, 185, 129, 0.15); }
.badge-target.proxy { color: #60a5fa; background: rgba(59, 130, 246, 0.15); }
.badge-target.block { color: #f87171; background: rgba(239, 68, 68, 0.15); }

/* Common styles */
.form-control {
  background: var(--bg-input, #1e293b);
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
}

.form-control:focus {
  border-color: #0284c7;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s ease;
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
  border: 1px solid var(--border-color, #334155);
  color: var(--text-main, #f8fafc);
}
.btn-secondary:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.08);
}

.btn-success {
  background: #10b981;
  color: #fff;
}
.btn-success:hover:not(:disabled) {
  background: #059669;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-card {
  background: var(--bg-card, #0f172a);
  border: 1px solid var(--border-color, #1e293b);
  border-radius: 12px;
  width: 90%;
  max-width: 680px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #1e293b);
}

.modal-header h3 {
  font-size: 16px;
  font-weight: 700;
  margin: 0;
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
}

.modal-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border-color, #1e293b);
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
