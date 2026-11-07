import { useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router";
import { ApiError } from "../../api/client";
import { sessionApi } from "../../api/endpoints";
import { Button } from "../../components/Button";
import {
  FormError,
  PasswordField,
  TextField,
} from "../../components/form/Field";
import { AuthLayout } from "../../components/shell/AppShell";
import { BrandIcon } from "../../components/shell/BrandMark";
import { ROUTES } from "../../constants/routes";
import { useSession } from "../../session/SessionContext";
import styles from "../session/Auth.module.css";

const COPY = {
  title: "Claim your home",
  intro:
    "Set up your household in Keeps. You'll use the one-time setup code from the server log.",
  setupCode: "Setup code",
  setupCodePlaceholder: "XXXX-XXXX-XXXX-XXXX",
  householdName: "Household name",
  householdPlaceholder: "e.g. The Miller Home",
  username: "Username",
  usernameHint: "You'll use this to sign in.",
  password: "Admin password",
  passwordHint: "At least 10 characters. A few words together work well.",
  confirm: "Confirm password",
  mismatch: "The passwords don't match.",
  create: "Create admin",
  creating: "Creating…",
} as const;

/** Panel 01, with the approved Username field (DEV-02). */
export function ClaimPage() {
  const { status, refresh } = useSession();
  const navigate = useNavigate();
  const [setupCode, setSetupCode] = useState("");
  const [householdName, setHouseholdName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  if (status === "signed-in" || status === "signed-out") {
    return (
      <Navigate
        to={status === "signed-in" ? ROUTES.home : ROUTES.signIn}
        replace
      />
    );
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setErrors({});
    setMessage("");
    if (password !== confirm) {
      setErrors({ confirm: COPY.mismatch });
      return;
    }
    setBusy(true);
    try {
      await sessionApi.claim({ setupCode, householdName, username, password });
      await refresh();
      navigate(ROUTES.home, { replace: true });
    } catch (caught) {
      const failure = caught instanceof ApiError ? caught : null;
      if (failure?.code === "setup_code_used") {
        await refresh().catch(() => undefined);
        return;
      }
      setErrors(failure?.fields ?? {});
      setMessage(
        failure && Object.keys(failure.fields).length === 0
          ? failure.message
          : "",
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout
      aside={<div className={styles.illustration} role="presentation" />}
    >
      <form className={styles.panel} onSubmit={submit}>
        <div className={styles.heading}>
          <BrandIcon />
          <h1>{COPY.title}</h1>
          <p>{COPY.intro}</p>
        </div>
        <FormError message={message} />
        <TextField
          label={COPY.setupCode}
          value={setupCode}
          onValueChange={setSetupCode}
          placeholder={COPY.setupCodePlaceholder}
          autoComplete="off"
          spellCheck={false}
          error={errors.setupCode}
          required
          autoFocus
        />
        <TextField
          label={COPY.householdName}
          value={householdName}
          onValueChange={setHouseholdName}
          placeholder={COPY.householdPlaceholder}
          error={errors.householdName}
          required
        />
        <TextField
          label={COPY.username}
          value={username}
          onValueChange={setUsername}
          autoComplete="username"
          hint={COPY.usernameHint}
          error={errors.username}
          required
        />
        <PasswordField
          label={COPY.password}
          value={password}
          onValueChange={setPassword}
          autoComplete="new-password"
          hint={COPY.passwordHint}
          error={errors.password}
          required
        />
        <PasswordField
          label={COPY.confirm}
          value={confirm}
          onValueChange={setConfirm}
          autoComplete="new-password"
          error={errors.confirm}
          required
        />
        <Button
          type="submit"
          variant="primary"
          busy={busy}
          busyLabel={COPY.creating}
        >
          {COPY.create}
        </Button>
      </form>
    </AuthLayout>
  );
}
