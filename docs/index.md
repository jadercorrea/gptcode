---
layout: default
title: GPTCode CLI — Premium Compiled Agent Documentation
description: Official documentation for the compiled CLI agent included with gptcode live subscriptions. Secure local execution, real-time dashboard visualization, and active command controls.
---

<div class="hero">
  <h1>The Compiled CLI Agent for<br/>gptcode live</h1>
  <p>Run high-performance, secure autonomous coding loops directly from your local terminal or remote server. The <strong>gptcode CLI</strong> is a premium pre-compiled binary provided exclusively to active <strong>gptcode live</strong> subscribers, fully connected to your real-time dashboard.</p>
  <div class="hero-cta">
    <a href="#connection-setup" class="btn btn-primary">Connect Your Agent</a>
    <a href="/reference/commands" class="btn btn-secondary">Commands Reference</a>
  </div>
</div>

<div class="section">
  <h2 class="section-title">Cohesive Architecture: Local Execution, Centralized Control</h2>
  <p class="section-subtitle">Your code stays local; your monitoring, cost guardrails, and audit logs are live.</p>

  <div class="workflow-steps" style="grid-template-columns: repeat(3, 1fr);">
    <div class="workflow-step">
      <h3>1. Secure Local Run</h3>
      <p>The compiled CLI agent executes tasks on your machine, leveraging local git, compilers, and test suites securely.</p>
      <pre><code>gt do "implement JWT auth"</code></pre>
    </div>
    
    <div class="workflow-step">
      <h3>2. WebSocket Stream</h3>
      <p>Every stdout/stderr line, token transaction, and model choice is streamed live in real-time to your dashboard.</p>
      <pre><code>Connection: STREAMS ACTIVE</code></pre>
    </div>
    
    <div class="workflow-step">
      <h3>3. Active Supervision</h3>
      <p>The Live Dashboard isn't passive. Remotely pause, inject commands, or terminate loops instantly via the visual panel.</p>
      <pre><code>Signal: SIGINT (Remote)</code></pre>
    </div>
  </div>
</div>

<div class="section" id="connection-setup">
  <h2 class="section-title">Connection & Setup</h2>
  <p class="section-subtitle">Connect your compiled CLI agent to the Live Dashboard in three simple steps</p>
  
  <div class="quick-start">
    <h3>1. Download Your OS Binary</h3>
    <p>Sign in to your <a href="https://gptcode.live">gptcode live</a> account, navigate to the Downloads tab, and fetch the compiled binary for your platform (macOS Apple Silicon/Intel, Linux x86/ARM, Windows).</p>
    
    <h3>2. Authenticate Your Terminal</h3>
    <p>Run the login command and paste your unique secure Live Token to link your host device to the dashboard:</p>
    <pre><code>gt login --token &lt;your-live-token&gt;</code></pre>
    
    <h3>3. Start Coding with Live Streams</h3>
    <p>Run any autonomous or interactive command. Your terminal session will instantly stream live into your web and mobile dashboards:</p>
    <pre><code># Run an autonomous coding loop
gt do "fix payment webhook race condition"

# Start a code-focused chat session
gt chat

# Perform an automated security & quality review
gt review .</code></pre>
  </div>
</div>

<div class="section">
  <h2 class="section-title">Core CLI Agent Capabilities</h2>
  
  <h3>Flagship Operations</h3>
  <ul>
    <li><strong>Autonomous Copilot (<code>gt do "task"</code>)</strong>: Orchestrates specialized agents (Analyzer → Planner → Editor → Validator) to solve tasks, run tests, and self-heal with automatic model switching.</li>
    <li><strong>Interactive REPL (<code>gt chat</code>)</strong>: Context-aware interactive console. Pre-indexes your files using a localized dependency graph for efficient model usage.</li>
    <li><strong>Quality & Security Audit (<code>gt review [target]</code>)</strong>: Automated review of files or folders against security, performance, and naming conventions.</li>
    <li><strong>Operation Runner (<code>gt run "task"</code>)</strong>: Command-line agent mode designed for DevOps and operational loops (HTTP requests, container logs monitoring, deployment runs).</li>
  </ul>
  
  <h3>Enterprise Guardrails Enabled Natively</h3>
  <ul>
    <li><strong>Active Security Interception</strong>: When the agent attempts to run a blocked command defined in your `agentops.yml`, the Live dashboard actively intercepts and interrupts the process.</li>
    <li><strong>Automatic PII Redaction</strong>: Standard ML and regex scrubbing plugins run on the stream in real-time, removing secrets, credentials, or personal information before logs reach the dashboard.</li>
    <li><strong>Runaway Loop Breaker</strong>: If the agent gets stuck in a loop, the Live policy engine triggers a remote `SIGTERM` automatically when cost budgets are exceeded.</li>
  </ul>
</div>

<div class="section">
  <h2 class="section-title">Why Compiled Agent + Live?</h2>
  
  <p>Traditional AI agents run local-only, presenting severe security, visibility, and compliance risks in professional environments. The gptcode compiled CLI agent + Live Dashboard is designed to make agent execution robust, visible, and fully auditable.</p>
  
  <ul>
    <li><strong>Enterprise Ready</strong>: Meets SOC2 CC logical access and system monitoring specifications.</li>
    <li><strong>Complete Auditability</strong>: Fully searchable history of all agent sessions, actions, and file diffs.</li>
    <li><strong>Resource Metering</strong>: Centralized dashboards to track token spending, API usage, and costs across all host servers and developers.</li>
  </ul>
</div>
