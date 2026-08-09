import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { useDebouncedCallback } from '@/hooks/use-debounced-callback';

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

test('fires only the latest call after the quiet period', () => {
  const spy = vi.fn();
  const { result } = renderHook(() => useDebouncedCallback(spy, 500));

  act(() => {
    result.current('first');
    result.current('second');
  });
  expect(spy).not.toHaveBeenCalled();

  act(() => {
    vi.advanceTimersByTime(500);
  });
  expect(spy).toHaveBeenCalledTimes(1);
  expect(spy).toHaveBeenCalledWith('second');
});

test('cancels the pending call on unmount', () => {
  const spy = vi.fn();
  const { result, unmount } = renderHook(() => useDebouncedCallback(spy, 500));

  act(() => {
    result.current('pending');
  });
  unmount();
  act(() => {
    vi.advanceTimersByTime(1000);
  });
  expect(spy).not.toHaveBeenCalled();
});

test('flushes the latest pending call immediately', () => {
  const spy = vi.fn((_value: string) => 'saved');
  const { result } = renderHook(() => useDebouncedCallback(spy, 500));

  act(() => {
    result.current('pending');
  });

  let flushed: string | undefined;
  act(() => {
    flushed = result.current.flush();
  });

  expect(flushed).toBe('saved');
  expect(spy).toHaveBeenCalledOnce();

  act(() => {
    vi.advanceTimersByTime(500);
  });
  expect(spy).toHaveBeenCalledOnce();
});

test('can flush a pending save on unmount', () => {
  const spy = vi.fn();
  const { result, unmount } = renderHook(() =>
    useDebouncedCallback(spy, 500, { flushOnUnmount: true }),
  );

  act(() => {
    result.current('latest');
  });
  unmount();

  expect(spy).toHaveBeenCalledWith('latest');
});
