---
layout: post
title: "Running GPTCode Completely Offline with Ollama"
date: 2025-11-17
author: Jader Correa
description: "Run GPTCode completely offline with Ollama. Your code never leaves your machine. Zero cost per token, full privacy, no network latency."
tags: [configuration, ollama, local, privacy, offline]
---

# Running GPTCode Completely Offline with Ollama

Want to use GPTCode without sending your code to the cloud? Ollama lets you run powerful LLMs locally on your machine, completely free and private.

## Why Run Local Models?

- **Privacy**: Your code never leaves your machine
- **Cost**: $0 per token - run unlimited queries
- **Speed**: No network latency for small models
- **Offline**: Work anywhere, even without internet
- **Control**: Full control over model versions and updates

## Prerequisites

1. Install Ollama: https://ollama.com/download
2. Have at least 8GB RAM (16GB+ recommended for larger models)
3. SSD storage (models can be 4-30GB each)

## Recommended Model Configurations

### Balanced Setup (16GB RAM)

Best all-around configuration for most machines:

```yaml
backend:
  ollama:
    type: ollama
    base_url: http://localhost:11434
    default_model: llama3.1:8b
    agent_models:
      router: llama3.1:8b         # Fast routing
      query: gpt-oss:latest       # 20B for comprehension
      editor: qwen3-coder:latest  # Specialized for code
      research: gpt-oss:latest    # Good at synthesis
```

**Required models:**
```bash
ollama pull llama3.1:8b        # ~4.7GB
ollama pull gpt-oss:latest     # ~13GB
ollama pull qwen3-coder:latest # ~18GB
```

**Total storage**: ~36GB

### Performance Setup (32GB+ RAM)

For powerful machines that can run larger models:

```yaml
backend:
  ollama:
    type: ollama
    base_url: http://localhost:11434
    default_model: deepseek-r1:32b
    agent_models:
      router: llama3.1:8b         # Fast routing
      query: deepseek-r1:32b      # Strong reasoning
      editor: qwen3-coder:latest  # Code specialist
      research: deepseek-r1:32b   # Excellent research
```

**Required models:**
```bash
ollama pull llama3.1:8b
ollama pull deepseek-r1:32b    # ~20GB
ollama pull qwen3-coder:latest
```

**Total storage**: ~43GB

### Minimal Setup (8GB RAM)

For resource-constrained machines:

```yaml
backend:
  ollama:
    type: ollama
    base_url: http://localhost:11434
    default_model: llama3.1:8b
    agent_models:
      router: llama3.1:8b
      query: llama3.1:8b
      editor: llama3.1:8b
      research: llama3.1:8b
```

**Required models:**
```bash
ollama pull llama3.1:8b
```

**Total storage**: ~4.7GB

## Model Recommendations by Task

### Router (Intent Classification)
- **Best**: `llama3.1:8b` - Fast and accurate enough
- **Alternative**: `phi3:mini` - Even smaller/faster

### Query (Code Analysis)
- **Best**: `deepseek-r1:32b` - Excellent reasoning
- **Good**: `gpt-oss:latest` - Strong comprehension
- **Budget**: `llama3.1:8b` - Decent understanding

### Editor (Code Generation)
- **Best**: `qwen3-coder:latest` - Specialized for code (30B MoE)
- **Good**: `deepseek-coder-v2:latest` - Strong coding model
- **Budget**: `codellama:13b` - Decent code generation

### Research (Information Synthesis)
- **Best**: `deepseek-r1:32b` - Excellent at reasoning
- **Good**: `gpt-oss:latest` - Good synthesis
- **Budget**: `llama3.1:8b` - Basic research

## Popular Ollama Models for Development

| Model | Size | RAM | Best For | Quantization |
|-------|------|-----|----------|--------------|
| llama3.1:8b | 4.7GB | 8GB | Fast, general use | Q4_K_M |
| phi3:mini | 2.3GB | 4GB | Extremely fast routing | Q4_K_M |
| gpt-oss:latest | 13GB | 16GB | Comprehension, analysis | MXFP4 |
| qwen3-coder:latest | 18GB | 20GB | Code generation | Q4_K_M |
| deepseek-r1:32b | 20GB | 24GB | Reasoning, research | Q4_K_M |
| deepseek-coder-v2 | 16GB | 18GB | Code-focused tasks | Q4_K_M |
| codellama:13b | 7.4GB | 12GB | Budget code generation | Q4_K_M |

## Setting Up

1. **Pull your chosen models:**
```bash
ollama pull llama3.1:8b
ollama pull gpt-oss:latest
ollama pull qwen3-coder:latest
```

2. **Update GPTCode's model catalog:**
```bash
gt models update
```

This will automatically detect installed Ollama models.

3. **Configure in Neovim:**
```
Ctrl+X (in chat buffer)
Select ollama backend
Configure agent models
```

4. **Or edit `~/.gptcode/setup.yaml` directly**

## Performance Tips

### Speed Up Inference

1. **Use quantized models**: Ollama defaults to Q4_K_M (good balance)
2. **Keep models in RAM**: First run loads model, subsequent runs are fast
3. **Use smaller models for router**: Router is called most frequently
4. **SSD storage**: Models load much faster from SSD

### Memory Management

```bash
# Check running models
ollama ps

# Stop a model to free memory
ollama stop llama3.1:8b

# Models auto-unload after 5 minutes of inactivity
```

### Parallel Model Loading

Ollama can run multiple models simultaneously if you have enough RAM:

```bash
# In separate terminals
ollama run llama3.1:8b
ollama run qwen3-coder:latest
```

## Switching Between Local and Cloud

You can configure multiple backends and switch between them as needed:

**Example setup with both Ollama and Groq** (`~/.gptcode/setup.yaml`):
```yaml
defaults:
  backend: ollama  # Currently active backend

backend:
  ollama:
    type: ollama
    base_url: http://localhost:11434
    default_model: llama3.1:8b
    agent_models:
      router: llama3.1:8b
      query: qwen3-coder:latest
      editor: qwen3-coder:latest
      research: llama3.1:8b
  
  groq:
    type: openai
    base_url: https://api.groq.com/openai/v1
    default_model: gpt-oss-120b-128k
    agent_models:
      router: llama-3.1-8b-instant
      query: gpt-oss-120b-128k
      editor: deepseek-r1-distill-qwen-32b
      research: gpt-oss-120b-128k
```

**To switch backends:**
1. Via CLI: `gt backend use ollama` or `gt backend use groq`
2. In Neovim: Press `Ctrl+X` in chat buffer and select different backend

**Important**: Only one backend is active at a time. Each backend has its own set of agent_models. You cannot mix models from different backends in the same session.

**When to use which:**
- **Ollama**: Privacy-sensitive code, unlimited usage, working offline
- **Groq**: Need faster inference, larger context, better quality for critical tasks

See our [Hybrid Cloud/Local guide](2025-11-18-hybrid-cloud-local) for detailed switching strategies.

## Troubleshooting

### Model Loading Slowly
- Check if you have enough RAM: `ollama ps`
- Ensure model is on SSD, not HDD
- Close other applications to free memory

### Out of Memory
- Use smaller models or quantizations
- Run one model at a time
- Increase swap space (not recommended for performance)

### Poor Quality Responses
- Try larger models (requires more RAM)
- Use specialized models (e.g., qwen3-coder for code)
- Check model quantization (Q4_K_M is good balance)

## Comparing Local vs Cloud

| Aspect | Ollama (Local) | Groq (Cloud) |
|--------|----------------|--------------|
| Privacy | Complete | Code sent to API |
| Cost | Free | Pay per token |
| Speed (first run) | Model load time | Instant |
| Speed (loaded) | No network | Very fast |
| Model quality | Limited by RAM | Largest models |
| Offline | Works offline | Requires internet |
| Setup | Download models | Just API key |

## Model Discovery and Installation

GPTCode includes built-in model discovery and installation for Ollama:

### Search for Models

```bash
# Search all ollama models
gt models search -b ollama

# Search with filters (ANDed together)
gt models search ollama coding fast
gt models search ollama llama3
```

The search results include an `installed` field showing which models are already available:

```json
{
  "id": "llama3.1:8b",
  "name": "llama3.1:8b",
  "tags": ["free", "fast", "versatile"],
  "context_window": 8192,
  "installed": true
}
```

### Install Models

```bash
# Install a specific model
gt models install llama3.1:8b

# If already installed, you'll see:
# ✓ Model llama3.1:8b already installed
```

### Discover New Models

For the full catalog of available models, visit [ollama.com/library](https://ollama.com/library).

Update GPTCode's model catalog periodically:
```bash
gt models update
```

## Community Recommendations

Share your Ollama configuration on [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues) and help others find the best setup for their hardware!

---

*Running into issues? Ask in [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*
