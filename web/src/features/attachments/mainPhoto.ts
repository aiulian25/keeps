/**
 * Keeps makes the first linked photo the item's main photo, so the chosen
 * one goes first. If it isn't among the ids, the order is left as it is.
 */
export function withMainPhotoFirst(
  attachmentIds: number[],
  mainPhotoId: number | null,
) {
  if (mainPhotoId === null || !attachmentIds.includes(mainPhotoId)) {
    return attachmentIds;
  }
  return [mainPhotoId, ...attachmentIds.filter((id) => id !== mainPhotoId)];
}
