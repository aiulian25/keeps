import { useState } from "react";
import { KeyRound, ListOrdered, ShieldCheck } from "lucide-react";
import type { Account, PasskeySummary, TotpSetup } from "../../api/types";
import { OverflowMenu } from "../../components/Bits";
import { Button } from "../../components/Button";
import { useToast } from "../../components/Toast";
import { COMMON } from "../../constants/copy";
import { passkeysSupported } from "../../session/webauthn";
import { AuthenticatorSetupDialog } from "./AuthenticatorSetupDialog";
import { ConfirmDialog } from "./ConfirmDialog";
import { TOTAL_RECOVERY_CODES } from "./constants";
import { AUTHENTICATOR_COPY, PASSKEY_COPY, RECOVERY_COPY } from "./copy";
import { errorMessage } from "./errors";
import { formatRelativeTime, formatShortDate } from "./format";
import { NameDialog } from "./NameDialog";
import { RecoveryCodesDialog } from "./RecoveryCodesDialog";
import { SettingsCard, SettingsRow } from "./SettingsRow";
import { useAccountActions } from "./useAccountActions";
import styles from "./Account.module.css";

type PendingConfirmation = "turn-off-authenticator" | "new-codes" | null;

function describePasskeyError(error: unknown) {
  return error instanceof DOMException
    ? PASSKEY_COPY.cancelled
    : errorMessage(error);
}

function passkeyDetails(passkey: PasskeySummary) {
  const used = passkey.lastUsedAt
    ? PASSKEY_COPY.lastUsed(formatRelativeTime(passkey.lastUsedAt))
    : PASSKEY_COPY.neverUsed;
  return `${PASSKEY_COPY.added(formatShortDate(passkey.createdAt))} · ${used}`;
}

/** Passkeys (with their list), authenticator and recovery codes: the Security tab. */
export function SecuritySections({ account }: { account: Account }) {
  const actions = useAccountActions();
  const { notify } = useToast();
  const [namingPasskey, setNamingPasskey] = useState(false);
  const [renaming, setRenaming] = useState<PasskeySummary | null>(null);
  const [removing, setRemoving] = useState<PasskeySummary | null>(null);
  const [totpSetup, setTotpSetup] = useState<TotpSetup | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);
  const [pending, setPending] = useState<PendingConfirmation>(null);
  const [startingAuthenticator, setStartingAuthenticator] = useState(false);

  const passkeysUsable = account.passkeysAvailable && passkeysSupported();

  const startAuthenticator = async () => {
    setStartingAuthenticator(true);
    try {
      setTotpSetup(await actions.beginAuthenticator());
    } catch (caught) {
      const message = errorMessage(caught);
      if (message) {
        notify({ title: message, tone: "error" });
      }
    } finally {
      setStartingAuthenticator(false);
    }
  };

  const confirmAuthenticator = async (code: string) => {
    const codes = await actions.confirmAuthenticator(code);
    setTotpSetup(null);
    setRecoveryCodes(codes);
  };

  const authenticatorAction = account.totpEnabled ? (
    <Button compact onClick={() => setPending("turn-off-authenticator")}>
      {AUTHENTICATOR_COPY.turnOff}
    </Button>
  ) : (
    <Button compact busy={startingAuthenticator} onClick={startAuthenticator}>
      {AUTHENTICATOR_COPY.setUp}
    </Button>
  );

  return (
    <>
      <SettingsCard>
        <SettingsRow
          icon={<KeyRound />}
          label={PASSKEY_COPY.title}
          optional
          description={
            passkeysUsable ? PASSKEY_COPY.description : PASSKEY_COPY.unavailable
          }
          action={
            <Button
              compact
              disabled={!passkeysUsable}
              onClick={() => setNamingPasskey(true)}
            >
              {PASSKEY_COPY.setUp}
            </Button>
          }
        >
          <ul className={styles.subList}>
            {account.passkeys.length === 0 && (
              <li className="muted small">{PASSKEY_COPY.none}</li>
            )}
            {account.passkeys.map((passkey) => (
              <li key={passkey.id} className={styles.subRow}>
                <div className={styles.rowText}>
                  <span>{passkey.name}</span>
                  <span className={styles.rowDescription}>
                    {passkeyDetails(passkey)}
                  </span>
                </div>
                <OverflowMenu
                  label={PASSKEY_COPY.menuLabel(passkey.name)}
                  actions={[
                    {
                      label: COMMON.rename,
                      onSelect: () => setRenaming(passkey),
                    },
                    {
                      label: COMMON.remove,
                      onSelect: () => setRemoving(passkey),
                      tone: "danger",
                    },
                  ]}
                />
              </li>
            ))}
          </ul>
        </SettingsRow>
        <SettingsRow
          icon={<ShieldCheck />}
          label={AUTHENTICATOR_COPY.title}
          optional
          description={AUTHENTICATOR_COPY.description}
          value={account.totpEnabled ? AUTHENTICATOR_COPY.on : undefined}
          action={authenticatorAction}
        />
        <SettingsRow
          icon={<ListOrdered />}
          label={RECOVERY_COPY.title}
          description={
            account.totpEnabled
              ? RECOVERY_COPY.description
              : RECOVERY_COPY.needsAuthenticator
          }
          value={
            account.totpEnabled
              ? RECOVERY_COPY.remaining(
                  account.recoveryCodesRemaining,
                  TOTAL_RECOVERY_CODES,
                )
              : undefined
          }
          action={
            <Button
              compact
              disabled={!account.totpEnabled}
              onClick={() => setPending("new-codes")}
            >
              {RECOVERY_COPY.newCodes}
            </Button>
          }
        />
      </SettingsCard>

      <NameDialog
        open={namingPasskey}
        title={PASSKEY_COPY.nameTitle}
        label={PASSKEY_COPY.nameLabel}
        hint={PASSKEY_COPY.nameHint}
        initialName={PASSKEY_COPY.defaultName}
        submitLabel={PASSKEY_COPY.continue}
        busyLabel={PASSKEY_COPY.creating}
        describeError={describePasskeyError}
        onSubmit={async (name) => {
          await actions.addPasskey(name);
          notify({ title: PASSKEY_COPY.saved });
        }}
        onClose={() => setNamingPasskey(false)}
      />
      <NameDialog
        open={renaming !== null}
        title={PASSKEY_COPY.renameTitle}
        label={PASSKEY_COPY.nameLabel}
        initialName={renaming?.name ?? ""}
        submitLabel={COMMON.save}
        busyLabel={COMMON.saving}
        onSubmit={(name) => actions.renamePasskey(renaming?.id ?? 0, name)}
        onClose={() => setRenaming(null)}
      />
      <ConfirmDialog
        open={removing !== null}
        title={PASSKEY_COPY.removeTitle(removing?.name ?? "")}
        text={PASSKEY_COPY.removeText}
        confirmLabel={PASSKEY_COPY.remove}
        danger
        onConfirm={() => actions.removePasskey(removing?.id ?? 0)}
        onClose={() => setRemoving(null)}
      />
      <ConfirmDialog
        open={pending === "turn-off-authenticator"}
        title={AUTHENTICATOR_COPY.turnOffTitle}
        text={AUTHENTICATOR_COPY.turnOffText}
        confirmLabel={AUTHENTICATOR_COPY.turnOff}
        danger
        onConfirm={async () => {
          await actions.turnOffAuthenticator();
          notify({ title: AUTHENTICATOR_COPY.turnedOff });
        }}
        onClose={() => setPending(null)}
      />
      <ConfirmDialog
        open={pending === "new-codes"}
        title={RECOVERY_COPY.newCodesTitle}
        text={RECOVERY_COPY.newCodesText}
        confirmLabel={RECOVERY_COPY.makeCodes}
        onConfirm={async () =>
          setRecoveryCodes(await actions.newRecoveryCodes())
        }
        onClose={() => setPending(null)}
      />
      <AuthenticatorSetupDialog
        setup={totpSetup}
        onConfirm={confirmAuthenticator}
        onClose={() => setTotpSetup(null)}
      />
      <RecoveryCodesDialog
        codes={recoveryCodes}
        onDone={() => setRecoveryCodes(null)}
      />
    </>
  );
}
