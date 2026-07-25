# GPTCode CLI Docker Image

Official Docker image for GPTCode CLI - your AI coding assistant.

## Quick Start

```bash
# Pull the image
docker pull gptcode/cli:latest

# Run with your token
docker run -e GPTCODE_TOKEN=$GPTCODE_TOKEN gptcode/cli:latest gt chat "explain Docker"
```

## Usage in CI/CD

### GitHub Actions

```yaml
jobs:
  ai-review:
    runs-on: ubuntu-latest
    container:
      image: gptcode/cli:latest
    steps:
      - uses: actions/checkout@v4
      
      - name: AI Code Review
        env:
          GPTCODE_TOKEN: ${{ secrets.GPTCODE_TOKEN }}
        run: gt review --ci
```

### GitLab CI

```yaml
code-review:
  image: gptcode/cli:latest
  script:
    - gt review --ci
  variables:
    GPTCODE_TOKEN: $GPTCODE_TOKEN
```

### Jenkins Pipeline

```groovy
pipeline {
    agent {
        docker { image 'gptcode/cli:latest' }
    }
    environment {
        GPTCODE_TOKEN = credentials('gptcode-token')
    }
    stages {
        stage('AI Review') {
            steps {
                sh 'gt review --ci'
            }
        }
    }
}
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GPTCODE_TOKEN` | Authentication token from gptcode.app | Yes |
| `GPTCODE_BACKEND` | LLM backend (openai, anthropic, etc.) | No |
| `GPTCODE_MODEL` | Model to use | No |

## Volume Mounts

For persistent configuration:

```bash
docker run -v ~/.gptcode:/home/gptcode/.gptcode \
           -v $(pwd):/workspace \
           -w /workspace \
           gptcode/cli:latest gt do "fix the tests"
```

## Available Tags

- `latest` - Latest stable release
- `v1.x.x` - Specific version
- `main` - Latest from main branch (unstable)

## Image Details

- **Base**: Alpine Linux 3.19
- **Size**: ~50MB
- **Platforms**: linux/amd64, linux/arm64
- **User**: Non-root (uid 1000)

## Links

- [GitHub Repository](https://github.com/jadercorrea/gptcode)
- [Documentation](https://gptcode.app/docs)
- [Get Your Token](https://gptcode.app/login)
