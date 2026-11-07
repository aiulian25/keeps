import { useEffect, useState } from "react";
import { groupsApi, itemsApi } from "../../api/endpoints";
import type { Group, GroupKind } from "../../api/types";

/**
 * Groups of one kind, busiest first. Empty while loading or on error: filters are optional.
 * Bump `version` to load them again after a change.
 */
export function useGroups(kind: GroupKind, version = 0) {
  const [groups, setGroups] = useState<Group[]>([]);
  useEffect(() => {
    let active = true;
    groupsApi
      .list(kind)
      .then((loaded) => {
        if (active) {
          setGroups(
            [...loaded].sort(
              (first, second) => second.itemCount - first.itemCount,
            ),
          );
        }
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [kind, version]);
  return groups;
}

/** How many items have no room, so the "Unassigned" chip only shows when it leads somewhere. */
export function useUnassignedCount() {
  const [count, setCount] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    itemsApi
      .list({ unassigned: true, limit: 1 }, controller.signal)
      .then((page) => setCount(page.total))
      .catch(() => undefined);
    return () => controller.abort();
  }, []);
  return count;
}
