import type { CharacterJson } from 'hanzi-writer';

type StrokeDataResult = { data: CharacterJson; valid: true } | { valid: false };
type DrawnPoint = { x: number; y: number };

function parseDrawnPath(path: string): DrawnPoint[] | undefined {
  const tokens = path.trim().split(/\s+/);
  if (tokens.length < 3 || tokens.length % 3 !== 0) {
    return undefined;
  }

  const points: DrawnPoint[] = [];
  for (let index = 0; index < tokens.length; index += 3) {
    const command = tokens[index];
    const x = Number(tokens[index + 1]);
    const y = Number(tokens[index + 2]);
    if (
      command !== (index === 0 ? 'M' : 'L') ||
      !Number.isFinite(x) ||
      !Number.isFinite(y)
    ) {
      return undefined;
    }
    points.push({ x, y });
  }

  return points;
}

function isNumberPair(value: unknown): value is [number, number] {
  return (
    Array.isArray(value) &&
    value.length === 2 &&
    value.every(
      (coordinate) =>
        typeof coordinate === 'number' && Number.isFinite(coordinate),
    )
  );
}

function isCharacterJson(value: unknown): value is CharacterJson {
  if (
    typeof value !== 'object' ||
    value === null ||
    !('strokes' in value) ||
    !('medians' in value)
  ) {
    return false;
  }
  const { medians, strokes } = value;
  if (
    !Array.isArray(strokes) ||
    strokes.length === 0 ||
    !strokes.every(
      (stroke) => typeof stroke === 'string' && stroke.length > 0,
    ) ||
    !Array.isArray(medians) ||
    medians.length !== strokes.length ||
    !medians.every(
      (median) =>
        Array.isArray(median) &&
        median.length >= 2 &&
        median.every(isNumberPair),
    )
  ) {
    return false;
  }
  if (!('radStrokes' in value) || value.radStrokes === undefined) {
    return true;
  }
  return (
    Array.isArray(value.radStrokes) &&
    value.radStrokes.every(
      (strokeIndex) =>
        Number.isInteger(strokeIndex) &&
        strokeIndex >= 0 &&
        strokeIndex < strokes.length,
    )
  );
}

function parseStrokeData(data: string): StrokeDataResult {
  try {
    const parsed: unknown = JSON.parse(data);
    return isCharacterJson(parsed)
      ? { data: parsed, valid: true }
      : { valid: false };
  } catch {
    return { valid: false };
  }
}

export type { StrokeDataResult };
export { parseDrawnPath, parseStrokeData };
