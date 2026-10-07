import { ref, watch } from 'vue'

export type ThemeMode = 'dark' | 'light'
export type LangType = 'zh-CN' | 'en-US'

const savedTheme = (localStorage.getItem('dae_webui_theme') as ThemeMode) || 'dark'
const savedLang = (localStorage.getItem('dae_webui_lang') as LangType) || 'zh-CN'

export const theme = ref<ThemeMode>(savedTheme)
export const lang = ref<LangType>(savedLang)

// Apply theme to document element
export function applyTheme(t: ThemeMode) {
  theme.value = t
  document.documentElement.setAttribute('data-theme', t)
  localStorage.setItem('dae_webui_theme', t)
}

// Apply lang
export function setLang(l: LangType) {
  lang.value = l
  localStorage.setItem('dae_webui_lang', l)
}

// Translations dictionary
const messages: Record<LangType, Record<string, string>> = {
  'zh-CN': {
    'brand.title': 'dae 控制台',
    'brand.sub': 'eBPF 透明网关面板',
    'nav.section.overview': '概览分析',
    'nav.section.core': '核心功能',
    'nav.section.system': '系统状态',
    'nav.home': '首页大盘',
    'nav.config': '配置管理',
    'nav.nodes': '节点与策略',
    'nav.logs': '实时日志',
    'nav.service': '服务状态',
    'nav.settings': '系统设置',
    'status.active': '运行中',
    'status.inactive': '未启动',
    'status.version': '版本',
    'status.env': '环境',
    'status.env.mock': '安全假数据',
    'settings.title': '系统设置',
    'settings.subtitle': '管理界面偏好、语言主题与 WebUI 用户凭据',
    'settings.theme': '界面主题',
    'settings.theme.dark': '深色模式',
    'settings.theme.light': '浅色模式',
    'settings.lang': '系统语言',
    'settings.user.title': 'WebUI 登录用户信息',
    'settings.user.notice': '说明：此账号仅用于 WebUI 网页管理控制台登录认证，与 Linux 底层系统的 root 用户完全独立。',
    'settings.user.username': '用户名',
    'settings.user.role': '账户角色',
    'settings.user.role.admin': 'WebUI 管理员',
    'settings.user.old_pwd': '原登录密码',
    'settings.user.new_pwd': '新登录密码',
    'settings.user.confirm_pwd': '确认新密码',
    'settings.user.save': '保存修改',
    'settings.about.title': '系统关于'
  },
  'en-US': {
    'brand.title': 'dae WebUI',
    'brand.sub': 'eBPF Transparent Proxy',
    'nav.section.overview': 'Overview',
    'nav.section.core': 'Management',
    'nav.section.system': 'System',
    'nav.home': 'Dashboard',
    'nav.config': 'Config',
    'nav.nodes': 'Nodes & Groups',
    'nav.logs': 'Live Logs',
    'nav.service': 'Service Status',
    'nav.settings': 'Settings',
    'status.active': 'Active',
    'status.inactive': 'Inactive',
    'status.version': 'Version',
    'status.env': 'Env',
    'status.env.mock': 'Mock Data',
    'settings.title': 'System Settings',
    'settings.subtitle': 'Manage UI preferences, language, themes, and WebUI user credentials',
    'settings.theme': 'Theme Mode',
    'settings.theme.dark': 'Dark Mode',
    'settings.theme.light': 'Light Mode',
    'settings.lang': 'Language',
    'settings.user.title': 'WebUI User Account',
    'settings.user.notice': 'Notice: This account is only for logging into the WebUI dashboard. It is completely independent of the Linux system root account.',
    'settings.user.username': 'Username',
    'settings.user.role': 'Account Role',
    'settings.user.role.admin': 'WebUI Administrator',
    'settings.user.old_pwd': 'Current Password',
    'settings.user.new_pwd': 'New Password',
    'settings.user.confirm_pwd': 'Confirm New Password',
    'settings.user.save': 'Save Changes',
    'settings.about.title': 'About System'
  }
}

export function t(key: string): string {
  const currentLang = lang.value
  return messages[currentLang]?.[key] || messages['zh-CN'][key] || key
}

// Initial apply
applyTheme(theme.value)
