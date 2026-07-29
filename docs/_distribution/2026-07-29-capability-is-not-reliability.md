# Capability Is Not Reliability

Canonical paper:
https://gptcode.dev/blog/2026-07-29-one-successful-agent-run-proves-almost-nothing

Technical brief:
https://gptcode.dev/blog/2026-07-29-capability-is-not-reliability

Evidence release:
https://github.com/jadercorrea/ai-experiments/releases/tag/2026.07.29.1

## LinkedIn

I watched a local coding agent solve a security-sensitive task.

It changed one Go file, preserved the public API, passed the tests under the
race detector, and passed static analysis. No hosted model was used.

Then I ran the same configuration three more times.

It failed every run.

That experiment reinforced a distinction that is easy to miss when evaluating
AI systems:

Capability is not reliability.

One successful run proves that a system can produce a result. It tells us very
little about how often it will produce that result under controlled repetition.

The first campaign demonstrated bounded capability: 1/1 passed in 26 minutes.
The subsequent repeatability campaign was 0/3, with every run reaching the
30-minute timeout.

The failed runs were not noise to discard. They exposed different failures at
the security boundary and defects in the surrounding agent harness: hidden
progress, mismatched timeouts, wasteful retries, and incorrect final status.

I published the full protocol, patches, command logs, snapshots, verification
results, limitations, and checksum as an immutable release.

Engineering decisions should depend on repeated, inspectable evidence, not the
most memorable demonstration.

One successful run demonstrates possibility. Engineering depends on
repeatability.

Full paper:
https://gptcode.dev/blog/2026-07-29-one-successful-agent-run-proves-almost-nothing

Raw evidence:
https://github.com/jadercorrea/ai-experiments/releases/tag/2026.07.29.1

## X thread

1/10

I watched a local coding agent solve a security-sensitive Go task. It preserved
the public API, passed `go test -race`, passed `go vet`, and used no paid API.

Then I ran the same configuration three more times.

It failed every run.

2/10

The lesson was not that local models are useless.

It was that capability is not reliability.

One successful run proves a system can produce a result. It does not tell us
how often it will.

3/10

The first campaign was a capability check:

1/1 passed in 26m08s.

The subsequent repeatability campaign:

0/3 passed. Every run timed out at roughly 30 minutes.

These campaigns answer different questions and should not be combined into an
artificial 1/4 score.

4/10

The task enforced filesystem containment:

- preserve legitimate nested paths;
- reject traversal and absolute paths;
- reject a pre-existing symlink escape;
- preserve the public API;
- pass the race detector and static analysis.

5/10

An earlier model “passed” by rejecting every filename containing `..`.

That also rejected the legitimate name `v1..v2.txt`.

The agent had not solved traversal. It had prohibited a substring.

Better evaluation made the model look worse and the evidence more truthful.

6/10

The failed runs mattered.

Two rejected legitimate nested paths. Another accepted an absolute path and a
symlink escape. Some retries regressed cases that had already passed.

Those artifacts explain more than an aggregate score.

7/10

The experiment also found defects in the agent system:

- a provider timeout shorter than the suite budget;
- progress hidden until completion;
- retries repeating equivalent failures;
- final status contradicting successful verification.

8/10

Fixing those defects did not make the model smarter.

It made the system more observable, economical, and honest when the model was
not capable enough.

9/10

A useful agent evaluation retains the initial state, task, configuration,
commands, patch, failed runs, verification output, final snapshot, duration,
and human-review boundary.

The unit of evidence is the restorable run, not the selected transcript.

10/10

I published the complete paper and immutable 2026.07.29.1 evidence release.

One successful run demonstrates possibility.

Engineering depends on repeatability.

https://gptcode.dev/blog/2026-07-29-one-successful-agent-run-proves-almost-nothing

## Hacker News

Suggested title:

One Successful Agent Run Proves Almost Nothing

Submission URL:

https://gptcode.dev/blog/2026-07-29-one-successful-agent-run-proves-almost-nothing

First comment:

Author here. I ran a local coding model against a small but security-sensitive
filesystem-containment fixture. It passed one capability check, then timed out
in all three runs of a subsequent repeatability campaign.

The article is about the evaluation method rather than the model: clean
workspaces, required failing baselines, retained failed patches, deterministic
verification, final snapshots, human-review boundaries, and why capability and
reliability are different claims.

The raw evidence is published in the immutable `2026.07.29.1` release:
https://github.com/jadercorrea/ai-experiments/releases/tag/2026.07.29.1

The main limitation is also documented: the historical runner came from a
precursor working tree whose exact transient build commit was not retained.
Future campaigns require a clean immutable harness revision before execution.

I would be especially interested in how others predeclare agent evaluations,
retain negative runs, and decide when a local model should escalate to a hosted
one.

## Five-minute video

### 0:00–0:35 — Hook

I watched a local coding agent solve a security-sensitive task. It changed one
Go file, preserved the public API, passed the Go race detector, and passed
static analysis.

Then I ran the same configuration three more times. It failed every run.

That is the difference between capability and reliability.

### 0:35–1:15 — The task

The fixture was a small Go file store. Legitimate nested files had to work, but
absolute paths, parent traversal, and escape through a pre-existing symlink had
to fail.

It also contained a false-positive guard: `v1..v2.txt` is a legitimate
filename. An earlier model looked successful only because it rejected every
name containing two adjacent dots.

The contract had to distinguish a real solution from a plausible shortcut.

### 1:15–2:05 — The result

With Devstral Small 2 running through Ollama at a 16,384-token context, the
capability check passed in 26 minutes and 8 seconds.

The same configuration then entered a separate repeatability campaign. Three
fresh workspaces, the same task, the same model profile, the same verification
commands, and the same 30-minute budget.

All three runs timed out.

Show the result figure here.

The first campaign proved possibility. The second did not support a reliability
claim.

### 2:05–3:00 — What failures revealed

The failed runs were valuable. Two rejected legitimate nested paths. Another
accepted an absolute path and a symlink escape. Some retries regressed
previously passing behavior.

The experiment also exposed defects in the agent harness: an internal timeout
shorter than the suite budget, no visible progress during long runs, repeated
equivalent failures, and a final status that could contradict successful
verification.

Fixing these problems did not improve the model. It improved the truthfulness
of the system around it.

### 3:00–4:00 — What counts as evidence

For agent evaluation, I now want the initial repository, exact task and
configuration, clean workspace, command stream, patch, deterministic
verification, final snapshot, duration, failed runs, and human-review boundary.

Tests establish truth only within the encoded contract. The successful patch
still retained a possible check-to-write race against an actively hostile
concurrent process. That limitation belongs in the result.

### 4:00–4:40 — The local inference boundary

Local AI may follow the path of recording tools. Much of the work that once
required a professional studio can now be done well at home. Studios remain
important for specialized work, but they are no longer mandatory for every
recording.

Likewise, frontier models and managed inference will remain essential, but not
every bounded engineering task should necessarily require them.

The system should try locally, verify deterministically, and escalate structured
evidence when progress plateaus.

### 4:40–5:00 — Close

The complete experiment, including failed patches and snapshots, is published
under release `2026.07.29.1`.

One successful run demonstrates possibility.

Engineering depends on repeatability.
