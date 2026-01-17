# ViewRA Frontend Coding Style Guide

## Overview

This document defines the coding standards for the ViewRA frontend codebase. Following these conventions ensures consistency, readability, and maintainability across the project.

## Function Declarations

### Rule: Use Const Arrow Functions

**Always use const arrow functions** instead of traditional function declarations.

✅ **CORRECT:**
```typescript
const MyComponent = ({ title }: Props) => {
  return <h1>{title}</h1>
}

const handleClick = () => {
  console.log('clicked')
}

const calculateTotal = (items: Item[]) => {
  return items.reduce((sum, item) => sum + item.price, 0)
}
```

❌ **INCORRECT:**
```typescript
function MyComponent({ title }: Props) {
  return <h1>{title}</h1>
}

function handleClick() {
  console.log('clicked')
}

function calculateTotal(items: Item[]) {
  return items.reduce((sum, item) => sum + item.price, 0)
}
```

### Exceptions

The only exception is for type definitions and interfaces:

```typescript
interface Props {
  title: string
}

type Status = 'pending' | 'success' | 'error'
```

### Why Arrow Functions?

1. **Consistency**: Same syntax everywhere
2. **Lexical `this`**: Automatic binding (no `.bind()` needed)
3. **Modern Standard**: Aligns with contemporary JavaScript/TypeScript practices
4. **Conciseness**: More compact syntax
5. **Enforceability**: Easy to lint

## Export Organization

### Rule: Exports at End of File

**Always place exports at the end of the file** (except for type-only exports which can be inline).

✅ **CORRECT:**
```typescript
// 1. Imports
import { useState } from 'react'
import { Button } from '@/components/ui'

// 2. Types
interface ComponentProps {
  title: string
}

// 3. Component
const MyComponent = ({ title }: ComponentProps) => {
  return <div>{title}</div>
}

// 4. Helpers
const helperFn = () => {
  return true
}

// 5. Exports (END OF FILE)
export { MyComponent, helperFn }
export type { ComponentProps }
```

❌ **INCORRECT:**
```typescript
import { useState } from 'react'

export interface ComponentProps {
  title: string
}

export const MyComponent = ({ title }: ComponentProps) => {
  return <div>{title}</div>
}

export const helperFn = () => {
  return true
}
```

### Exceptions

1. **Type-only exports** can remain inline: `export type Props = {...}`
2. **Default exports** can remain inline for single-component files
3. **TanStack Router** route exports: `export const Route = createFileRoute(...)`

### Why Exports at End?

1. **Clear API Surface**: Easy to see what's exported vs internal
2. **Better Tree-Shaking**: Bundlers can optimize better
3. **Refactoring**: Simple to convert internal functions to exported
4. **Readability**: Clear separation between implementation and interface
5. **Module Pattern**: Follows modern ES module best practices

## File Structure

### Standard Component File Order

```typescript
// 1. Imports
import { useState } from 'react'
import { Button } from '@/components/ui'
import type { ReactNode } from 'react'

// 2. Type Definitions
interface ComponentProps {
  title: string
  children?: ReactNode
}

interface State {
  count: number
}

// 3. Constants
const DEFAULT_TITLE = 'Untitled'
const MAX_COUNT = 100

// 4. Main Component
const MyComponent = ({ title, children }: ComponentProps) => {
  const [count, setCount] = useState(0)

  const handleIncrement = () => {
    setCount(prev => Math.min(prev + 1, MAX_COUNT))
  }

  return (
    <div>
      <h1>{title}</h1>
      <Button onClick={handleIncrement}>{count}</Button>
      {children}
    </div>
  )
}

// 5. Sub-components (if any)
const SubComponent = () => {
  return <span>Sub</span>
}

// 6. Utility Functions
const formatCount = (n: number) => {
  return n.toString().padStart(2, '0')
}

// 7. Exports
export { MyComponent, SubComponent }
export type { ComponentProps }
```

## Additional Style Rules

### Naming Conventions

- **Components**: PascalCase (`MyComponent`, `UserProfile`)
- **Functions/Variables**: camelCase (`handleClick`, `userData`)
- **Constants**: UPPER_SNAKE_CASE for true constants (`API_URL`, `MAX_RETRY`)
- **Types/Interfaces**: PascalCase (`UserData`, `ApiResponse`)
- **File Names**: Match export name (component files) or kebab-case (utilities)

### Import Organization

Group imports in this order:

1. External dependencies (React, third-party)
2. Internal absolute imports (`@/components`, `@/lib`)
3. Relative imports
4. Type imports (at the end or inline)

```typescript
// 1. External
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'

// 2. Internal absolute
import { Button } from '@/components/ui'
import { useAuth } from '@/hooks'

// 3. Relative
import { helper } from './utils'

// 4. Types
import type { User } from '@/types'
```

### TypeScript

- Always use explicit types for function parameters
- Let TypeScript infer return types (unless complex)
- Use `interface` for object shapes, `type` for unions/intersections
- Prefer `type` imports: `import type { User } from '@/types'`

### React

- Use functional components (const arrow functions)
- Use hooks (not class components)
- Destructure props in function signature
- Use explicit prop types

### Code Quality

- No `any` types (use `unknown` if truly unknown)
- No unused variables (prefix with `_` if required by API)
- Prefer `const` over `let`
- Never use `var`
- Always use `===` (not `==`)
- Always use curly braces for conditionals
- Prefer template literals over string concatenation

## ESLint Configuration

These rules are enforced by ESLint:

```javascript
{
  'prefer-arrow-callback': 'error',
  'func-style': ['error', 'expression', { allowArrowFunctions: true }],
  'prefer-const': 'error',
  'no-var': 'error',
  'eqeqeq': ['error', 'always'],
  'curly': ['error', 'all'],
  'prefer-template': 'warn',
}
```

## Migration Guide

When updating existing code:

1. Convert `function` declarations to `const` arrow functions
2. Move exports to end of file
3. Run `npm run lint` to catch violations
4. Fix any remaining issues

### Before:
```typescript
export function MyComponent({ title }: Props) {
  return <h1>{title}</h1>
}

export function helperFn() {
  return true
}
```

### After:
```typescript
const MyComponent = ({ title }: Props) => {
  return <h1>{title}</h1>
}

const helperFn = () => {
  return true
}

export { MyComponent, helperFn }
```

## Benefits

1. **Consistency**: Same patterns across entire codebase
2. **Modern**: Follows current JavaScript/TypeScript best practices
3. **Maintainability**: Easy to read and refactor
4. **Quality**: Enforced by linting tools
5. **Team**: Clear expectations for all contributors

## References

- [CONVENTIONS.md](../../docs/development/CONVENTIONS.md) - Full coding guidelines
- [ESLint Config](../eslint.config.js) - Linting rules
- [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html)
- [React Best Practices](https://react.dev/learn)
