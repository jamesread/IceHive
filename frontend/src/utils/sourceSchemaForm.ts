/** Matches `services/common/pkg/sourceschema` JSON on the wire. */

export const sourceSchemaKind = 'SourceSchema'

export interface SourceSchemaArg {
  id: string
  label: string
  format?: string
}

export interface SourceSchemaPrimaryPattern {
  id: string
  syntax_prefix: string
  label: string
  args: SourceSchemaArg[]
  value_template: string
  example?: string
}

export interface SourceSchemaModifier {
  id: string
  label: string
  kind: string
  syntax_suffix: string
}

export interface SourceSchemaCronHint {
  optional?: boolean
  description?: string
}

export interface SourceSchemaDoc {
  kind: string
  schema_version: string
  collector_type: string
  primary_patterns: SourceSchemaPrimaryPattern[]
  modifiers: SourceSchemaModifier[]
  cron?: SourceSchemaCronHint
}

export function parseSourceSchemaDoc(bodyJson: string): SourceSchemaDoc | null {
  try {
    const o = JSON.parse(bodyJson) as SourceSchemaDoc
    if (!o || o.kind !== sourceSchemaKind || !Array.isArray(o.primary_patterns)) {
      return null
    }
    return o
  } catch {
    return null
  }
}

export interface BuilderState {
  patternId: string
  args: Record<string, string>
  modifiers: Record<string, boolean>
}

export function defaultBuilderState(doc: SourceSchemaDoc): BuilderState {
  const first = doc.primary_patterns[0]
  const args: Record<string, string> = {}
  for (const a of first?.args ?? []) {
    args[a.id] = ''
  }
  const modifiers: Record<string, boolean> = {}
  for (const m of doc.modifiers) {
    modifiers[m.id] = false
  }
  return {
    patternId: first?.id ?? '',
    args,
    modifiers,
  }
}

/** Parse `source_spec` into builder fields when it matches this schema's patterns and modifiers. */
export function parseSpecIntoBuilder(doc: SourceSchemaDoc, spec: string): BuilderState | null {
  const trimmed = spec.trim()
  if (!trimmed) {
    return null
  }
  const parts = trimmed.split(/ \+/).map((s) => s.trim()).filter(Boolean)
  if (parts.length === 0) {
    return null
  }
  const primary = parts[0]
  const modTokens = new Set(parts.slice(1))

  for (const p of doc.primary_patterns) {
    if (!primary.startsWith(p.syntax_prefix)) {
      continue
    }
    const rest = primary.slice(p.syntax_prefix.length)
    const args: Record<string, string> = {}
    if (p.id === 'repo') {
      const m = /^([^/]+)\/(.+)$/.exec(rest)
      if (!m) {
        continue
      }
      args.owner = m[1]
      args.name = m[2]
    } else if (p.id === 'org.repos') {
      if (!/^[^/\s]+$/.test(rest)) {
        continue
      }
      args.login = rest
    } else if (p.args.length === 1) {
      args[p.args[0].id] = rest
    } else {
      continue
    }
    const modifiers: Record<string, boolean> = {}
    for (const mod of doc.modifiers) {
      modifiers[mod.id] = modTokens.has(mod.syntax_suffix)
    }
    return { patternId: p.id, args, modifiers }
  }
  return null
}

export function composeSpec(doc: SourceSchemaDoc, b: BuilderState): string {
  const p = doc.primary_patterns.find((x) => x.id === b.patternId)
  if (!p) {
    return ''
  }
  let s = p.value_template
  for (const a of p.args) {
    const v = (b.args[a.id] ?? '').trim()
    s = s.split(`{${a.id}}`).join(v)
  }
  for (const m of doc.modifiers) {
    if (m.kind === 'boolean' && b.modifiers[m.id]) {
      s += ` +${m.syntax_suffix}`
    }
  }
  return s
}

export function builderStateForPattern(doc: SourceSchemaDoc, patternId: string, prev?: BuilderState | null): BuilderState {
  const p = doc.primary_patterns.find((x) => x.id === patternId)
  const args: Record<string, string> = {}
  for (const a of p?.args ?? []) {
    args[a.id] = prev?.args[a.id] ?? ''
  }
  const modifiers: Record<string, boolean> = {}
  for (const m of doc.modifiers) {
    modifiers[m.id] = prev?.modifiers[m.id] ?? false
  }
  return { patternId, args, modifiers }
}
