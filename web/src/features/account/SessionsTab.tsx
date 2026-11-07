import { useState } from "react";
import { Monitor } from "lucide-react";
import { LoadingLine } from "../../components/Bits";
import { Button } from "../../components/Button";
import { FormError } from "../../components/form/Field";
import { useToast } from "../../components/Toast";
import { SESSIONS_COPY } from "./copy";
import { errorMessage } from "./errors";
import { describeUserAgent, formatRelativeTime } from "./format";
import { SettingsCard, SettingsRow, SettingsSection } from "./SettingsRow";
import { useSessions } from "./useSessions";
import styles from "./Account.module.css";

/** A session id, or every other session at once. */
type BusyTarget = number | "others";

/** Sign-in tab: every signed-in browser, with sign-out per session. */
export function SessionsTab() {
  const { sessions, loadError, revoke, revokeOthers } = useSessions();
  const { notify } = useToast();
  const [busyTarget, setBusyTarget] = useState<BusyTarget | null>(null);
  const [actionError, setActionError] = useState("");

  const run = async (
    target: BusyTarget,
    action: () => Promise<void>,
    doneTitle: string,
  ) => {
    setBusyTarget(target);
    setActionError("");
    try {
      await action();
      notify({ title: doneTitle });
    } catch (caught) {
      setActionError(errorMessage(caught));
    } finally {
      setBusyTarget(null);
    }
  };

  if (sessions === null) {
    return loadError ? <FormError message={loadError} /> : <LoadingLine />;
  }

  const hasOthers = sessions.some((session) => !session.current);
  return (
    <SettingsSection title={SESSIONS_COPY.title} hint={SESSIONS_COPY.hint}>
      <FormError message={actionError} />
      <SettingsCard>
        {sessions.map((session) => {
          const device = describeUserAgent(session.userAgent);
          return (
            <SettingsRow
              key={session.id}
              icon={<Monitor />}
              label={device}
              description={SESSIONS_COPY.lastActive(
                formatRelativeTime(session.lastSeenAt),
              )}
              value={
                session.current ? (
                  <span className={styles.badge}>
                    {SESSIONS_COPY.thisDevice}
                  </span>
                ) : undefined
              }
              action={
                !session.current && (
                  <Button
                    compact
                    busy={busyTarget === session.id}
                    aria-label={SESSIONS_COPY.signOutLabel(device)}
                    onClick={() =>
                      run(
                        session.id,
                        () => revoke(session.id),
                        SESSIONS_COPY.signedOut,
                      )
                    }
                  >
                    {SESSIONS_COPY.signOut}
                  </Button>
                )
              }
            />
          );
        })}
      </SettingsCard>
      {hasOthers ? (
        <div>
          <Button
            busy={busyTarget === "others"}
            onClick={() =>
              run("others", revokeOthers, SESSIONS_COPY.othersSignedOut)
            }
          >
            {SESSIONS_COPY.signOutOthers}
          </Button>
        </div>
      ) : (
        <p className="muted small">{SESSIONS_COPY.onlyThisDevice}</p>
      )}
    </SettingsSection>
  );
}
