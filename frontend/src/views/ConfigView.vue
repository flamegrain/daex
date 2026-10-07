<script setup lang="ts">
import { ref, onMounted, shallowRef } from 'vue'
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
  ExternalLink
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

const editorContainer = ref<HTMLDivElement | null>(null)
const editorView = shallowRef<EditorView | null>(null)

const configContent = ref('')
const configPath = ref('')
const lastModTime = ref('')
const isDirty = ref(false)
const loading = ref(false)
const saving = ref(false)
const validating = ref(false)

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
        '.cm-content': {
          padding: '12px 0',
        },
        '.cm-line': {
          padding: '0 16px',
        },
        '.cm-gutters': {
          backgroundColor: '#090d16',
          color: '#475569',
          borderRight: '1px solid #1e293b',
        },
        '.cm-activeLine': {
          backgroundColor: '#1e293b55',
        },
        '.cm-activeLineGutter': {
          backgroundColor: '#1e293b',
          color: '#94a3b8',
        },
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
    const res = await fetch('/api/config')
    const data = await res.json()
    if (res.ok) {
      configPath.value = data.path
      lastModTime.value = data.modTime
      setEditorText(data.content)
      validateCurrentConfig(false)
    } else {
      alert('加载配置失败: ' + (data.error || res.statusText))
    }
  } catch (err: any) {
    alert('请求错误: ' + err.message)
  } finally {
    loading.value = false
  }
}

async function validateCurrentConfig(showAlert = true) {
  validating.value = true
  try {
    const res = await fetch('/api/config/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: configContent.value })
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
      if (confirm(`准备导入文件: ${file.name}，是否载入编辑器？`)) {
        setEditorText(content)
        isDirty.value = true
        validateCurrentConfig(true)
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
    if (res.ok) {
      backups.value = data.backups || []
    }
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
  if (!confirm(`确认回滚恢复此备份？\n[${name}]\n恢复后当前未保存的编辑将被替换。`)) {
    return
  }
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
      alert('恢复失败: ' + (data.error || '') + '\n' + (data.details || ''))
    }
  } catch (e: any) {
    alert('请求失败: ' + e.message)
  }
}

async function deleteBackup(name: string) {
  if (!confirm(`确定删除备份文件 [${name}] 吗？`)) return
  try {
    const res = await fetch(`/api/backups/${encodeURIComponent(name)}`, {
      method: 'DELETE'
    })
    if (res.ok) {
      await fetchBackups()
    }
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
        <div class="file-badge">
          <FileCode class="icon" :size="16" />
          <span class="file-name">{{ configPath ? configPath.split('/').pop() : 'config.mock.dae' }}</span>
          <span v-if="isDirty" class="dirty-dot" title="有未保存修改">●</span>
        </div>
        <span class="file-desc">使用 <code>config.mock.dae</code> 隔离开发调试</span>
      </div>

      <div class="toolbar-actions">
        <!-- Validation trigger -->
        <button
          class="btn btn-secondary"
          :disabled="validating"
          @click="validateCurrentConfig(true)"
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

        <!-- Backups -->
        <button class="btn btn-secondary" @click="openBackupManager" title="查看与还原历史备份">
          <Archive class="icon" :size="15" />
          <span>备份管理</span>
        </button>

        <div class="divider"></div>

        <!-- Reload option -->
        <label class="checkbox-label" title="保存后自动触发 systemctl reload dae">
          <input type="checkbox" v-model="reloadAfterSave" />
          <span>自动重载</span>
        </label>

        <!-- Save Button -->
        <button
          class="btn btn-primary"
          :disabled="saving"
          @click="saveConfig"
        >
          <Save class="icon" :size="15" />
          <span>{{ saving ? '保存中...' : '保存配置' }}</span>
        </button>
      </div>
    </header>

    <!-- Editor and Status Bar Area -->
    <div class="editor-wrapper">
      <div ref="editorContainer" class="editor-host"></div>
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
          <span class="val-msg">尚未校验（按 Ctrl/Cmd+S 或点击「语法校验」）</span>
        </div>
      </div>

      <div class="status-right">
        <span>修改时间: {{ formatDate(lastModTime) }}</span>
        <button class="btn-text" @click="loadConfig" title="重新从磁盘读取">
          <RefreshCw class="icon" :size="12" /> 重新载入
        </button>
      </div>
    </footer>

    <!-- Backup Modal -->
    <div v-if="showBackupModal" class="modal-backdrop" @click.self="showBackupModal = false">
      <div class="modal-card">
        <div class="modal-header">
          <h3>
            <Archive class="icon" :size="18" />
            配置文件版本备份
          </h3>
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
            <div v-if="backups.length === 0" class="empty-state">
              暂无历史备份版本
            </div>
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
                    <a
                      :href="`/api/backups/${encodeURIComponent(b.name)}/download`"
                      target="_blank"
                      class="btn-icon"
                      title="下载此备份"
                    >
                      <Download :size="14" />
                    </a>
                    <button
                      class="btn-icon restore"
                      @click="restoreBackup(b.name)"
                      title="回滚/恢复到此配置"
                    >
                      <RotateCcw :size="14" />
                    </button>
                    <button
                      class="btn-icon delete"
                      @click="deleteBackup(b.name)"
                      title="删除此备份"
                    >
                      <Trash2 :size="14" />
                    </button>
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
  padding: 10px 16px;
  background: #0f172a;
  border-bottom: 1px solid #1e293b;
  gap: 12px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
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

.file-desc {
  font-size: 12px;
  color: #64748b;
}

.file-desc code {
  color: #94a3b8;
  background: #1e293b;
  padding: 1px 4px;
  border-radius: 4px;
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

.editor-wrapper {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.editor-host {
  height: 100%;
}

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

/* Modal */
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
  width: 640px;
  max-height: 80vh;
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
  display: flex;
  align-items: center;
  gap: 8px;
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

.create-backup-bar input:focus {
  border-color: #38bdf8;
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

.action-cell {
  text-align: right;
  white-space: nowrap;
}

.btn-icon {
  background: #1e293b;
  border: 1px solid #334155;
  color: #94a3b8;
  border-radius: 4px;
  padding: 4px 6px;
  margin-left: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  text-decoration: none;
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

.hidden {
  display: none;
}
</style>
