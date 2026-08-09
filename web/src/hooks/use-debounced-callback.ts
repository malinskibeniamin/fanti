import { useEffect, useRef } from 'react';

interface DebounceOptions {
  flushOnUnmount?: boolean;
}

export interface DebouncedCallback<Args extends unknown[], Result> {
  (...args: Args): void;
  cancel: () => void;
  flush: () => Result | undefined;
}

/** Delays calls until quiet, with explicit flush/cancel lifecycle control. */
export function useDebouncedCallback<Args extends unknown[], Result>(
  callback: (...args: Args) => Result,
  delayMs: number,
  options: DebounceOptions = {},
): DebouncedCallback<Args, Result> {
  const callbackRef = useRef(callback);
  callbackRef.current = callback;
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const pendingArgsRef = useRef<Args>(undefined);
  const flushOnUnmountRef = useRef(options.flushOnUnmount ?? false);
  flushOnUnmountRef.current = options.flushOnUnmount ?? false;

  function cancel() {
    clearTimeout(timerRef.current);
    timerRef.current = undefined;
    pendingArgsRef.current = undefined;
  }

  function flush(): Result | undefined {
    const pendingArgs = pendingArgsRef.current;
    if (!pendingArgs) {
      return undefined;
    }

    clearTimeout(timerRef.current);
    timerRef.current = undefined;
    pendingArgsRef.current = undefined;

    return callbackRef.current(...pendingArgs);
  }

  useEffect(function settlePendingOnUnmount() {
    return () => {
      clearTimeout(timerRef.current);
      const pendingArgs = pendingArgsRef.current;
      pendingArgsRef.current = undefined;
      if (flushOnUnmountRef.current && pendingArgs) {
        callbackRef.current(...pendingArgs);
      }
    };
  }, []);

  function schedule(...args: Args) {
    clearTimeout(timerRef.current);
    pendingArgsRef.current = args;
    timerRef.current = setTimeout(() => {
      flush();
    }, delayMs);
  }

  return Object.assign(schedule, { cancel, flush });
}
