# Project Rules

Use Bun. Typecheck with `bun run type:check` (`tsgo`). Lint and format with `bun run lint:fix` (Biome).

## UI

- React function components only.
- Tailwind utilities and shadcn theme tokens.
- Interactive UI from `@/components/ui`.
- Use TanStack Router `Link` for navigation.
- Route files live in `src/routes`; regenerate with `bun run generate:routes`.

## Quality

- No `as any` or `@ts-ignore`.
- Keep registry components in `src/components/ui` close to shadcn defaults.
- Run `bun run lint:fix`, `bun run type:check`, and `bun run build` before finishing.
