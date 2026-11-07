import { useEffect, useState } from "react";
import { groupsApi } from "../../api/endpoints";
import type { Group, GroupKind, RoomDetails } from "../../api/types";
import { errorMessage } from "../account/errors";

/**
 * Groups of one kind, reloaded after every change so counts stay right.
 * The page remounts per kind, so the list never mixes kinds.
 */
export function useGroups(kind: GroupKind) {
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [version, setVersion] = useState(0);

  useEffect(() => {
    let current = true;
    groupsApi.list(kind).then(
      (list) => {
        if (current) {
          setGroups(list);
          setLoadError("");
        }
      },
      (caught) => {
        if (current) {
          setLoadError(errorMessage(caught));
        }
      },
    );
    return () => {
      current = false;
    };
  }, [kind, version]);

  const reload = () => setVersion((count) => count + 1);

  return {
    groups,
    loadError,
    create: async (name: string, details: RoomDetails = {}) => {
      await groupsApi.create(kind, name, details);
      reload();
    },
    update: async (id: number, changes: RoomDetails & { name?: string }) => {
      await groupsApi.update(kind, id, changes);
      reload();
    },
    rename: async (id: number, name: string) => {
      await groupsApi.rename(kind, id, name);
      reload();
    },
    remove: async (id: number, moveTo?: number) => {
      await groupsApi.remove(kind, id, moveTo);
      reload();
    },
  };
}
