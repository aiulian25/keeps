import { KeyRound } from "lucide-react";
import { Navigate, useLocation, useNavigate } from "react-router";
import { Button } from "../../components/Button";
import {
  FormError,
  PasswordField,
  TextField,
} from "../../components/form/Field";
import { AuthLayout } from "../../components/shell/AppShell";
import { BrandIcon } from "../../components/shell/BrandMark";
import { COMMON } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";
import { useSession } from "../../session/SessionContext";
import { SESSION_COPY } from "./copy";
import { useSignIn } from "./useSignIn";
import styles from "./Auth.module.css";

/** Panel 02, with DEV-02 (username) and DEV-03 (code as a second step). */
export function SignInPage() {
  const { status, refresh } = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const signedIn = async () => {
    await refresh();
    navigate(
      (location.state as { from?: string } | null)?.from ?? ROUTES.home,
      { replace: true },
    );
  };

  const {
    step,
    username,
    setUsername,
    password,
    setPassword,
    code,
    setCode,
    useRecovery,
    toggleRecovery,
    error,
    busy,
    submitPassword,
    submitCode,
    signInWithPasskey,
    startOver,
    showPasskey,
  } = useSignIn(signedIn);

  if (status === "unclaimed") {
    return <Navigate to={ROUTES.setup} replace />;
  }
  if (status === "signed-in") {
    return (
      <Navigate
        to={(location.state as { from?: string } | null)?.from ?? ROUTES.home}
        replace
      />
    );
  }

  return (
    <AuthLayout>
      {step === "password" && (
        <form className={styles.panel} onSubmit={submitPassword}>
          <div className={styles.heading}>
            <BrandIcon />
            <h1>{SESSION_COPY.signInTitle}</h1>
          </div>
          <FormError message={error} />
          <TextField
            label={SESSION_COPY.username}
            autoComplete="username"
            value={username}
            onValueChange={setUsername}
            required
            autoFocus
          />
          <PasswordField
            label={SESSION_COPY.password}
            autoComplete="current-password"
            value={password}
            onValueChange={setPassword}
            required
          />
          <Button
            type="submit"
            variant="primary"
            busy={busy}
            busyLabel={SESSION_COPY.signingIn}
          >
            {SESSION_COPY.signIn}
          </Button>
          {showPasskey && (
            <>
              <div className={styles.divider}>{COMMON.or}</div>
              <Button
                icon={<KeyRound aria-hidden="true" />}
                onClick={signInWithPasskey}
                disabled={busy}
              >
                {SESSION_COPY.usePasskey}
              </Button>
            </>
          )}
        </form>
      )}
      {step === "second-factor" && (
        <form className={styles.panel} onSubmit={submitCode}>
          <div className={styles.heading}>
            <BrandIcon />
            <h1>{SESSION_COPY.authenticatorTitle}</h1>
            {!useRecovery && <p>{SESSION_COPY.authenticatorHint}</p>}
          </div>
          <FormError message={error} />
          <TextField
            label={useRecovery ? SESSION_COPY.recoveryCode : SESSION_COPY.code}
            autoComplete="one-time-code"
            inputMode={useRecovery ? "text" : "numeric"}
            value={code}
            onValueChange={setCode}
            required
            autoFocus
          />
          <Button type="submit" variant="primary" busy={busy}>
            {SESSION_COPY.verify}
          </Button>
          <button
            type="button"
            className={styles.linkButton}
            onClick={toggleRecovery}
          >
            {useRecovery
              ? SESSION_COPY.useAuthenticator
              : SESSION_COPY.useRecoveryCode}
          </button>
          <button
            type="button"
            className={styles.linkButton}
            onClick={startOver}
          >
            {SESSION_COPY.startOver}
          </button>
        </form>
      )}
    </AuthLayout>
  );
}
