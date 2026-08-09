# Fanti

Client-side React app built with Bun, Rsbuild, TanStack Router, Tailwind CSS, and shadcn Base UI.

## Stack

- Bun package manager
- Rsbuild React app
- TanStack Router file-based routes
- Tailwind CSS v4
- shadcn Base UI Imperial archive theme (`base-nova`, stone tokens)
- All default shadcn components in `src/components/ui`
- Biome and tsgo

## Commands

```bash
bun install
bun run dev
bun run lint
bun run type:check
bun run build
```

Regenerate TanStack routes after route changes:

```bash
bun run generate:routes
```
