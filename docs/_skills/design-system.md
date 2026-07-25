---
layout: skill
title: Design System
name: design-system
category: design
description: Build consistent, scalable component libraries following Atomic Design principles and modern design system practices.
---

# Design System

Guidelines for building maintainable, scalable design systems and component libraries.

## When to Activate

- Creating or extending a design system
- Building Storybook stories
- Designing component APIs
- Working with design tokens

## Atomic Design Principles

### Structure components hierarchically

```
atoms/       → Button, Input, Icon, Text
molecules/   → SearchInput, FormField, Card
organisms/   → Header, ProductCard, CommentThread
templates/   → PageLayout, DashboardLayout
pages/       → HomePage, ProductPage
```

### Atoms: Single-purpose primitives

```tsx
// GOOD - focused, composable
interface ButtonProps {
  variant: 'primary' | 'secondary' | 'ghost';
  size: 'sm' | 'md' | 'lg';
  children: React.ReactNode;
  onClick?: () => void;
  disabled?: boolean;
}

// BAD - too many responsibilities
interface ButtonProps {
  text: string;
  icon?: string;
  iconPosition?: 'left' | 'right';
  loading?: boolean;
  loadingText?: string;
  tooltip?: string;
  // ... 20 more props
}
```

### Molecules: Meaningful combinations

```tsx
// GOOD - combines atoms with clear purpose
function SearchInput({ onSearch, placeholder }) {
  return (
    <div className="search-input">
      <Icon name="search" />
      <Input placeholder={placeholder} />
      <Button onClick={onSearch}>Search</Button>
    </div>
  );
}
```

## Design Tokens

### Use semantic naming

```css
/* GOOD - semantic tokens */
:root {
  /* Primitives (don't use directly in components) */
  --color-blue-500: #3b82f6;
  --color-gray-100: #f3f4f6;
  
  /* Semantic (use these in components) */
  --color-primary: var(--color-blue-500);
  --color-background: var(--color-gray-100);
  --color-text-primary: var(--color-gray-900);
  --color-text-secondary: var(--color-gray-600);
  
  /* Component-specific */
  --button-bg: var(--color-primary);
  --button-text: white;
}

/* BAD - magic values */
.button {
  background: #3b82f6;
  color: white;
}
```

### Define spacing scale

```css
:root {
  --space-1: 0.25rem;  /* 4px */
  --space-2: 0.5rem;   /* 8px */
  --space-3: 0.75rem;  /* 12px */
  --space-4: 1rem;     /* 16px */
  --space-6: 1.5rem;   /* 24px */
  --space-8: 2rem;     /* 32px */
}
```

## Component API Design

### Prefer composition over configuration

```tsx
// GOOD - composable
<Card>
  <Card.Header>
    <Card.Title>Product</Card.Title>
    <Card.Actions><Button>Edit</Button></Card.Actions>
  </Card.Header>
  <Card.Body>Content here</Card.Body>
</Card>

// BAD - configuration explosion
<Card
  title="Product"
  showActions
  actions={[{ label: 'Edit', onClick: handleEdit }]}
  headerVariant="large"
  bodyPadding="lg"
/>
```

### Use consistent prop patterns

```tsx
// Size: 'sm' | 'md' | 'lg'
// Variant: 'primary' | 'secondary' | 'ghost'
// State: disabled, loading, error

// GOOD - predictable API
<Button size="md" variant="primary" disabled />
<Input size="md" variant="primary" disabled />
<Select size="md" variant="primary" disabled />
```

## Storybook Best Practices

### Write comprehensive stories

{% raw %}
```tsx
// Button.stories.tsx
import type { Meta, StoryObj } from '@storybook/react';
import { Button } from './Button';

const meta: Meta<typeof Button> = {
  title: 'Atoms/Button',
  component: Button,
  tags: ['autodocs'],
  argTypes: {
    variant: {
      control: 'select',
      options: ['primary', 'secondary', 'ghost'],
    },
    size: {
      control: 'select', 
      options: ['sm', 'md', 'lg'],
    },
  },
};

export default meta;
type Story = StoryObj<typeof Button>;

export const Primary: Story = {
  args: {
    variant: 'primary',
    children: 'Click me',
  },
};

export const AllVariants: Story = {
  render: () => (
    <div style={{ display: 'flex', gap: '1rem' }}>
      <Button variant="primary">Primary</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="ghost">Ghost</Button>
    </div>
  ),
};
```
{% endraw %}

### Document with MDX

```mdx
{/* Button.mdx */}
import { Canvas, Meta, Story } from '@storybook/blocks';
import * as ButtonStories from './Button.stories';

<Meta of={ButtonStories} />

# Button

Buttons trigger actions. Use primary for main actions, secondary for alternatives.

## Usage Guidelines

- One primary button per section
- Use loading state for async actions
- Disable when action unavailable

<Canvas of={ButtonStories.Primary} />
```

## Accessibility

### Always include ARIA attributes

```tsx
// GOOD
<button
  aria-label="Close dialog"
  aria-pressed={isPressed}
  disabled={isDisabled}
>
  <Icon name="x" aria-hidden="true" />
</button>

// GOOD - form inputs
<label htmlFor="email">Email</label>
<input 
  id="email"
  type="email"
  aria-describedby="email-error"
  aria-invalid={hasError}
/>
{hasError && <span id="email-error">Invalid email</span>}
```

### Ensure keyboard navigation

```tsx
// GOOD - focusable, keyboard handlers
<div
  role="button"
  tabIndex={0}
  onClick={handleClick}
  onKeyDown={(e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      handleClick();
    }
  }}
>
  Custom Button
</div>
```

## File Structure

```
design-system/
├── tokens/
│   ├── colors.css
│   ├── spacing.css
│   └── typography.css
├── atoms/
│   ├── Button/
│   │   ├── Button.tsx
│   │   ├── Button.stories.tsx
│   │   ├── Button.test.tsx
│   │   └── index.ts
│   └── Input/
├── molecules/
├── organisms/
└── index.ts  # Public exports only
```
