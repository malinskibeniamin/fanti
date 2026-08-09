import { useRouterState } from '@tanstack/react-router';

export const TAB_PATHS = ['/', '/convert', '/study', '/discover'] as const;

/** Shell chrome mode derived from the current location. */
export function useShellState() {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  });
  const normalized =
    pathname.length > 1 && pathname.endsWith('/')
      ? pathname.slice(0, -1)
      : pathname;
  const isTabScreen =
    (TAB_PATHS as ReadonlyArray<string>).includes(normalized) ||
    normalized === '/components';
  const isReader = normalized.startsWith('/read/');
  return { pathname: normalized, isTabScreen, isReader };
}
