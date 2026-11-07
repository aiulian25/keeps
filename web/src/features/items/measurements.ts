import type { Measurements } from "../../api/types";

/** Null when nothing was entered, so an untouched form stays unchanged. */
export function measurementsOrNull(value: Measurements) {
  const entered = [
    value.width,
    value.height,
    value.depth,
    value.weight,
    value.fitNote,
  ].some((part) => part.trim() !== "");
  return entered ? value : null;
}

/** "80 × 201.5 cm (W × H)", using only the values given. */
export function formatSize(value: Measurements) {
  const given = [
    { letter: "W", amount: value.width },
    { letter: "H", amount: value.height },
    { letter: "D", amount: value.depth },
  ].filter((part) => part.amount !== "");
  if (given.length === 0) {
    return "";
  }
  const amounts = given.map((part) => part.amount).join(" × ");
  const letters = given.map((part) => part.letter).join(" × ");
  return `${amounts} ${value.lengthUnit} (${letters})`;
}

export function formatWeight(value: Measurements) {
  return value.weight ? `${value.weight} ${value.weightUnit}` : "";
}

/** One line for comparisons and history. */
export function formatMeasurements(value: Measurements | null) {
  if (!value) {
    return "";
  }
  return [formatSize(value), formatWeight(value), value.fitNote.trim()]
    .filter((part) => part !== "")
    .join("; ");
}
