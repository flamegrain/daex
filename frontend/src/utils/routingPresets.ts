export interface RoutingPreset {
  id: string
  name: string
  category: 'common' | 'security' | 'media' | 'dev' | 'advanced'
  condition: string
  recommendedTarget: string
  description: string
  tip?: string
}

export const ROUTING_PRESETS: RoutingPreset[] = [
  {
    id: 'lan-private',
    name: '局域网私网直连 (推荐置顶)',
    category: 'common',
    condition: 'dip(geoip:private)',
    recommendedTarget: 'direct',
    description: '局域网互访流量直接通行，防止误走代理导致内网设备或路由器后台无法访问',
    tip: '建议放在路由规则的第一条！'
  },
  {
    id: 'cn-domains',
    name: '中国大陆常用域名直连',
    category: 'common',
    condition: 'domain(geosite:cn)',
    recommendedTarget: 'direct',
    description: '匹配国内主流互联网网站与服务域名，直接走本地网络提高访问速度'
  },
  {
    id: 'cn-ip',
    name: '中国大陆目标 IP 直连',
    category: 'common',
    condition: 'dip(geoip:cn)',
    recommendedTarget: 'direct',
    description: '通过 GeoIP 规则库匹配所有位于中国大陆的 IPv4/IPv6 地址直接放行'
  },
  {
    id: 'block-ads',
    name: '全网广告与隐私追踪拦截',
    category: 'security',
    condition: 'domain(geosite:category-ads-all)',
    recommendedTarget: 'block',
    description: '拦截已知的广告、追踪统计、弹窗及恶意域名，节省带宽并保护隐私'
  },
  {
    id: 'upstream-dns',
    name: '上游 DNS 流量防回环直连',
    category: 'security',
    condition: 'dport(53)',
    recommendedTarget: 'must_direct',
    description: '强制放行 DNS 53 端口流量，免除本地 DNS 劫持以防形成递归解析环路',
    tip: '推荐选择 must_direct 强制直连'
  },
  {
    id: 'google-youtube',
    name: 'Google & YouTube 服务分流',
    category: 'media',
    condition: 'domain(geosite:google, geosite:youtube)',
    recommendedTarget: 'proxy',
    description: '匹配 Google 全系服务与 YouTube 视频流走代理出站'
  },
  {
    id: 'telegram',
    name: 'Telegram 社交应用分流',
    category: 'common',
    condition: 'domain(geosite:telegram) || dip(geoip:telegram)',
    recommendedTarget: 'proxy',
    description: '匹配 Telegram 域名与官方数据中心 IP 段走代理'
  },
  {
    id: 'github-dev',
    name: 'GitHub 与常用开源代码托管',
    category: 'dev',
    condition: 'domain(geosite:github, geosite:gitlab)',
    recommendedTarget: 'proxy',
    description: '加速代码拉取、Git 克隆与 Releases 下载'
  },
  {
    id: 'openai-ai',
    name: 'OpenAI & Claude & AI 常用工具',
    category: 'dev',
    condition: 'domain(geosite:openai, geosite:anthropic)',
    recommendedTarget: 'proxy',
    description: '精准分流主流人工智能大模型 API 与网页服务'
  },
  {
    id: 'streaming-media',
    name: '海外流媒体 (Netflix / Disney+ / Spotify)',
    category: 'media',
    condition: 'domain(geosite:netflix, geosite:disney, geosite:spotify)',
    recommendedTarget: 'proxy',
    description: '定向分流至支持解锁流媒体节点的代理策略组'
  },
  {
    id: 'web-ports',
    name: 'HTTP / HTTPS 常用 Web 端口',
    category: 'advanced',
    condition: 'dport(80, 443)',
    recommendedTarget: 'proxy',
    description: '仅分流标准 Web 流量，其他未知协议可保持直连或自定义'
  },
  {
    id: 'fallback-rule',
    name: '全局兜底规则 (必须置于末尾)',
    category: 'common',
    condition: 'fallback',
    recommendedTarget: 'proxy',
    description: '未命中前面任何规则的所有剩余流量的统一出站出口',
    tip: '必须放在所有路由规则的最后一行！'
  }
]

export interface SyntaxDocSection {
  title: string
  summary: string
  syntax: string
  examples: { code: string; explain: string }[]
}

export const ROUTING_SYNTAX_GUIDE: SyntaxDocSection[] = [
  {
    title: 'domain(...) 域名匹配规则',
    summary: '支持按 geosite 规则库、域名后缀、关键字、完整匹配或正则表达式匹配目标域名。',
    syntax: 'domain( [type:]pattern [, ...] )',
    examples: [
      { code: 'domain(geosite:cn)', explain: '匹配 geosite.dat 中收录的所有中国大陆域名' },
      { code: 'domain(geosite:google, geosite:youtube)', explain: '支持逗号分隔匹配多个 geosite 列表' },
      { code: 'domain(suffix: google.com)', explain: '后缀匹配，匹配以 .google.com 结尾的所有域名' },
      { code: 'domain(keyword: bilibili)', explain: '关键字匹配，域名包含 bilibili 即命中' },
      { code: 'domain(full: api.openai.com)', explain: '全域名精准匹配，不包含子域名' },
      { code: 'domain(regex: \'^.*\\.apple\\.com$\')', explain: '正则表达式匹配特定格式域名' }
    ]
  },
  {
    title: 'dip(...) 目标 IP (Destination IP) 匹配',
    summary: '匹配流量的目的 IP 地址，支持 geoip 规则库、单 IP 或 CIDR 网段。',
    syntax: 'dip( [geoip:tag | ip/cidr] [, ...] )',
    examples: [
      { code: 'dip(geoip:private)', explain: '匹配私有局域网 IP (10/8, 172.16/12, 192.168/16 等)' },
      { code: 'dip(geoip:cn)', explain: '匹配 geoip.dat 中收录的中国大陆公网 IP 段' },
      { code: 'dip(223.5.5.5, 119.29.29.29)', explain: '匹配特定上游 DNS IP 地址' },
      { code: 'dip(192.168.1.0/24, 10.0.0.0/8)', explain: '匹配指定 CIDR 网段' }
    ]
  },
  {
    title: 'dport(...) 与 sport(...) 端口匹配',
    summary: '匹配目标端口 (dport) 或源端口 (sport)，支持单端口、多端口或端口范围。',
    syntax: 'dport(port[, port, ... | start:end])',
    examples: [
      { code: 'dport(80, 443)', explain: '匹配常见的 HTTP / HTTPS 网页服务端口' },
      { code: 'dport(53)', explain: '匹配 DNS 域名解析端口' },
      { code: 'dport(22)', explain: '匹配 SSH 远程连接端口' },
      { code: 'dport(10000:20000)', explain: '匹配 10000 到 20000 范围内的端口' }
    ]
  },
  {
    title: 'sip(...) 客户端源 IP 与 mac(...) 物理地址',
    summary: '在网关部署模式下，根据内网不同客户端设备、IP 网段或 MAC 地址进行分流。',
    syntax: 'sip(ip[/cidr]) / mac(\'xx:xx:xx:xx:xx:xx\')',
    examples: [
      { code: 'sip(192.168.1.100)', explain: '针对内网某台特定电脑/电视盒子单独配置分流' },
      { code: 'sip(192.168.1.0/24)', explain: '针对某个特定 VLAN 或内网子网进行分流' },
      { code: 'mac(\'00:11:22:33:44:55\')', explain: '按客户端物理网卡 MAC 地址精准识别设备' }
    ]
  },
  {
    title: 'l4proto(...) 传输协议与 pname(...) 进程',
    summary: '根据传输层协议 (TCP/UDP) 或 dae 宿主机发起的本地进程名称进行分流。',
    syntax: 'l4proto(tcp | udp) / pname(process_name[, ...])',
    examples: [
      { code: 'l4proto(tcp)', explain: '仅匹配 TCP 连接' },
      { code: 'l4proto(udp)', explain: '仅匹配 UDP 数据报文' },
      { code: 'pname(curl, wget)', explain: '仅匹配由 curl 或 wget 命令行发起的网络连接' },
      { code: 'pname(chrome, telegram)', explain: '匹配本机 Chrome 浏览器或 Telegram 客户端发出的流量' }
    ]
  },
  {
    title: '逻辑组合 (&&) 与取反 (!)',
    summary: '可以在一条规则中组合多个条件，条件之间使用 && 连接，支持在条件前使用 ! 取反。',
    syntax: 'condition1 && condition2 && !condition3',
    examples: [
      { code: 'domain(geosite:cn) && dport(443)', explain: '访问国内域名且为 HTTPS 443 端口的流量' },
      { code: 'dip(geoip:cn) && !dport(80, 443)', explain: '目的为国内 IP 但不是标准 Web 端口的流量' },
      { code: 'l4proto(udp) && !dport(53, 443)', explain: '排除 DNS 和 QUIC 的其他 UDP 流量' }
    ]
  },
  {
    title: '目标动作 (Target) 说明',
    summary: '匹配成功后，流量将被导向指定的内置动作或出站策略组。',
    syntax: 'direct | must_direct | block | <策略组名>',
    examples: [
      { code: 'direct', explain: '常规直连，绕过代理节点由网关直接路由出站' },
      { code: 'must_direct', explain: '强制直连，同时豁免 dae 本地 DNS 嗅探劫持（常用于上游 DNS）' },
      { code: 'block', explain: '直接阻断并丢弃连接（常用于广告与恶意域名拦截）' },
      { code: 'my_proxy_group', explain: '将流量交由在 group {} 块中定义的出站代理策略组' }
    ]
  },
  {
    title: 'fallback: 兜底出口机制',
    summary: '自上而下匹配未命中任何规则时的最终出口，在整个 routing 块中有且只能有一个，且必须位于最后一行。',
    syntax: 'fallback: <target>',
    examples: [
      { code: 'fallback: proxy', explain: '前面规则均未匹配时，默认走名为 proxy 的代理组' },
      { code: 'fallback: direct', explain: '白名单模式：仅特定规则走代理，其余未知流量全部默认直连' }
    ]
  }
]
