<script setup lang="ts">
import { ref, onMounted, shallowRef, computed, watch } from 'vue'
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
  LayoutGrid
} from 'lucide-vue-next'
import { EditorState } from '@codemirror/state'
import { EditorView, lineNumbers, keymap } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'

interface BackupItem {
  name: string
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

interface DaeConfigModel {
  globals: Record<string, string>
  subscriptions: ConfigSubItem[]
  nodes: ConfigNodeItem[]
  groups: ConfigGroupItem[]
  routingRules: ConfigRouteRule[]
  rawDnsBlock: string
}

type Mode = 'form' | 'code'
type FormSection = 'nodes' | 'groups' | 'routing' | 'global' | 'subscriptions'

const currentMode = ref<Mode>('form')
const activeSection = ref<FormSection>('nodes')

const editorContainer = ref<HTMLDivElement | null>(null)
const editorView = shallowRef<EditorView | null>(null)

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

const showGroupModal = ref(false)
const groupModalMode = ref<'add' | 'edit'>('add')
const editingGroup = ref<{
  name: string
  policy: string
  filtersText: string
}>({
  name: '',
  policy: 'min_moving_avg',
  filtersText: ''
})

const showRouteModal = ref(false)
const routeModalMode = ref<'add' | 'edit'>('add')
const editingRoute = ref<{
  condition: string
  target: string
  index: number
}>({
  condition: '',
  target: 'proxy',
  index: -1
})

const showSubModal = ref(false)
const editingSub = ref<ConfigSubItem>({ tag: '', url: '' })

// Initialize CodeMirror editor
function initEditor(content: string) {
  if (!editorContainer.value) return

  if (editorView.value) {
    editorView.value.destroy()
  }

  const startState = EditorState.create({
    doc: content,
    extensions: [
      lineNumbers(),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          configContent.value = update.state.doc.toString()
          isDirty.value = true
          validationStatus.value.checked = false
        }
      }),
      EditorView.theme({
        '&': {
          height: '100%',
          fontSize: '13px',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
          backgroundColor: '#0f172a',
          color: '#e2e8f0',
        },
        '.cm-content': { padding: '12px 0' },
        '.cm-line': { padding: '0 16px' },
        '.cm-gutters': {
          backgroundColor: '#090d16',
          color: '#475569',
          borderRight: '1px solid #1e293b',
        },
        '.cm-activeLine': { backgroundColor: '#1e293b55' },
        '.cm-activeLineGutter': { backgroundColor: '#1e293b', color: '#94a3b8' },
      })
    ]
  })

  editorView.value = new EditorView({
    state: startState,
    parent: editorContainer.value
  })
}

function setEditorText(text: string) {
  if (!editorView.value) {
    initEditor(text)
    return
  }
  const curr = editorView.value.state.doc.toString()
  if (curr !== text) {
    editorView.value.dispatch({
      changes: { from: 0, to: curr.length, insert: text }
    })
  }
  configContent.value = text
  isDirty.value = false
}

async function loadConfig() {
  loading.value = true
  try {
    const [resRaw, resModel] = await Promise.all([
      fetch('/api/config'),
      fetch('/api/config/model')
    ])
    if (resRaw.ok) {
      const data = await resRaw.json()
      configPath.value = data.path
      lastModTime.value = data.modTime
      setEditorText(data.content)
      validateContent(data.content, false)
    }
    if (resModel.ok) {
      configModel.value = await resModel.json()
    }
  } catch (err: any) {
    alert('加载配置失败: ' + err.message)
  } finally {
    loading.value = false
  }
}

async function switchMode(mode: Mode) {
  if (currentMode.value === mode) return

  if (mode === 'code') {
    // Sync model to code
    await syncModelToCode()
    currentMode.value = 'code'
    setTimeout(() => {
      initEditor(configContent.value)
    }, 50)
  } else {
    // Sync code to model
    currentMode.value = 'form'
    await loadConfig()
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
      message: data.message || (data.valid ? '语法与规则完全有效' : '验证未通过')
    }
    if (showAlert && !data.valid) {
      alert('dae validate 错误提示:\n' + data.message)
    }
  } catch (e: any) {
    validationStatus.value = {
      checked: true,
      valid: false,
      message: '验证请求失败: ' + e.message
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
    if (!confirm('当前配置检测存在语法或校验错误，确认强行保存吗？')) {
      return
    }
  }

  saving.value = true
  try {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: configContent.value,
        autoBackup: autoBackup.value,
        reloadConfig: reloadAfterSave.value
      })
    })
    const data = await res.json()
    if (res.ok) {
      isDirty.value = false
      alert('保存成功！' + (data.reloaded ? ' (dae 服务已自动热重载)' : ''))
      loadConfig()
    } else {
      alert('保存失败: ' + (data.error || '') + '\n' + (data.details || ''))
    }
  } catch (e: any) {
    alert('网络请求失败: ' + e.message)
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
    alert('请填写节点标识 Key 与连接 URL')
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
  if (!confirm(`确认删除节点 [${key}]？`)) return
  configModel.value.nodes = configModel.value.nodes.filter(n => n.key !== key)
  isDirty.value = true
  syncModelToCode()
}

// Group CRUD
function openAddGroup() {
  groupModalMode.value = 'add'
  editingGroup.value = {
    name: 'custom_group',
    policy: 'min_moving_avg',
    filtersText: 'filter: subtag(mock_sub_a)'
  }
  showGroupModal.value = true
}

function openEditGroup(g: ConfigGroupItem) {
  groupModalMode.value = 'edit'
  editingGroup.value = {
    name: g.name,
    policy: g.policy,
    filtersText: g.filters.join('\n')
  }
  showGroupModal.value = true
}

function saveGroupModal() {
  if (!editingGroup.value.name) {
    alert('组名不能为空')
    return
  }
  const filters = editingGroup.value.filtersText
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)
    .map(s => s.replace(/^filter:\s*/, ''))

  const groupItem: ConfigGroupItem = {
    name: editingGroup.value.name,
    policy: editingGroup.value.policy,
    filters
  }

  if (groupModalMode.value === 'add') {
    configModel.value.groups.push(groupItem)
  } else {
    const idx = configModel.value.groups.findIndex(g => g.name === editingGroup.value.name)
    if (idx >= 0) {
      configModel.value.groups[idx] = groupItem
    }
  }
  showGroupModal.value = false
  isDirty.value = true
  syncModelToCode()
}

function deleteGroup(name: string) {
  if (!confirm(`确认删除策略组 [${name}]？`)) return
  configModel.value.groups = configModel.value.groups.filter(g => g.name !== name)
  isDirty.value = true
  syncModelToCode()
}

// Routing CRUD
function openAddRoute() {
  routeModalMode.value = 'add'
  editingRoute.value = {
    condition: 'domain(geosite:custom)',
    target: 'proxy',
    index: -1
  }
  showRouteModal.value = true
}

function openEditRoute(r: ConfigRouteRule, idx: number) {
  routeModalMode.value = 'edit'
  editingRoute.value = {
    condition: r.condition,
    target: r.target,
    index: idx
  }
  showRouteModal.value = true
}

function saveRouteModal() {
  if (!editingRoute.value.condition || !editingRoute.value.target) {
    alert('分流匹配条件与目标出口动作不能为空')
    return
  }
  const item: ConfigRouteRule = {
    condition: editingRoute.value.condition,
    target: editingRoute.value.target
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
  window.open('/api/config/export', '_blank')
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
      if (confirm(`准备导入文件: ${file.name}，是否载入？`)) {
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
    alert('备份失败: ' + e)
  }
}

async function restoreBackup(name: string) {
  if (!confirm(`确认回滚恢复此备份？\n[${name}]\n恢复后当前未保存的编辑将被替换。`)) return
  try {
    const res = await fetch('/api/backups/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name })
    })
    const data = await res.json()
    if (res.ok) {
      alert('恢复成功！')
      showBackupModal.value = false
      loadConfig()
    } else {
      alert('恢复失败: ' + (data.error || ''))
    }
  } catch (e: any) {
    alert('请求失败: ' + e.message)
  }
}

async function deleteBackup(name: string) {
  if (!confirm(`确定删除备份文件 [${name}] 吗？`)) return
  try {
    const res = await fetch(`/api/backups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    if (res.ok) await fetchBackups()
  } catch (e) {
    alert('删除失败: ' + e)
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

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="config-view">
    <!-- Top toolbar -->
    <header class="toolbar">
      <div class="toolbar-left">
        <div class="mode-switcher">
          <button
            class="mode-btn"
            :class="{ active: currentMode === 'form' }"
            @click="switchMode('form')"
          >
            <LayoutGrid :size="14" />
            <span>可视化表单</span>
          </button>
          <button
            class="mode-btn"
            :class="{ active: currentMode === 'code' }"
            @click="switchMode('code')"
          >
            <Code :size="14" />
            <span>专业全文编辑</span>
          </button>
        </div>

        <div class="file-badge">
          <FileCode class="icon" :size="15" />
          <span class="file-name">{{ configPath ? configPath.split('/').pop() : 'config.mock.dae' }}</span>
          <span v-if="isDirty" class="dirty-dot" title="有未保存修改">●</span>
        </div>
      </div>

      <div class="toolbar-actions">
        <!-- Validation trigger -->
        <button
          class="btn btn-secondary"
          :disabled="validating"
          @click="validateContent(configContent, true)"
          title="调用 dae validate 校验配置文件"
        >
          <ShieldCheck class="icon" :size="15" />
          <span>{{ validating ? '校验中...' : '语法校验' }}</span>
        </button>

        <!-- Import / Export -->
        <input
          ref="fileInput"
          type="file"
          accept=".dae,.conf,.txt"
          class="hidden"
          @change="handleFileSelect"
        />
        <button class="btn btn-secondary" @click="triggerImport" title="从本地上传导入配置">
          <Upload class="icon" :size="15" />
          <span>导入</span>
        </button>

        <button class="btn btn-secondary" @click="exportConfig" title="下载导出当前配置">
          <Download class="icon" :size="15" />
          <span>导出</span>
        </button>

        <button class="btn btn-secondary" @click="openBackupManager" title="查看与还原历史备份">
          <Archive class="icon" :size="15" />
          <span>备份管理</span>
        </button>

        <div class="divider"></div>

        <label class="checkbox-label" title="保存后自动触发 systemctl reload dae">
          <input type="checkbox" v-model="reloadAfterSave" />
          <span>自动重载</span>
        </label>

        <!-- Save Button -->
        <button class="btn btn-primary" :disabled="saving" @click="saveConfig">
          <Save class="icon" :size="15" />
          <span>{{ saving ? '保存中...' : '保存配置' }}</span>
        </button>
      </div>
    </header>

    <!-- Main Workspace Area -->
    <div class="workspace-area">
      <!-- 1. Visual Form Mode -->
      <div v-if="currentMode === 'form'" class="form-mode-container">
        <!-- Sub-navigation tabs for form sections -->
        <div class="form-subnav">
          <button
            class="subnav-item"
            :class="{ active: activeSection === 'nodes' }"
            @click="activeSection = 'nodes'"
          >
            <Server :size="15" />
            <span>节点配置 (node)</span>
            <span class="count-pill">{{ configModel.nodes.length }}</span>
          </button>
          <button
            class="subnav-item"
            :class="{ active: activeSection === 'groups' }"
            @click="activeSection = 'groups'"
          >
            <Layers :size="15" />
            <span>代理组与策略 (group)</span>
            <span class="count-pill">{{ configModel.groups.length }}</span>
          </button>
          <button
            class="subnav-item"
            :class="{ active: activeSection === 'routing' }"
            @click="activeSection = 'routing'"
          >
            <GitFork :size="15" />
            <span>分流规则 (routing)</span>
            <span class="count-pill">{{ configModel.routingRules.length }}</span>
          </button>
          <button
            class="subnav-item"
            :class="{ active: activeSection === 'global' }"
            @click="activeSection = 'global'"
          >
            <Sliders :size="15" />
            <span>全局参数 (global)</span>
          </button>
          <button
            class="subnav-item"
            :class="{ active: activeSection === 'subscriptions' }"
            @click="activeSection = 'subscriptions'"
          >
            <Download :size="15" />
            <span>订阅管理 (subscription)</span>
            <span class="count-pill">{{ configModel.subscriptions.length }}</span>
          </button>
        </div>

        <!-- Section 1: Nodes -->
        <div v-if="activeSection === 'nodes'" class="form-section-body">
          <div class="section-title-bar">
            <div>
              <h3>代理节点配置</h3>
              <p>支持协议链接（VLESS, Trojan, SS, VMess, Hysteria2, SOCKS5 等）快速解析与增删改</p>
            </div>
            <button class="btn btn-primary" @click="openAddNode">
              <Plus :size="14" /> 添加新节点
            </button>
          </div>

          <div class="cards-grid">
            <div v-for="node in configModel.nodes" :key="node.key" class="item-card">
              <div class="card-top">
                <span class="proto-badge">{{ node.protocol || 'NODE' }}</span>
                <span class="card-key">{{ node.key }}</span>
                <div class="card-actions">
                  <button class="btn-icon" @click="openEditNode(node)" title="编辑节点">
                    <Edit2 :size="13" />
                  </button>
                  <button class="btn-icon delete" @click="deleteNode(node.key)" title="删除节点">
                    <Trash2 :size="13" />
                  </button>
                </div>
              </div>
              <div class="card-info">
                <div class="row">
                  <span class="lbl">别名:</span>
                  <span class="val bold truncate" :title="node.name">{{ node.name || node.key }}</span>
                </div>
                <div class="row">
                  <span class="lbl">目标:</span>
                  <span class="val mono truncate" :title="`${node.address}:${node.port}`">{{ node.address }}:{{ node.port }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Section 2: Groups -->
        <div v-else-if="activeSection === 'groups'" class="form-section-body">
          <div class="section-title-bar">
            <div>
              <h3>出站策略与分组 (group)</h3>
              <p>定义各分组策略：<code>min_moving_avg</code>、<code>min_avg10</code>、<code>fixed</code>、<code>random</code> 等</p>
            </div>
            <button class="btn btn-primary" @click="openAddGroup">
              <Plus :size="14" /> 添加策略组
            </button>
          </div>

          <div class="cards-grid">
            <div v-for="group in configModel.groups" :key="group.name" class="item-card group-card">
              <div class="card-top">
                <span class="group-badge">Group</span>
                <span class="card-key">{{ group.name }}</span>
                <div class="card-actions">
                  <button class="btn-icon" @click="openEditGroup(group)" title="编辑策略组">
                    <Edit2 :size="13" />
                  </button>
                  <button class="btn-icon delete" @click="deleteGroup(group.name)" title="删除分组">
                    <Trash2 :size="13" />
                  </button>
                </div>
              </div>
              <div class="card-info">
                <div class="row">
                  <span class="lbl">出站策略:</span>
                  <span class="val code-tag">{{ group.policy || '默认' }}</span>
                </div>
                <div class="filter-list">
                  <span class="lbl">节点过滤:</span>
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

        <!-- Section 3: Routing -->
        <div v-else-if="activeSection === 'routing'" class="form-section-body">
          <div class="section-title-bar">
            <div>
              <h3>分流与路由规则 (routing)</h3>
              <p>匹配流量特征（geosite, geoip, dip, dport, pname 等）并指定动作 (direct, block, 或策略组)</p>
            </div>
            <button class="btn btn-primary" @click="openAddRoute">
              <Plus :size="14" /> 添加路由规则
            </button>
          </div>

          <div class="rule-table-wrap">
            <table class="rule-table">
              <thead>
                <tr>
                  <th style="width: 50px">#</th>
                  <th>匹配条件 (Match Condition)</th>
                  <th style="width: 140px">动作 / 目标出口</th>
                  <th style="width: 90px; text-align: right">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(rule, idx) in configModel.routingRules" :key="idx">
                  <td class="idx-cell">{{ idx + 1 }}</td>
                  <td class="mono cond-cell">
                    <span v-if="rule.condition === 'fallback'" class="fallback-tag">fallback (兜底)</span>
                    <span v-else>{{ rule.condition }}</span>
                  </td>
                  <td>
                    <span class="target-tag" :class="rule.target">{{ rule.target }}</span>
                  </td>
                  <td style="text-align: right">
                    <button class="btn-icon" @click="openEditRoute(rule, idx)"><Edit2 :size="12" /></button>
                    <button class="btn-icon delete" @click="deleteRoute(idx)"><Trash2 :size="12" /></button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Section 4: Global -->
        <div v-else-if="activeSection === 'global'" class="form-section-body">
          <div class="section-title-bar">
            <div>
              <h3>全局网络与代理模式参数 (global)</h3>
              <p>配置内核参数、代理端口、网络接口及连通性探测设置</p>
            </div>
          </div>

          <div class="form-grid">
            <div class="form-group">
              <label>日志等级 (log_level)</label>
              <select v-model="configModel.globals['log_level']" @change="isDirty = true">
                <option value="trace">trace</option>
                <option value="debug">debug</option>
                <option value="info">info</option>
                <option value="warn">warn</option>
                <option value="error">error</option>
              </select>
            </div>

            <div class="form-group">
              <label>代理模式 (dial_mode)</label>
              <select v-model="configModel.globals['dial_mode']" @change="isDirty = true">
                <option value="domain">domain (域名分流 - 推荐)</option>
                <option value="ip">ip (直连 IP 优先)</option>
                <option value="domain+">domain+ (强制域名嗅探)</option>
              </select>
            </div>

            <div class="form-group">
              <label>局域网接口 (lan_interface)</label>
              <input v-model="configModel.globals['lan_interface']" @input="isDirty = true" placeholder="eth0, ens18" />
            </div>

            <div class="form-group">
              <label>广域网接口 (wan_interface)</label>
              <input v-model="configModel.globals['wan_interface']" @input="isDirty = true" placeholder="eth0, ens18" />
            </div>

            <div class="form-group">
              <label>TProxy 端口 (tproxy_port)</label>
              <input v-model="configModel.globals['tproxy_port']" @input="isDirty = true" placeholder="12345" />
            </div>

            <div class="form-group">
              <label>TCP 检测地址 (tcp_check_url)</label>
              <input v-model="configModel.globals['tcp_check_url']" @input="isDirty = true" placeholder="http://cp.cloudflare.com" />
            </div>

            <div class="form-group">
              <label>检测间隔 (check_interval)</label>
              <input v-model="configModel.globals['check_interval']" @input="isDirty = true" placeholder="30s" />
            </div>

            <div class="form-group">
              <label>允许不安全证书 (allow_insecure)</label>
              <select v-model="configModel.globals['allow_insecure']" @change="isDirty = true">
                <option value="false">false (严格校验 - 安全)</option>
                <option value="true">true (允许自签)</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Section 5: Subscriptions -->
        <div v-else-if="activeSection === 'subscriptions'" class="form-section-body">
          <div class="section-title-bar">
            <div>
              <h3>远程订阅配置 (subscription)</h3>
              <p>绑定外部订阅文件，可在策略组中通过 <code>subtag(标签名)</code> 统一过滤节点</p>
            </div>
            <button class="btn btn-primary" @click="openAddSub">
              <Plus :size="14" /> 添加订阅
            </button>
          </div>

          <div class="cards-grid">
            <div v-for="sub in configModel.subscriptions" :key="sub.tag" class="item-card">
              <div class="card-top">
                <span class="sub-badge">Sub</span>
                <span class="card-key">{{ sub.tag }}</span>
                <button class="btn-icon delete" @click="deleteSub(sub.tag)"><Trash2 :size="13" /></button>
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
      </div>

      <!-- 2. Code Editor Mode -->
      <div v-show="currentMode === 'code'" class="editor-wrapper">
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
          <span class="val-msg">尚未校验（点击「语法校验」或「保存配置」）</span>
        </div>
      </div>

      <div class="status-right">
        <span>修改时间: {{ formatDate(lastModTime) }}</span>
        <button class="btn-text" @click="loadConfig" title="重新从磁盘读取">
          <RefreshCw class="icon" :size="12" /> 重新载入
        </button>
      </div>
    </footer>

    <!-- Node Modal -->
    <div v-if="showNodeModal" class="modal-backdrop" @click.self="showNodeModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{{ nodeModalMode === 'add' ? '新增代理节点' : '编辑节点信息' }}</h3>
          <button class="close-btn" @click="showNodeModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>节点唯一标识 Key (如 hk_vless_01)</label>
            <input v-model="editingNode.key" :disabled="nodeModalMode === 'edit'" placeholder="unique_key" />
          </div>
          <div class="field">
            <label>节点链接 URL (支持主流协议分享链接)</label>
            <textarea v-model="editingNode.url" rows="3" placeholder="填写节点完整分享链接 (如 key: scheme://...#Alias)"></textarea>
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showNodeModal = false">取消</button>
            <button class="btn btn-primary" @click="saveNodeModal">确定保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Group Modal -->
    <div v-if="showGroupModal" class="modal-backdrop" @click.self="showGroupModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{{ groupModalMode === 'add' ? '新增代理组' : '编辑代理策略组' }}</h3>
          <button class="close-btn" @click="showGroupModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>分组名称 (如 proxy, streaming, japan)</label>
            <input v-model="editingGroup.name" :disabled="groupModalMode === 'edit'" placeholder="group_name" />
          </div>
          <div class="field">
            <label>出站策略 (policy)</label>
            <select v-model="editingGroup.policy">
              <option value="min_moving_avg">min_moving_avg (最低滑动平均延迟)</option>
              <option value="min_avg10">min_avg10 (近10次最低均值)</option>
              <option value="min">min (最低绝对延迟)</option>
              <option value="random">random (随机负载均衡)</option>
              <option value="fixed(0)">fixed(0) (固定首选节点)</option>
            </select>
          </div>
          <div class="field">
            <label>节点过滤规则 (每行一条 filter)</label>
            <textarea v-model="editingGroup.filtersText" rows="4" placeholder="subtag(mock_sub_a)&#10;name(keyword: 'HK')"></textarea>
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showGroupModal = false">取消</button>
            <button class="btn btn-primary" @click="saveGroupModal">确定保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Route Rule Modal -->
    <div v-if="showRouteModal" class="modal-backdrop" @click.self="showRouteModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>{{ routeModalMode === 'add' ? '新增路由分流规则' : '编辑分流规则' }}</h3>
          <button class="close-btn" @click="showRouteModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>匹配条件 (Condition)</label>
            <input v-model="editingRoute.condition" placeholder="domain(geosite:google) 或 dip(geoip:cn)" />
          </div>
          <div class="field">
            <label>目标动作 (Target)</label>
            <input v-model="editingRoute.target" placeholder="direct, block, proxy 或策略组名" />
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showRouteModal = false">取消</button>
            <button class="btn btn-primary" @click="saveRouteModal">确定保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Sub Modal -->
    <div v-if="showSubModal" class="modal-backdrop" @click.self="showSubModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>新增订阅</h3>
          <button class="close-btn" @click="showSubModal = false">&times;</button>
        </div>
        <div class="modal-body form-modal">
          <div class="field">
            <label>订阅标识 Tag (如 sub_a)</label>
            <input v-model="editingSub.tag" placeholder="sub_tag" />
          </div>
          <div class="field">
            <label>订阅 URL</label>
            <input v-model="editingSub.url" placeholder="https://example.com/subscribe" />
          </div>
          <div class="modal-actions">
            <button class="btn btn-secondary" @click="showSubModal = false">取消</button>
            <button class="btn btn-primary" @click="saveSubModal">保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Backup Modal -->
    <div v-if="showBackupModal" class="modal-backdrop" @click.self="showBackupModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3><Archive class="icon" :size="18" /> 配置文件版本备份</h3>
          <button class="close-btn" @click="showBackupModal = false">&times;</button>
        </div>
        <div class="modal-body">
          <div class="create-backup-bar">
            <input
              type="text"
              v-model="newBackupLabel"
              placeholder="备份备注标签 (例如: 改动前备份, v1.2)"
              @keydown.enter="createManualBackup"
            />
            <button class="btn btn-primary" @click="createManualBackup">创建立即备份</button>
          </div>

          <div class="backup-list-box">
            <div v-if="backups.length === 0" class="empty-state">暂无历史备份版本</div>
            <table v-else class="backup-table">
              <thead>
                <tr>
                  <th>备份文件名</th>
                  <th>时间</th>
                  <th>大小</th>
                  <th style="text-align: right">操作</th>
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
  </div>
</template>

<style scoped>
.config-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #090d16;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: #0f172a;
  border-bottom: 1px solid #1e293b;
  gap: 12px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mode-switcher {
  display: flex;
  background: #090d16;
  border: 1px solid #1e293b;
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
  color: #94a3b8;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-btn.active {
  background: #1e293b;
  color: #38bdf8;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.file-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  background: #1e293b;
  padding: 4px 10px;
  border-radius: 6px;
  font-weight: 500;
  color: #38bdf8;
  font-size: 13px;
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
  background: #334155;
  margin: 0 4px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #94a3b8;
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
  background: #2563eb;
  color: #fff;
}
.btn-primary:hover:not(:disabled) {
  background: #1d4ed8;
}

.btn-secondary {
  background: #1e293b;
  border-color: #334155;
  color: #cbd5e1;
}
.btn-secondary:hover:not(:disabled) {
  background: #334155;
  color: #fff;
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
}

/* Form Mode Layout */
.form-mode-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #090d16;
}

.form-subnav {
  display: flex;
  gap: 4px;
  padding: 8px 16px;
  background: #0b1120;
  border-bottom: 1px solid #1e293b;
}

.subnav-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 6px;
  color: #94a3b8;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.subnav-item:hover {
  color: #f1f5f9;
  background: #1e293b66;
}

.subnav-item.active {
  background: #1e293b;
  color: #38bdf8;
  border-color: #334155;
}

.count-pill {
  background: #090d16;
  color: #94a3b8;
  padding: 1px 6px;
  border-radius: 10px;
  font-size: 11px;
}

.form-section-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-title-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-title-bar h3 {
  margin: 0;
  font-size: 16px;
  color: #f1f5f9;
}

.section-title-bar p {
  margin: 4px 0 0;
  font-size: 12px;
  color: #64748b;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.item-card {
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 8px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
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
  color: #38bdf8;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}

.group-badge {
  font-size: 10px;
  font-weight: 600;
  background: #7e22ce22;
  color: #c084fc;
  padding: 2px 6px;
  border-radius: 4px;
}

.sub-badge {
  font-size: 10px;
  font-weight: 600;
  background: #16a34a22;
  color: #4ade80;
  padding: 2px 6px;
  border-radius: 4px;
}

.card-key {
  font-size: 12px;
  font-weight: 600;
  color: #e2e8f0;
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
  color: #94a3b8;
}

.card-info .row {
  display: flex;
  gap: 6px;
}

.card-info .lbl {
  color: #64748b;
  width: 36px;
  flex-shrink: 0;
}

.card-info .val {
  color: #cbd5e1;
}

.code-tag {
  background: #1e293b;
  color: #38bdf8;
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
  background: #1e293b;
  color: #cbd5e1;
  font-size: 11px;
  font-family: monospace;
  padding: 2px 6px;
  border-radius: 4px;
}

/* Rule table */
.rule-table-wrap {
  background: #0f172a;
  border: 1px solid #1e293b;
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
  background: #0b1120;
  color: #64748b;
  border-bottom: 1px solid #1e293b;
}

.rule-table td {
  padding: 10px 14px;
  border-bottom: 1px solid #1e293b55;
  color: #cbd5e1;
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
  background: #1e293b;
  color: #38bdf8;
}
.target-tag.direct {
  background: #064e3b33;
  color: #34d399;
}
.target-tag.block {
  background: #7f1d1d33;
  color: #f87171;
}

/* Form grid */
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  background: #0f172a;
  padding: 20px;
  border-radius: 8px;
  border: 1px solid #1e293b;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  color: #94a3b8;
  font-weight: 500;
}

.form-group input, .form-group select {
  background: #1e293b;
  border: 1px solid #334155;
  color: #f1f5f9;
  padding: 7px 10px;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
}

.form-group input:focus, .form-group select:focus {
  border-color: #38bdf8;
}

/* Editor Wrapper */
.editor-wrapper {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.editor-host {
  height: 100%;
}

/* Status Bar */
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px;
  background: #090d16;
  border-top: 1px solid #1e293b;
  font-size: 12px;
  color: #64748b;
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
  background: #064e3b44;
  color: #34d399;
}

.val-indicator.invalid {
  background: #7f1d1d44;
  color: #f87171;
}

.val-indicator.neutral {
  color: #64748b;
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
  color: #94a3b8;
  cursor: pointer;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0;
}
.btn-text:hover {
  color: #38bdf8;
}

/* Modals */
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
  width: 580px;
  max-height: 85vh;
  background: #0f172a;
  border: 1px solid #1e293b;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid #1e293b;
}

.modal-header h3 {
  margin: 0;
  font-size: 15px;
  color: #e2e8f0;
}

.close-btn {
  background: none;
  border: none;
  color: #94a3b8;
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
  color: #94a3b8;
}

.form-modal input, .form-modal textarea, .form-modal select {
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 6px;
  padding: 8px 12px;
  color: #f1f5f9;
  font-size: 13px;
  outline: none;
}

.form-modal input:focus, .form-modal textarea:focus, .form-modal select:focus {
  border-color: #38bdf8;
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
  background: #1e293b;
  border: 1px solid #334155;
  border-radius: 6px;
  padding: 6px 12px;
  color: #f1f5f9;
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
  color: #64748b;
  border-bottom: 1px solid #1e293b;
}

.backup-table td {
  padding: 10px 8px;
  border-bottom: 1px solid #1e293b77;
  color: #cbd5e1;
}

.name-cell {
  font-family: monospace;
  color: #38bdf8;
}

.btn-icon {
  background: #1e293b;
  border: 1px solid #334155;
  color: #94a3b8;
  border-radius: 4px;
  padding: 4px 6px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}
.btn-icon:hover {
  color: #fff;
  background: #334155;
}
.btn-icon.restore:hover {
  color: #38bdf8;
  border-color: #38bdf8;
}
.btn-icon.delete:hover {
  color: #f87171;
  border-color: #f87171;
}

.empty-state {
  text-align: center;
  padding: 30px;
  color: #64748b;
  font-size: 13px;
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
</style>
