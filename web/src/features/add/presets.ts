import type { ItemInput, Preset, PresetInput } from "../../api/types";

export interface PresetResult {
  next: ItemInput;
  /** Fields the preset fills in or adds to. */
  filled: (keyof PresetInput)[];
  /** Fields left as typed because they already had a value. */
  kept: (keyof PresetInput)[];
}

const SINGLE_FIELDS = ["room", "category", "condition", "notes"] as const;
const LIST_FIELDS = ["tags", "collections"] as const;

/**
 * Applies a preset without overwriting anything typed: empty single fields
 * are filled, and tags and collections are added to.
 */
export function applyPreset(input: ItemInput, preset: Preset): PresetResult {
  const next = { ...input };
  const filled: (keyof PresetInput)[] = [];
  const kept: (keyof PresetInput)[] = [];
  for (const field of SINGLE_FIELDS) {
    if (preset[field] === "") {
      continue;
    }
    if (input[field] === "") {
      Object.assign(next, { [field]: preset[field] });
      filled.push(field);
    } else if (input[field] !== preset[field]) {
      kept.push(field);
    }
  }
  for (const field of LIST_FIELDS) {
    const known = new Set(input[field].map((name) => name.toLowerCase()));
    const added = preset[field].filter(
      (name) => !known.has(name.toLowerCase()),
    );
    if (added.length > 0) {
      next[field] = [...input[field], ...added];
      filled.push(field);
    }
  }
  return { next, filled, kept };
}

/** The details of the current form a preset would keep. */
export function presetFromInput(name: string, input: ItemInput): PresetInput {
  return {
    name,
    room: input.room,
    category: input.category,
    tags: input.tags,
    collections: input.collections,
    condition: input.condition,
    notes: input.notes,
  };
}

export function hasPresetDetails(input: ItemInput) {
  return (
    [input.room, input.category, input.condition, input.notes].some(
      (value) => value.trim() !== "",
    ) ||
    input.tags.length > 0 ||
    input.collections.length > 0
  );
}
