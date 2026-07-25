---
layout: skill
title: Security
name: security
category: ops
description: Write secure code following OWASP guidelines. Input validation, authentication, authorization, and secure coding patterns.
---

# Security

Guidelines for writing secure code and preventing common vulnerabilities.

## When to Activate

- Handling user input
- Implementing authentication/authorization
- Working with sensitive data
- Building APIs or web applications

## Input Validation

### Validate all inputs at boundaries

```typescript
// GOOD - explicit validation
function createUser(input: unknown): User {
  const parsed = userSchema.safeParse(input);
  if (!parsed.success) {
    throw new ValidationError(parsed.error);
  }
  return parsed.data;
}

// Use Zod for runtime validation
const userSchema = z.object({
  email: z.string().email(),
  password: z.string().min(12),
  age: z.number().int().positive().max(150),
});

// BAD - trusting input
function createUser(input: any): User {
  return { email: input.email, password: input.password };
}
```

### Sanitize for output context

```typescript
// HTML context - escape HTML entities
function renderComment(text: string): string {
  return escapeHtml(text);
}

// SQL context - use parameterized queries
const user = await db.query(
  'SELECT * FROM users WHERE email = $1',
  [email]  // NEVER interpolate
);

// URL context - encode components
const url = `/search?q=${encodeURIComponent(query)}`;
```

## Authentication

### Password handling

```typescript
import bcrypt from 'bcrypt';

// GOOD - bcrypt with sufficient rounds
const SALT_ROUNDS = 12;

async function hashPassword(password: string): Promise<string> {
  return bcrypt.hash(password, SALT_ROUNDS);
}

async function verifyPassword(password: string, hash: string): Promise<boolean> {
  return bcrypt.compare(password, hash);
}

// BAD - weak hashing
const hash = crypto.createHash('md5').update(password).digest('hex');
```

### Token generation

```typescript
import crypto from 'crypto';

// GOOD - cryptographically secure random tokens
function generateToken(): string {
  return crypto.randomBytes(32).toString('hex');
}

function generateSessionId(): string {
  return crypto.randomUUID();
}

// BAD - predictable tokens
function generateToken(): string {
  return Date.now().toString();
}
```

### JWT best practices

```typescript
import jwt from 'jsonwebtoken';

const JWT_OPTIONS = {
  algorithm: 'RS256',  // Use asymmetric
  expiresIn: '15m',    // Short-lived
  issuer: 'your-app',
};

// Always verify issuer, audience, expiry
function verifyToken(token: string): JwtPayload {
  return jwt.verify(token, publicKey, {
    algorithms: ['RS256'],
    issuer: 'your-app',
    complete: true,
  });
}
```

## Authorization

### Principle of least privilege

```typescript
// GOOD - check permissions explicitly
async function deletePost(userId: string, postId: string) {
  const post = await posts.findById(postId);
  
  // Check ownership or admin
  if (post.authorId !== userId && !user.isAdmin) {
    throw new ForbiddenError('Not authorized to delete this post');
  }
  
  await posts.delete(postId);
}

// BAD - missing authorization
async function deletePost(postId: string) {
  await posts.delete(postId);  // Anyone can delete!
}
```

### RBAC patterns

```typescript
const permissions = {
  admin: ['read', 'write', 'delete', 'manage_users'],
  editor: ['read', 'write'],
  viewer: ['read'],
};

function can(role: Role, action: string): boolean {
  return permissions[role]?.includes(action) ?? false;
}

// Middleware
function requirePermission(action: string) {
  return (req, res, next) => {
    if (!can(req.user.role, action)) {
      return res.status(403).json({ error: 'Forbidden' });
    }
    next();
  };
}
```

## SQL Injection Prevention

### Always use parameterized queries

```typescript
// GOOD - parameterized
const users = await db.query(
  'SELECT * FROM users WHERE email = $1 AND status = $2',
  [email, 'active']
);

// GOOD - ORM with safe methods
const user = await prisma.user.findUnique({
  where: { email },
});

// BAD - string interpolation
const users = await db.query(
  `SELECT * FROM users WHERE email = '${email}'`  // SQL INJECTION!
);
```

## XSS Prevention

### Content Security Policy

```typescript
// Express middleware
app.use((req, res, next) => {
  res.setHeader(
    'Content-Security-Policy',
    "default-src 'self'; " +
    "script-src 'self' 'nonce-abc123'; " +
    "style-src 'self' 'unsafe-inline'; " +
    "img-src 'self' data: https:;"
  );
  next();
});
```

### React - dangerouslySetInnerHTML

{% raw %}
```tsx
// GOOD - sanitize before rendering
import DOMPurify from 'dompurify';

function RichContent({ html }: { html: string }) {
  return (
    <div 
      dangerouslySetInnerHTML={{ 
        __html: DOMPurify.sanitize(html) 
      }} 
    />
  );
}

// BAD - raw HTML
<div dangerouslySetInnerHTML={{ __html: userContent }} />
```
{% endraw %}

## CSRF Protection

### SameSite cookies + CSRF tokens

```typescript
// Cookie settings
res.cookie('session', token, {
  httpOnly: true,
  secure: true,
  sameSite: 'strict',
  maxAge: 3600000,
});

// CSRF token in forms
app.use(csrf());

// Verify on mutations
app.post('/api/transfer', csrfProtection, (req, res) => {
  // Token verified by middleware
});
```

## Rate Limiting

### Protect against brute force

```typescript
import rateLimit from 'express-rate-limit';

const loginLimiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 5, // 5 attempts
  message: 'Too many login attempts, try again later',
  standardHeaders: true,
  legacyHeaders: false,
});

app.post('/login', loginLimiter, loginHandler);
```

## Secrets Management

### Never hardcode secrets

```typescript
// GOOD - environment variables
const dbPassword = process.env.DATABASE_PASSWORD;

// GOOD - secrets manager
const secret = await secretsManager.getSecret('api-key');

// BAD - hardcoded
const apiKey = 'sk-1234567890abcdef';

// BAD - in code comments
// API key: sk-1234567890abcdef
```

### .gitignore for secrets

```gitignore
.env
.env.local
*.pem
*.key
secrets/
```

## Security Headers

```typescript
import helmet from 'helmet';

app.use(helmet({
  contentSecurityPolicy: true,
  crossOriginEmbedderPolicy: true,
  crossOriginOpenerPolicy: true,
  crossOriginResourcePolicy: true,
  hsts: { maxAge: 31536000 },
  noSniff: true,
  referrerPolicy: { policy: 'strict-origin-when-cross-origin' },
  xssFilter: true,
}));
```

## Logging Security Events

```typescript
// Log security events (without sensitive data)
logger.warn({
  event: 'login_failed',
  email: maskEmail(email),
  ip: req.ip,
  userAgent: req.headers['user-agent'],
  timestamp: new Date().toISOString(),
});

// Never log
// - Passwords
// - Tokens
// - Credit card numbers
// - Full SSN
```
