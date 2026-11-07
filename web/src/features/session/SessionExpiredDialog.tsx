import { KeyRound } from "lucide-react";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import {
  FormError,
  PasswordField,
  TextField,
} from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { useSession } from "../../session/SessionContext";
import { SESSION_COPY } from "./copy";
import { useSignIn } from "./useSignIn";
import styles from "./Auth.module.css";

/**
 * Shown when any request finds the session expired. The page underneath
 * keeps its state, so after signing in the user retries their save without
 * retyping anything. It can't be dismissed: the only way on is signing in,
 * with any method the account has.
 */
export function SessionExpiredDialog() {
  const { sessionExpired, account, refresh, acknowledgeExpiry } = useSession();
  const signIn = useSignIn(async () => {
    await refresh();
    acknowledgeExpiry();
  }, account?.username ?? "");
  const needsCode = signIn.step === "second-factor";

  return (
    <Dialog
      open={sessionExpired}
      title={SESSION_COPY.expiredTitle}
      onClose={() => undefined}
      dismissible={false}
    >
      <form
        onSubmit={needsCode ? signIn.submitCode : signIn.submitPassword}
        className="stack"
      >
        <p className="muted">{SESSION_COPY.expiredText}</p>
        <FormError message={signIn.error} />
        {!needsCode && (
          <>
            <TextField
              label={SESSION_COPY.username}
              autoComplete="username"
              value={signIn.username}
              onValueChange={signIn.setUsername}
            />
            <PasswordField
              label={SESSION_COPY.password}
              autoComplete="current-password"
              value={signIn.password}
              onValueChange={signIn.setPassword}
            />
          </>
        )}
        {needsCode && (
          <TextField
            label={
              signIn.useRecovery ? SESSION_COPY.recoveryCode : SESSION_COPY.code
            }
            autoComplete="one-time-code"
            inputMode={signIn.useRecovery ? "text" : "numeric"}
            value={signIn.code}
            onValueChange={signIn.setCode}
            autoFocus
          />
        )}
        <Button
          type="submit"
          variant="primary"
          busy={signIn.busy}
          busyLabel={SESSION_COPY.signingIn}
        >
          {needsCode ? SESSION_COPY.verify : SESSION_COPY.signIn}
        </Button>
        {!needsCode && signIn.showPasskey && (
          <>
            <div className={styles.divider}>{COMMON.or}</div>
            <Button
              icon={<KeyRound aria-hidden="true" />}
              onClick={signIn.signInWithPasskey}
              disabled={signIn.busy}
            >
              {SESSION_COPY.usePasskey}
            </Button>
          </>
        )}
        {needsCode && (
          <>
            <button
              type="button"
              className={styles.linkButton}
              onClick={signIn.toggleRecovery}
            >
              {signIn.useRecovery
                ? SESSION_COPY.useAuthenticator
                : SESSION_COPY.useRecoveryCode}
            </button>
            <button
              type="button"
              className={styles.linkButton}
              onClick={signIn.startOver}
            >
              {SESSION_COPY.startOver}
            </button>
          </>
        )}
      </form>
    </Dialog>
  );
}
