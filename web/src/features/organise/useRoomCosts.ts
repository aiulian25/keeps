import { useEffect, useState } from "react";
import { itemsApi } from "../../api/endpoints";
import type { Group, RoomCosts } from "../../api/types";

/** Room costs, reloaded whenever the room list changes. Skipped for other kinds. */
export function useRoomCosts(enabled: boolean, groups: Group[] | null) {
  const [costs, setCosts] = useState<RoomCosts | null>(null);
  useEffect(() => {
    if (!enabled || groups === null) {
      return;
    }
    let current = true;
    itemsApi.roomCosts().then(
      (loaded) => current && setCosts(loaded),
      () => current && setCosts(null),
    );
    return () => {
      current = false;
    };
  }, [enabled, groups]);
  return enabled ? costs : null;
}
