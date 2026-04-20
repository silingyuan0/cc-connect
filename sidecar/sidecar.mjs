/**
 * Claude Agent SDK Sidecar for cc-connect
 *
 * Protocol (JSON lines over stdin/stdout):
 *   Go → Sidecar (stdin):  { "type": "send"|"permission"|"close"|"set_mode"|"abort", ... }
 *   Sidecar → Go (stdout): { "event": "system"|"text"|"tool_use"|..., ... }
 *
 * CLI args: sidecar.mjs <cwd> [model] [mode] [resume] [claude_path]
 * Config:   SIDECAR_CONFIG env var with JSON for systemPrompt, allowedTools, mcpServers, etc.
 *
 * Multi-turn: one query() call handles multiple turns via PushableAsyncIterable.
 * Each turn: push user message → SDK emits system.init → assistant messages → result → ready.
 * On result, sidecar writes {event:"ready"} and waits for the next stdin command.
 * If next command is "send", push into same iterable (same session, same process).
 * If next command is "close", end the iterable and exit.
 *
 * Hot mode switch: "set_mode" command calls response.setPermissionMode() without restarting.
 * Abort: "abort" command triggers AbortController to cancel the running query.
 */

import { query } from '@anthropic-ai/claude-agent-sdk'

// ── PushableAsyncIterable ────────────────────────────────────

class PushableAsyncIterable {
  #queue = []
  #waiters = []
  #done = false

  push(value) {
    if (this.#done) throw new Error('ended')
    if (this.#queue.length > 0 || this.#waiters.length === 0) {
      this.#queue.push(value)
    } else {
      this.#waiters.shift()({ done: false, value })
    }
  }

  end() {
    this.#done = true
    while (this.#waiters.length) this.#waiters.shift()({ done: true })
  }

  get ended() { return this.#done }

  [Symbol.asyncIterator]() { return this }

  async next() {
    if (this.#queue.length) return { done: false, value: this.#queue.shift() }
    if (this.#done) return { done: true }
    return new Promise((r) => this.#waiters.push(r))
  }
}

// ── Helpers ──────────────────────────────────────────────────

function writeStdout(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n')
}

function stderr(...args) {
  process.stderr.write('[sidecar] ' + args.join(' ') + '\n')
}

// Pending permission requests: requestId → resolve function
const pendingPermissions = new Map()

function waitForPermission(requestId) {
  return new Promise((resolve) => {
    pendingPermissions.set(requestId, resolve)
  })
}

function resolvePermission(requestId, result) {
  const resolve = pendingPermissions.get(requestId)
  if (resolve) {
    pendingPermissions.delete(requestId)
    resolve(result)
  }
}

// ── Stdin reader with intercept support ──────────────────────
// The reader stays attached to stdin and dispatches commands.
// Permission responses are intercepted inline and never reach
// the command handler.  set_mode/abort are handled immediately
// and the reader keeps going for the next real command.

let stdinBuffer = ''
let stdinResolve = null

function startStdinReader() {
  process.stdin.on('data', (chunk) => {
    stdinBuffer += chunk.toString()
    processStdinBuffer()
  })

  process.stdin.on('end', () => {
    // stdin closed — treat as close
    if (stdinResolve) {
      stdinResolve({ type: 'close' })
      stdinResolve = null
    }
  })
}

function processStdinBuffer() {
  while (true) {
    const newlineIdx = stdinBuffer.indexOf('\n')
    if (newlineIdx === -1) break

    const line = stdinBuffer.substring(0, newlineIdx).trim()
    stdinBuffer = stdinBuffer.substring(newlineIdx + 1)

    if (!line) continue

    let cmd
    try { cmd = JSON.parse(line) } catch (e) {
      stderr('parse error:', e.message)
      continue
    }

    // Intercept: permission responses never reach the command handler
    if (cmd.type === 'permission') {
      resolvePermission(cmd.requestId, cmd.result)
      continue
    }

    // Intercept: abort — trigger abort controller
    if (cmd.type === 'abort') {
      if (abortController) {
        stderr('abort requested')
        abortController.abort()
      }
      continue
    }

    // Intercept: set_mode — hot switch without restart
    if (cmd.type === 'set_mode') {
      if (queryResponse && queryResponse.setPermissionMode) {
        stderr('hot mode switch:', cmd.mode)
        queryResponse.setPermissionMode(cmd.mode).catch(e => {
          stderr('setPermissionMode error:', e.message)
        })
      }
      continue
    }

    // Real command — deliver to waiting reader
    if (stdinResolve) {
      stdinResolve(cmd)
      stdinResolve = null
    } else {
      stderr('unexpected command (no reader waiting):', cmd.type)
    }
  }
}

function readStdinCommand() {
  return new Promise((resolve) => {
    stdinResolve = resolve
    // Check if buffer already has a command
    processStdinBuffer()
  })
}

// ── AbortController for the query ────────────────────────────

let abortController = new AbortController()
let queryResponse = null

// ── Map SDK assistant content blocks → sidecar events ────────

function mapAssistantContent(content) {
  if (!Array.isArray(content)) return
  for (const block of content) {
    if (block.type === 'text' && block.text) {
      writeStdout({ event: 'text', content: block.text })
    } else if (block.type === 'tool_use') {
      writeStdout({
        event: 'tool_use',
        toolName: block.name,
        toolId: block.id,
        input: block.input,
      })
    } else if (block.type === 'thinking' && block.thinking) {
      writeStdout({ event: 'thinking', content: block.thinking })
    }
  }
}

// ── Build systemPrompt for SDK ───────────────────────────────

// Prevent local MCP HTTP servers from being routed through HTTP proxy.
// Appends 127.0.0.1, localhost, ::1 to NO_PROXY if missing.
function ensureLocalProxyBypass(env) {
  const existing = env.NO_PROXY ?? env.no_proxy ?? ''
  const entries = existing.split(',').map(s => s.trim()).filter(Boolean)
  const toAdd = ['127.0.0.1', 'localhost', '::1'].filter(h => !entries.includes(h))
  if (toAdd.length === 0) return
  const updated = existing ? `${existing},${toAdd.join(',')}` : toAdd.join(',')
  env.NO_PROXY = updated
  env.no_proxy = updated
}

// ── Build systemPrompt for SDK ───────────────────────────────

function buildSystemPrompt(config) {
  if (!config) return undefined

  // Custom system prompt takes full precedence
  if (config.customSystemPrompt) {
    return config.customSystemPrompt
  }

  // appendSystemPrompt triggers claude_code preset + append
  if (config.appendSystemPrompt) {
    return {
      type: 'preset',
      preset: 'claude_code',
      append: config.appendSystemPrompt,
    }
  }

  // No custom prompt — use default claude_code preset for full capabilities
  return { type: 'preset', preset: 'claude_code' }
}

// ── Parse SIDECAR_CONFIG env var ─────────────────────────────

function parseSidecarConfig() {
  const raw = process.env.SIDECAR_CONFIG
  if (!raw) return {}

  try {
    const config = JSON.parse(raw)
    stderr('loaded SIDECAR_CONFIG with keys:', Object.keys(config).join(', '))
    return config
  } catch (e) {
    stderr('failed to parse SIDECAR_CONFIG:', e.message)
    return {}
  }
}

// ── Main ─────────────────────────────────────────────────────

async function main() {
  const cwd = process.argv[2] || process.cwd()
  const model = process.argv[3] || undefined
  const mode = process.argv[4] || undefined
  const resume = process.argv[5] || undefined
  const claudePath = process.argv[6] || undefined

  // Parse extended config from env var
  const config = parseSidecarConfig()

  stderr(`starting cwd=${cwd} model=${model || 'default'} mode=${mode || 'default'} resume=${resume || 'none'}`)

  // Start the stdin reader early so we don't miss any commands
  startStdinReader()

  // Wait for first user message from Go
  stderr('waiting for first message...')
  const firstCmd = await readStdinCommand()
  if (firstCmd.type !== 'send') {
    stderr('expected "send" as first command, got:', firstCmd.type)
    process.exit(1)
  }

  // Create prompt stream and push first message
  const messages = new PushableAsyncIterable()
  messages.push({
    type: 'user',
    parent_tool_use_id: null,
    message: { role: 'user', content: firstCmd.content },
  })

  // Detect Claude CLI path
  const resolvedClaudePath = claudePath || process.env.CLAUDE_PATH || undefined
  if (resolvedClaudePath) stderr(`using claude at: ${resolvedClaudePath}`)

  // Build env with local MCP proxy bypass
  const queryEnv = { ...process.env }
  if (config.mcpServers && Object.keys(config.mcpServers).length > 0) {
    ensureLocalProxyBypass(queryEnv)
  }

  // Build SDK query options with full config
  const queryOptions = {
    cwd,
    model: model || undefined,
    resume: resume || undefined,
    permissionMode: mode || undefined,
    pathToClaudeCodeExecutable: resolvedClaudePath,
    abortController,

    // Environment with local MCP proxy bypass
    env: queryEnv,

    // Required when using bypassPermissions mode
    allowDangerouslySkipPermissions: mode === 'bypassPermissions' ? true : undefined,

    // System prompt
    systemPrompt: buildSystemPrompt(config),

    // Reasoning
    effort: config.effort || undefined,
    thinking: config.thinking || undefined,

    // Tools
    allowedTools: config.allowedTools || undefined,
    disallowedTools: config.disallowedTools || undefined,
    tools: config.tools || undefined,

    // MCP
    mcpServers: config.mcpServers || undefined,
    strictMcpConfig: config.strictMcpConfig || undefined,

    // Settings
    settings: config.settingsPath || undefined,
    settingSources: config.settingSources || undefined,

    // Limits
    maxTurns: config.maxTurns || undefined,
    maxBudgetUsd: config.maxBudgetUsd || undefined,
    fallbackModel: config.fallbackModel || undefined,

    // Agent system
    agent: config.agent || undefined,
    agents: config.agents || undefined,

    // Plugins
    plugins: config.plugins || undefined,

    // Sandbox
    sandbox: config.sandbox || undefined,

    // Other
    enableFileCheckpointing: config.enableFileCheckpointing || undefined,
    includeHookEvents: config.includeHookEvents || undefined,
    debug: config.debug || undefined,
    debugFile: config.debugFile || undefined,

    // Session
    continue: config.continue || undefined,

    // Permission callback
    canUseTool: async (toolName, input, options) => {
      const requestId = options.toolUseID
      writeStdout({
        event: 'permission_request',
        requestId,
        toolName,
        input,
      })
      return await waitForPermission(requestId)
    },
  }

  stderr('query options:', JSON.stringify({
    systemPrompt: queryOptions.systemPrompt ? 'configured' : 'none',
    effort: queryOptions.effort || 'default',
    thinking: queryOptions.thinking ? 'configured' : 'default',
    allowedTools: queryOptions.allowedTools?.length || 0,
    disallowedTools: queryOptions.disallowedTools?.length || 0,
    tools: queryOptions.tools ? 'configured' : 'default',
    mcpServers: queryOptions.mcpServers ? Object.keys(queryOptions.mcpServers).length : 0,
    settings: queryOptions.settings || 'none',
    settingSources: queryOptions.settingSources || 'none',
    maxTurns: queryOptions.maxTurns || 'unlimited',
    maxBudgetUsd: queryOptions.maxBudgetUsd || 'unlimited',
    agent: queryOptions.agent || 'none',
    agents: queryOptions.agents ? Object.keys(queryOptions.agents).length : 0,
    plugins: queryOptions.plugins?.length || 0,
    sandbox: queryOptions.sandbox ? 'configured' : 'none',
  }))

  // Start query with multi-turn support
  queryResponse = query({
    prompt: messages,
    options: queryOptions,
  })

  stderr('query() started, iterating response...')

  try {
    for await (const msg of queryResponse) {
      switch (msg.type) {
        case 'system':
          if (msg.subtype === 'init') {
            writeStdout({
              event: 'system',
              subtype: 'init',
              sessionId: msg.session_id,
              model: msg.model,
              cwd: msg.cwd,
              tools: msg.tools,
              slashCommands: msg.slash_commands,
            })
          }
          break

        case 'assistant':
          if (msg.message?.content) {
            mapAssistantContent(msg.message.content)
          }
          break

        case 'result':
          writeStdout({
            event: 'result',
            content: msg.result || '',
            sessionId: msg.session_id,
            usage: msg.usage,
            cost: msg.total_cost_usd,
          })
          // Signal ready and wait for next command from Go
          writeStdout({ event: 'ready' })

          stderr('result received, waiting for next command...')
          const cmd = await readStdinCommand()

          if (cmd.type === 'send') {
            // Multi-turn: push into same query — same session, same process
            stderr('pushing next message into same query')
            messages.push({
              type: 'user',
              parent_tool_use_id: null,
              message: { role: 'user', content: cmd.content },
            })
          } else if (cmd.type === 'close') {
            stderr('close command received, ending session')
            messages.end()
            return
          }
          break

        case 'user':
          // Tool results etc — forwarded for visibility
          break
      }
    }
  } catch (e) {
    if (e.name === 'AbortError') {
      stderr('query aborted')
      writeStdout({ event: 'aborted' })
    } else {
      writeStdout({ event: 'error', message: e.message, name: e.name || 'Error' })
      stderr('query error:', e.message)
    }
  }

  stderr('exiting')
}

main().catch((e) => {
  stderr('fatal:', e.message)
  writeStdout({ event: 'error', message: e.message })
  process.exit(1)
})
