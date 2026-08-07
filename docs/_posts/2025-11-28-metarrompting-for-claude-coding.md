# Metaprompting for Claude Coding: 286 Lines Buggy vs. 4 Modular Files Functional

In a live demo building a Node.js CLI for generative pentatonic music with trippy visuals, direct prompting delivered a 286-line `index.js` with clicky audio and emoji glitches in ~1 minute. Metaprompting produced a 63-line entrypoint + `audio.js` (118 lines), `visuals.js` (92 lines), and `README.md` with FM synthesis, fractal visuals, and 8 interactive scales in 8 minutes.

## Problem (with metrics)

Direct "vibe coding" prompts from tools like v0 or Lovable require 20-30 follow-ups per feature: 5 clarifications on libraries/visuals/sequencing/runtime/dependencies, plus fixes for breaks and untested code.

Demo evidence:
```
User: Create a CLI tool for generative music with trippy visuals...
Claude: 1. Audio library? 2. Terminal graphics? 3. Note sequencing? 4. Runtime? 5. NPM installs?
```
Result after answers: `npm start` yields:
```
[Music: clicky, uniform random notes]
Visuals: Emojis, no rainbows
Single 286-line index.js, blocks on audio
Scale change: Works but text glitches
```

No verification steps; 100% manual debugging needed.

## Solution (with examples)

Prefix tasks with `/create-prompt "description"` to spawn a prompt engineer. It analyzes clarity, parallelizes subtasks, generates XML prompts saved to `./prompts/001-description.txt`, then offers:
1. Run now (`/run-prompt`)
2. Edit
3. Save

`/run-prompt` launches self-contained subagents (fresh context windows).

Demo #1 initial creation:
```
/create-prompt "Create a CLI based tool for generative music using pentatonic scales (75% 16th-note play chance, weighted towards roots/octaves), trippy rainbow visuals with Blessed, indefinite runtime."
```
Generated `./prompts/001-generative-music-cli.txt` excerpt:
```
<objective>Build terminal-based generative music app...</objective>
<context>New Node.js CLI. Audio: speaker lib, 75% note chance, weighted pentatonic.</context>
<requirements>Visuals: Blessed rainbow aesthetics. Avoid blocking ops.</requirements>
<success_criteria>Plays pleasing music 30s, visuals evolve, npm start runs indefinitely.</success_criteria>
<verification>npm install && npm start; check no blocks, weighted notes audible.</verification>
```

User: "1" → Subagent runs, outputs modular files.

Demo #2 enhancement:
```
/create-prompt "Add rainbow fractal visuals, sample-and-hold sine feedback (FM style), real-time scale selection (major modes + pentatonics) via arrows."
```
Generated prompt examines existing files (`ls src/`), adds `<technical_approach>Research terminal fractals (Unicode spirals)... Modify audio.js, visuals.js.</technical_approach>`.

## Impact (comparative numbers)

| Metric | Direct Prompt | Metaprompting |
|--------|---------------|---------------|
| Files | 1 (`index.js`: 286 lines) | 4 (entry: 63 lines, audio: 118, visuals: 92, README: 45) |
| Audio | Clicky, uniform random (no weighting) | FM-modulated sines, weighted roots/octaves/fifths (pleasing, 75% density) |
| Visuals | Emojis, static | Evolving fractals/spirals, rainbow spectrum (60 FPS terminal) |
| Interactivity | Scale switch: glitches text | 8 scales (major modes + pentatonics), bottom-right indicator |
| Runtime | Blocks occasionally | Indefinite, non-blocking |
| Time to functional | ~1 min + 10+ fixes | 8 min total (create 2 min + run 6 min) |
| Verification | Manual | Built-in: "Ran 30s, no errors" |

`npm start` output (direct, buggy):
```
🎵 [clicky notes] 🌈😎 [emoji static]
```
Metaprompting (functional):
```
[harmonic flow] Spiral fractals evolve rainbow...
Scale: Lydian → Pentatonic Minor [bottom-right indicator]
```

Modularity: 78% fewer lines in entrypoint (63/286).

## How It Works (technical)

1. **Analysis (`<thinking>`)**: Clarity check (ask if vague, e.g., "Build dashboard?" → "Admin/analytics? Data?"). Scope: single/multi-file, sequential/parallel subtasks (e.g., audio || visuals).
2. **Construction**: Anthropic XML [docs](https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering#structured-prompts-with-xml-tags):
   ```
   <objective>...</objective>
   <context>Examine: ls src/*.js</context>
   <requirements>Go beyond basics; impress with fractals.</requirements>
   <constraints>Avoid blocking; use async.</constraints>
   <output>Save to src/; npm start verifiable.</output>
   <success_criteria>Plays 30s without crash; scales switch.</success_criteria>
   ```
3. **Save/Number**: `./prompts/001.txt` (reads dir for seq: `ls prompts/ | tail -1 | sed 's/.../002/'`).
4. **Run**: Subagents (`/run-prompt 001.txt`): Parallel for indep files (3 agents: audio/visuals/entry); sequential if deps.
5. **Post-run**: Moves to `./completed/`.

Triggers: "Deeply consider" → extended thinking; tools: bash (`npm i`), file reads.

## Try It (working commands)

Setup: Claude Code project dir. Paste prompts as custom commands.

**create-prompt** (full in [Tash GitHub](https://github.com/tash/metaprompts)):
```
You are an expert prompt engineer... <user_request>{args}</user_request>
<thinking>Clarity? Multi-file? Parallel? Verification?</thinking>
```
Run:
```
$ /create-prompt "Your task here"
Claude: Clarify? No. Generated ./prompts/001-task.txt
1. Run 2. Edit 3. Save
$ 1
```
**run-prompt**:
```
Execute prompts from ./prompts/ as subagents...
```
Output:
```
Subagent launched: Reading 001-task.txt...
[6 min later] Files created: src/audio.js etc.
Verification: npm start ran 30s ✓
Moved to ./completed/
```

Test in empty dir:
```
mkdir music-cli && cd music-cli
npm init -y
/create-prompt "Generative music CLI as above"
```

## Breakdown (show the math)

Time: Create (2 min analysis + XML gen) + Run (6 min impl) = 8 min. Direct: 1 min + 20 min fixes (5 clarifs × 2 min + 10 bugs × 1 min) = 21 min. Net save: 13 min/task.

LOC efficiency: Direct 286 / 1 file = 286/file. Meta: 318 total / 4 files = 79.5/file (72% modular gain).

Subagent parallelism: 3 tasks (audio/visuals/entry) × 2 min serial = 6 min; parallel = 2 min (demo averaged 6 min with deps).

Token est. (Claude 3.5 Sonnet): Direct prompt: 500 tokens. Meta: Create 2k (thinking) + Run 3k fresh = 5k total (but isolated).

## Limitations (be honest)

- Overhead: 8 min vs. 10s for trivial (e.g., "Change CSS background: yellow → red").
- Clarification loops: Vague input ("build dashboard") → 3-5 questions (demo: 5 for direct).
- No auto-benchmarks; verification manual ("run 30s").
- Subagents: Max 3-4 parallel (context limits); sequential for deps adds 2x time.
- Current Claude only: No Opus native; XML from Anthropic docs (2024).

Source: Tash Teaches transcript demo (timestamps 12:30 initial, 22:45 enhancement). File LOC via `wc -l`; times screen-recorded. Apples-to-apples: Identical requests, same Node/Blessed/speaker stack.