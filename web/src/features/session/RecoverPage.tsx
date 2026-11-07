import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router";
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
import { SESSION_COPY } from "./copy";
import styles from "./Auth.module.css";

/** Redeems the one-time code printed by `keeps recover-admin`. */
export function RecoverPage() {
  const { refresh } = useSession();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setErrors({});
    setMessage("");
    try {
      await sessionApi.recover({ code, newPassword: password });
      await refresh();
      navigate(ROUTES.account("security"), { replace: true });
    } catch (caught) {
      const failure = caught instanceof ApiError ? caught : null;
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
    <AuthLayout>
      <form className={styles.panel} onSubmit={submit}>
        <div className={styles.heading}>
          <BrandIcon />
          <h1>{SESSION_COPY.recoverTitle}</h1>
          <p>{SESSION_COPY.recoverHint}</p>
        </div>
        <FormError message={message} />
        <TextField
          label={SESSION_COPY.recoveryCapability}
          value={code}
          onValueChange={setCode}
          autoComplete="off"
          error={errors.code}
          required
          autoFocus
        />
        <PasswordField
          label={SESSION_COPY.newPassword}
          autoComplete="new-password"
          value={password}
          onValueChange={setPassword}
          hint={SESSION_COPY.passwordHint}
          error={errors.newPassword}
          required
        />
        <Button type="submit" variant="primary" busy={busy}>
          {SESSION_COPY.setPassword}
        </Button>
      </form>
    </AuthLayout>
  );
}
