---
layout: default
title: GPTCode — Verifiable AI coding workflows
description: An open-source coding CLI for multi-model, repository-native, verifiable engineering workflows.
---

<section class="hero" id="overview">
  <span class="hero-kicker">Open source · Local-first · Built in Go</span>
  <h1>AI coding agents should produce<br><span>evidence, not just answers.</span></h1>
  <p>GPTCode is an open-source coding CLI for researching, planning, implementing, and reviewing software with multiple AI models—while keeping context, engineering rules, and validation close to the code.</p>
  <div class="hero-cta">
    <a href="#install" class="btn btn-primary">Install GPTCode</a>
    <a href="#architecture" class="btn btn-secondary">Explore the architecture</a>
  </div>
  <p class="hero-meta">Bring your own model · OpenRouter · OpenAI · Gemini · Groq · Ollama</p>
</section>

<section class="terminal-showcase" id="workflows" aria-labelledby="workflow-title">
  <div class="terminal-intro">
    <div class="section-label">A workflow you can inspect</div>
    <h2>From repository question to verified change</h2>
  </div>
  <div class="terminal-window">
    <div class="terminal-bar" aria-hidden="true">
      <span></span><span></span><span></span>
      <strong>gptcode — real terminal capture</strong>
    </div>
    <video class="terminal-recording" autoplay muted loop playsinline controls poster="{{ '/assets/gptcode-workflow.gif' | relative_url }}" aria-label="Real terminal recording: GPTCode researches and reviews a Go data race, repairs it without changing the public API, and verifies the result with the race detector.">
      <source src="{{ '/assets/gptcode-workflow.mp4' | relative_url }}" type="video/mp4">
      <img src="{{ '/assets/gptcode-workflow.gif' | relative_url }}" alt="GPTCode researches and reviews a Go data race, repairs it without changing the public API, and verifies the result with the race detector.">
    </video>
  </div>
  <p class="recording-caption">Real capture · GPTCode repository detection and skills · Go verification executed by the repository</p>
  <div class="workflow-signature" id="workflow-title">
    <span>Investigate</span><b>→</b><span>Plan</span><b>→</b><span>Implement</span><b>→</b><span>Review</span><b>→</b><span>Verify</span>
  </div>
  <p class="workflow-explainer"><strong>One model rarely excels at every task.</strong> GPTCode decomposes software development into explicit stages so each step can use the best model, repository knowledge, and deterministic verification.</p>
  <div class="evidence-links">
    <span>Inspect the evidence</span>
    <a href="https://github.com/jadercorrea/gptcode/tree/main/cmd/gptcode"><b>✓</b><strong>Workflow implementation</strong><small>Inspect the execution code</small></a>
    <a href="https://github.com/jadercorrea/gptcode/tree/main/tests"><b>✓</b><strong>Verification commands</strong><small>Inspect the test suite</small></a>
    <a href="https://github.com/jadercorrea/gptcode/tree/main/cmd/gptcode/skills"><b>✓</b><strong>Repository skills</strong><small>Inspect engineering rules</small></a>
  </div>
</section>

<section class="thesis-section">
  <div class="section-label">The thesis</div>
  <h2>Coding agents are useful.<br>Unverified agents are dangerous.</h2>
  <p>Language models are good at exploring possibilities, but production software depends on explicit constraints, stable contracts, and executable verification.</p>
  <p>GPTCode separates model reasoning from system authority. Agents can investigate, plan, implement, and review. The repository defines the constraints, and deterministic tools determine whether the result is correct.</p>
</section>

<section class="philosophy-section">
  <div class="section-label">Engineering philosophy</div>
  <h2>Reliable AI systems are built on explicit constraints, not optimistic prompts.</h2>
  <p><span>Models</span> generate possibilities.</p>
  <p><span>Repositories</span> define constraints.</p>
  <p><span>Verification</span> establishes truth.</p>
  <strong>GPTCode exists to keep these responsibilities separate.</strong>
</section>

<section class="pillar-section" aria-labelledby="pillars-title">
  <div class="section-label">Design principles</div>
  <h2 id="pillars-title">A workflow designed around engineering evidence</h2>
  <div class="pillar-grid">
    <article class="pillar">
      <span>01</span>
      <h3>Model routing by task</h3>
      <p>Use different models for research, planning, implementation, and review. Choose based on capability, latency, cost, or privacy instead of locking the workflow to one provider.</p>
    </article>
    <article class="pillar">
      <span>02</span>
      <h3>Repository-native skills</h3>
      <p>Store engineering conventions, architectural constraints, testing practices, and review criteria alongside the code they govern.</p>
    </article>
    <article class="pillar">
      <span>03</span>
      <h3>Explicit agent workflows</h3>
      <p>Separate research, planning, execution, and review into inspectable stages instead of hiding the development process behind a single prompt.</p>
    </article>
    <article class="pillar">
      <span>04</span>
      <h3>Executable verification</h3>
      <p>Treat tests, linters, type checks, and project commands as evidence. A model may propose a change; the system must verify it.</p>
    </article>
  </div>
</section>

<section class="architecture-section" id="architecture">
  <div class="architecture-copy">
    <div class="section-label">Repository-centered architecture</div>
    <h2>The workflow is the source of truth. Models are interchangeable execution engines.</h2>
    <p>Repository knowledge enters an explicit pipeline. Models contribute at each stage, while skills, contracts, tools, and verification remain anchored to the codebase.</p>
  </div>
  <div class="pipeline-diagram" role="img" aria-label="Repository skills, contracts, and tests feed the GPTCode research, planning, implementation, review, and verification pipeline, which can route tasks to multiple model providers.">
    <div class="pipeline-repository">
      <strong>Repository</strong>
      <div><span>├── Skills</span><span>├── Contracts</span><span>├── Tests</span><span>└── Documentation</span></div>
    </div>
    <div class="pipeline-arrow">↓</div>
    <div class="pipeline-core">
      <strong>GPTCode execution pipeline</strong>
      <div><span>Research</span><b>↓</b><span>Planning</span><b>↓</b><span>Implementation</span><b>↓</b><span>Review</span><b>↓</b><span>Verification</span></div>
    </div>
    <div class="pipeline-arrow">↓</div>
    <div class="pipeline-models">
      <small>Routed execution engines</small>
      <span>OpenAI</span><span>Gemini</span><span>Groq</span><span>OpenRouter</span><span>Ollama</span>
    </div>
  </div>
</section>

<section class="skills-section" id="skills">
  <div>
    <div class="section-label">Repository-native knowledge</div>
    <h2>Engineering knowledge that lives with the repository</h2>
    <p>Skills are reusable, version-controlled Markdown instructions for how work should be performed in a codebase. They can define language conventions, architectural boundaries, testing strategies, review checklists, and project-specific workflows.</p>
    <a href="{{ '/skills' | relative_url }}" class="text-link">Explore the included skills →</a>
  </div>
  <div class="skill-example">
    <div class="code-caption">skills/elixir.md</div>
    <pre><code># Elixir Patterns

Guidelines for writing idiomatic Elixir
and Phoenix applications.

## When to Activate

- Writing or editing Elixir code
- Working with Phoenix applications
- Reviewing Elixir code

## Pattern Matching

### Use pattern matching in function heads

def handle_response({:ok, body}),
  do: process(body)

def handle_response({:error, reason}),
  do: log_error(reason)</code></pre>
  </div>
</section>

<section class="use-cases">
  <div class="section-label">Use cases</div>
  <h2>From ambiguity to verified change</h2>
  <div class="use-case-list">
    <article><h3>Reduce onboarding time</h3><p>Map modules, dependencies, and execution paths before changing an unfamiliar codebase.</p></article>
    <article><h3>Make architectural decisions inspectable</h3><p>Turn an ambiguous request into a plan with explicit constraints, risks, and verification steps.</p></article>
    <article><h3>Reduce implementation risk</h3><p>Apply changes with repository-specific conventions and boundaries available throughout execution.</p></article>
    <article><h3>Challenge the first answer</h3><p>Use a distinct review stage to question assumptions, identify regressions, and verify the implementation.</p></article>
  </div>
</section>

<section class="founder-note">
  <div class="section-label">Why I built it</div>
  <h2>Why I built GPTCode</h2>
  <p>I built GPTCode to explore a question that became increasingly important while building production software with AI:</p>
  <blockquote>How can models contribute meaningfully to engineering work without becoming the source of truth for the system?</blockquote>
  <p>GPTCode is my answer in executable form. It combines multi-model workflows, repository-native knowledge, and deterministic verification in a tool I use to test ideas about reliable agentic development.</p>
  <p class="signature">— Jader Correa<br><span>Principal engineer and founder</span></p>
</section>

<section class="open-source-section">
  <div>
    <div class="section-label">Independent and open source</div>
    <h2>An engineering laboratory for reliable coding-agent workflows</h2>
    <p>The project is intentionally local, provider-independent, and transparent. Contributions, bug reports, and technical discussion are welcome.</p>
  </div>
  <p class="project-facts">MIT licensed <span>·</span> Written in Go <span>·</span> No hosted account required</p>
</section>

<section class="install-section" id="install">
  <div class="section-label">Get started</div>
  <h2>Bring your own models.<br>Keep control of the workflow.</h2>
  <p>Install GPTCode, connect a provider, and start exploring your repository from the terminal.</p>
  <div class="install-command">
    <pre><code>go install github.com/jadercorrea/gptcode/cmd/gptcode@latest

gt setup
gt key openrouter
gt chat</code></pre>
  </div>
  <div class="hero-cta">
    <a href="https://github.com/jadercorrea/gptcode" class="btn btn-primary">View source on GitHub</a>
    <a href="{{ '/reference/commands' | relative_url }}" class="btn btn-secondary">Read the documentation</a>
  </div>
</section>
