export interface ConfigRouteRule {
  condition: string
  target: string
}

export type ConflictSeverity = 'error' | 'warning' | 'info'

export interface RoutingConflict {
  id: string
  severity: ConflictSeverity
  ruleIndices: number[] // 涉及的规则索引 (0-based)
  title: string
  message: string
  suggestion: string
}

export interface RoutingHealthReport {
  totalRules: number
  hasErrors: boolean
  hasWarnings: boolean
  conflicts: RoutingConflict[]
  ruleStatusMap: Record<number, { severity: ConflictSeverity; count: number; messages: string[] }>
}

/**
 * 结构化解析 dae condition 字符串
 */
export interface ParsedConditionItem {
  negated: boolean // 是否有 !
  type: 'domain' | 'dip' | 'sip' | 'dport' | 'sport' | 'l4proto' | 'ipversion' | 'pname' | 'mac' | 'dscp' | 'other'
  rawSub: string
  values: string[] // 括号内的参数列表 (以逗号分隔清理后的值)
  isBroadDomain?: boolean // 是否属于宽泛域名如 geosite:cn 或泛域名
  isBroadDip?: boolean // 是否属于宽泛 IP 如 geoip:cn 或大网段
}

export interface ParsedRouteRule {
  index: number
  rawCondition: string
  target: string
  isFallback: boolean
  items: ParsedConditionItem[]
}

export function parseCondition(condStr: string): ParsedConditionItem[] {
  const trimmed = condStr.trim()
  if (!trimmed || trimmed === 'fallback') return []

  // 按 && 拆分各个子条件
  const parts = trimmed.split('&&').map(p => p.trim()).filter(Boolean)
  const items: ParsedConditionItem[] = []

  for (const part of parts) {
    let raw = part
    const negated = raw.startsWith('!')
    if (negated) {
      raw = raw.slice(1).trim()
    }

    // 匹配 func(args)
    const match = raw.match(/^([a-zA-Z0-9_]+)\((.*)\)$/)
    if (!match) {
      items.push({
        negated,
        type: 'other',
        rawSub: raw,
        values: [raw]
      })
      continue
    }

    const funcName = match[1].toLowerCase()
    const innerArgs = match[2].split(',').map(s => s.trim().replace(/^['"]|['"]$/g, '')).filter(Boolean)

    let type: ParsedConditionItem['type'] = 'other'
    let isBroadDomain = false
    let isBroadDip = false

    if (funcName === 'domain') {
      type = 'domain'
      // 判断是否宽泛
      for (const val of innerArgs) {
        if (val.startsWith('geosite:cn') || val === 'cn' || !val.includes('.')) {
          isBroadDomain = true
        }
      }
    } else if (funcName === 'dip') {
      type = 'dip'
      for (const val of innerArgs) {
        if (val.startsWith('geoip:cn') || val === '0.0.0.0/0') {
          isBroadDip = true
        }
      }
    } else if (funcName === 'sip') {
      type = 'sip'
    } else if (funcName === 'dport') {
      type = 'dport'
    } else if (funcName === 'sport') {
      type = 'sport'
    } else if (funcName === 'l4proto') {
      type = 'l4proto'
    } else if (funcName === 'ipversion') {
      type = 'ipversion'
    } else if (funcName === 'pname') {
      type = 'pname'
    } else if (funcName === 'mac') {
      type = 'mac'
    } else if (funcName === 'dscp') {
      type = 'dscp'
    }

    items.push({
      negated,
      type,
      rawSub: raw,
      values: innerArgs,
      isBroadDomain,
      isBroadDip
    })
  }

  return items
}

/**
 * 诊断整个分流路由列表中的冲突、阴影死规则、fallback 顺序等
 */
export function analyzeRoutingConflicts(
  rules: ConfigRouteRule[],
  availableGroups: string[] = []
): RoutingHealthReport {
  const conflicts: RoutingConflict[] = []
  const ruleStatusMap: Record<number, { severity: ConflictSeverity; count: number; messages: string[] }> = {}

  function markRule(idx: number, sev: ConflictSeverity, msg: string) {
    if (!ruleStatusMap[idx]) {
      ruleStatusMap[idx] = { severity: sev, count: 0, messages: [] }
    }
    const cur = ruleStatusMap[idx]
    if (sev === 'error') cur.severity = 'error'
    else if (sev === 'warning' && cur.severity !== 'error') cur.severity = 'warning'
    cur.count++
    cur.messages.push(msg)
  }

  // 解析所有规则
  const parsedRules: ParsedRouteRule[] = rules.map((r, idx) => ({
    index: idx,
    rawCondition: r.condition.trim(),
    target: r.target.trim(),
    isFallback: r.condition.trim() === 'fallback',
    items: parseCondition(r.condition)
  }))

  const groupSet = new Set(availableGroups)
  const builtinTargets = new Set(['direct', 'must_direct', 'block'])

  // 1. Fallback 规则检查
  const fallbackIndices = parsedRules
    .filter(r => r.isFallback)
    .map(r => r.index)

  if (fallbackIndices.length > 1) {
    conflicts.push({
      id: 'multiple-fallback',
      severity: 'error',
      ruleIndices: fallbackIndices,
      title: '存在多个 Fallback 兜底规则',
      message: `检测到第 ${fallbackIndices.map(i => i + 1).join(', ')} 行均为 fallback 规则。dae 只允许一个兜底出口。`,
      suggestion: '请删除多余的 fallback 规则，仅保留最后一条兜底出口。'
    })
    fallbackIndices.forEach(i => markRule(i, 'error', '重复的 fallback 兜底规则'))
  }

  if (fallbackIndices.length >= 1) {
    const firstFallbackIdx = fallbackIndices[0]
    if (firstFallbackIdx < rules.length - 1) {
      // fallback 后面还有其他规则，这些规则将永远无法被匹配！
      const deadIndices: number[] = []
      for (let i = firstFallbackIdx + 1; i < rules.length; i++) {
        deadIndices.push(i)
        markRule(i, 'error', `位于 fallback 兜底之后，永远无法被执行 (死规则)`)
      }
      conflicts.push({
        id: 'fallback-not-last',
        severity: 'error',
        ruleIndices: [firstFallbackIdx, ...deadIndices],
        title: 'Fallback 兜底规则未置于末尾 (产生死规则)',
        message: `第 ${firstFallbackIdx + 1} 行配置了 fallback 兜底出口，但其下方仍有 ${deadIndices.length} 条分流规则 (第 ${deadIndices.map(i => i + 1).join(', ')} 行)。根据 dae 规范，fallback 命中后立即终止，其后的所有规则将永远不会被匹配！`,
        suggestion: '请使用下方列表的排序按钮或直接拖动，将 fallback 规则移动到规则列表的最底端。'
      })
      markRule(firstFallbackIdx, 'error', 'Fallback 未置于列表末尾')
    }
  }

  // 2. 检查规则有效性与未知策略组
  parsedRules.forEach(r => {
    if (r.target && !builtinTargets.has(r.target) && groupSet.size > 0 && !groupSet.has(r.target)) {
      conflicts.push({
        id: `unknown-group-${r.index}`,
        severity: 'warning',
        ruleIndices: [r.index],
        title: `引用了未定义的策略组 "${r.target}"`,
        message: `第 ${r.index + 1} 行分流规则指定了策略组 "${r.target}"，但当前配置的策略组列表中未定义该组。`,
        suggestion: `请确认策略组拼写是否正确，或在「策略分组」选项卡中新增名为 "${r.target}" 的出站策略组。`
      })
      markRule(r.index, 'warning', `策略组 "${r.target}" 未定义`)
    }
  })

  // 3. 检查完全重复的规则 & 阴影规则 (Shadowed Rules / First Match Wins)
  for (let i = 0; i < parsedRules.length; i++) {
    const ruleA = parsedRules[i]
    if (ruleA.isFallback) continue

    for (let j = i + 1; j < parsedRules.length; j++) {
      const ruleB = parsedRules[j]
      if (ruleB.isFallback) continue

      // (a) 完全相同的条件
      if (ruleA.rawCondition === ruleB.rawCondition) {
        if (ruleA.target === ruleB.target) {
          conflicts.push({
            id: `duplicate-${i}-${j}`,
            severity: 'warning',
            ruleIndices: [i, j],
            title: `完全重复的分流规则 (#${i + 1} 与 #${j + 1})`,
            message: `第 ${i + 1} 行与第 ${j + 1} 行的匹配条件和目标出口完全一致 ("${ruleA.rawCondition} -> ${ruleA.target}")，第 ${j + 1} 行是冗余规则。`,
            suggestion: `建议删除第 ${j + 1} 行的重复规则以精简配置。`
          })
          markRule(j, 'warning', `与第 ${i + 1} 行完全重复`)
        } else {
          conflicts.push({
            id: `contradiction-${i}-${j}`,
            severity: 'error',
            ruleIndices: [i, j],
            title: `条件相同但动作冲突 (#${i + 1} 与 #${j + 1})`,
            message: `第 ${i + 1} 行与第 ${j + 1} 行条件完全相同 ("${ruleA.rawCondition}")，但前者路由至 "${ruleA.target}"，后者路由至 "${ruleB.target}"。根据首条命中原则，第 ${j + 1} 行永远无法生效！`,
            suggestion: `请删除第 ${j + 1} 行或修改其匹配条件以避免死规则。`
          })
          markRule(j, 'error', `被第 ${i + 1} 行遮蔽，永远无法生效`)
        }
        continue
      }

      // (b) 阴影规则分析：若 ruleA 是宽泛规则，而 ruleB 是 ruleA 加上了更细的条件 (如 ruleA 是 domain(geosite:cn)，ruleB 是 domain(geosite:cn) && dport(443))
      // 检查 ruleB 的所有子条件是否包含了 ruleA 的全部子条件
      const aRawSubs = new Set(ruleA.items.map(it => (it.negated ? '!' : '') + it.rawSub))
      const bRawSubs = new Set(ruleB.items.map(it => (it.negated ? '!' : '') + it.rawSub))

      let isASubsetOfB = true
      for (const aSub of aRawSubs) {
        if (!bRawSubs.has(aSub)) {
          isASubsetOfB = false
          break
        }
      }

      if (isASubsetOfB && ruleA.items.length < ruleB.items.length) {
        // ruleA 的条件少且全部在 ruleB 中，这意味着命中 ruleB 的流量必先命中 ruleA！
        conflicts.push({
          id: `shadowed-${i}-${j}`,
          severity: 'warning',
          ruleIndices: [i, j],
          title: `阴影规则遮蔽 (#${j + 1} 被 #${i + 1} 遮蔽)`,
          message: `第 ${i + 1} 行条件 "${ruleA.rawCondition}" 比第 ${j + 1} 行条件 "${ruleB.rawCondition}" 更通用。由于 dae 遵循自上而下「首条命中即终止」原则，满足第 ${j + 1} 行的流量必定在第 ${i + 1} 行被提前拦截并路由到 "${ruleA.target}"，导致第 ${j + 1} 行永远不会被执行！`,
          suggestion: `请调整规则顺序：将具体的第 ${j + 1} 行上移至第 ${i + 1} 行的上方，优先匹配特定流量。`
        })
        markRule(j, 'warning', `被第 ${i + 1} 行通用规则遮蔽 (Shadowed)`)
        continue
      }

      // (c) 域名遮蔽：上方是 geosite:cn，下方是属于 cn 的具体域名后缀或 geosite，且动作不同
      const aDomains = ruleA.items.filter(it => it.type === 'domain' && !it.negated)
      const bDomains = ruleB.items.filter(it => it.type === 'domain' && !it.negated)
      if (aDomains.length === 1 && bDomains.length === 1 && ruleA.items.length === 1 && ruleB.items.length === 1) {
        const aVal = aDomains[0].values.join(',')
        const bVal = bDomains[0].values.join(',')
        if (aVal === 'geosite:cn' && (bVal.includes('baidu.com') || bVal.includes('qq.com') || bVal.includes('bilibili.com'))) {
          conflicts.push({
            id: `domain-shadow-${i}-${j}`,
            severity: 'info',
            ruleIndices: [i, j],
            title: `域名规则顺序建议 (#${j + 1} 可能被 #${i + 1} 覆盖)`,
            message: `第 ${i + 1} 行已匹配广泛规则 "domain(geosite:cn)"，位于下方的中国大陆具体域名 "${bVal}" (第 ${j + 1} 行) 可能会先被上一条规则命中。`,
            suggestion: `如果希望对 "${bVal}" 施加特殊路由动作 (${ruleB.target})，请将其上移到 "domain(geosite:cn)" 规则的前面。`
          })
        }
      }
    }
  }

  // 4. 最佳实践检查：是否配置了内网私有地址放行 (dip(geoip:private) -> direct)
  const hasPrivateBypass = parsedRules.some(r => {
    return r.items.some(it => {
      if (it.type === 'dip' && !it.negated) {
        return it.values.some(v => v.includes('geoip:private') || v.includes('192.168.') || v.includes('10.0.') || v.includes('172.16.'))
      }
      return false
    }) && (r.target === 'direct' || r.target === 'must_direct')
  })

  if (!hasPrivateBypass && rules.length > 0) {
    conflicts.push({
      id: 'missing-private-bypass',
      severity: 'info',
      ruleIndices: [],
      title: '建议添加局域网私有 IP 直连规则',
      message: '当前分流规则中未检测到对局域网私网 IP (如 geoip:private) 的直连规则。若兜底规则或其他宽泛规则走向代理组，可能导致局域网设备互访受阻或网关管理面板无法打开。',
      suggestion: '建议在规则列表顶部添加一条分流规则：匹配条件为 "dip(geoip:private)"，目标出口为 "direct"。'
    })
  }

  // 5. 上游 DNS 规则直连建议 (DNS Loop 防范)
  parsedRules.forEach(r => {
    const hasDnsCondition = r.items.some(it => {
      if (it.type === 'dport' && it.values.some(p => p === '53')) return true
      if (it.type === 'dip' && it.values.some(ip => ip.includes('223.5.5.5') || ip.includes('119.29.29.29') || ip.includes('8.8.8.8') || ip.includes('1.1.1.1'))) return true
      return false
    })

    if (hasDnsCondition && r.target === 'direct') {
      conflicts.push({
        id: `dns-loop-risk-${r.index}`,
        severity: 'info',
        ruleIndices: [r.index],
        title: `上游 DNS 建议使用 must_direct (#${r.index + 1})`,
        message: `第 ${r.index + 1} 行匹配了 DNS 流量/上游服务器，当前动作为 "direct"。在 dae 开启 DNS 嗅探劫持的环境下，普通的 direct 可能会被再次捕获引发潜在递归回环。`,
        suggestion: '建议将目标动作更改为 "must_direct"（强制直连且免除 DNS 劫持），确保 DNS 解析安全稳定。'
      })
    }
  })

  const hasErrors = conflicts.some(c => c.severity === 'error')
  const hasWarnings = conflicts.some(c => c.severity === 'warning')

  return {
    totalRules: rules.length,
    hasErrors,
    hasWarnings,
    conflicts,
    ruleStatusMap
  }
}
