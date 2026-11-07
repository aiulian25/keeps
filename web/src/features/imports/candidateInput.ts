import type { ImportCandidate, ImportDraft, ItemInput } from "../../api/types";

/** Only descriptive fields; price paid and purchase date stay the user's. */
export function fieldsFromCandidate(
  candidate: ImportCandidate,
): Partial<ItemInput> {
  return {
    name: candidate.name,
    description: candidate.description,
    brand: candidate.brand,
    model: candidate.model,
    seller: candidate.seller,
  };
}

/** A finished draft always has a candidate, but the page title is a safe fallback. */
export function candidatesOf(draft: ImportDraft): ImportCandidate[] {
  if (draft.candidates.length > 0) {
    return draft.candidates;
  }
  return [
    {
      name: draft.pageTitle,
      description: "",
      brand: "",
      model: "",
      sku: "",
      listingPrice: null,
      availability: "",
      seller: "",
      imageIds: [],
    },
  ];
}
