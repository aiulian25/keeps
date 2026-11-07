import { useCallback, useEffect, useState } from "react";
import { accountApi } from "../../api/endpoints";
import type { SessionSummary } from "../../api/types";
import { errorMessage } from "./errors";
import { useReauthenticated } from "./ReauthProvider";

/** Signed-in sessions, current one first. */
export function useSessions() {
  const { withReauthentication } = useReauthenticated();
  const [sessions, setSessions] = useState<SessionSummary[] | null>(null);
  const [loadError, setLoadError] = useState("");

  const load = useCallback(
    () =>
      accountApi.sessions().then(
        (list) => {
          setSessions(
            [...list].sort(
              (first, second) => Number(second.current) - Number(first.current),
            ),
          );
          setLoadError("");
        },
        (caught) => setLoadError(errorMessage(caught)),
      ),
    [],
  );

  useEffect(() => {
    load();
  }, [load]);

  const revoke = async (id: number) => {
    await withReauthentication(() => accountApi.revokeSession(id));
    await load();
  };

  const revokeOthers = async () => {
    await withReauthentication(() => accountApi.revokeOthers());
    await load();
  };

  return { sessions, loadError, revoke, revokeOthers };
}
