import { TextField } from "../../components/form/Field";
import { AUTHENTICATOR_CODE_LENGTH } from "./constants";
import { SECOND_FACTOR_COPY } from "./copy";
import styles from "./Account.module.css";

export interface SecondFactor {
  code: string;
  useRecoveryCode: boolean;
}

export const EMPTY_SECOND_FACTOR: SecondFactor = {
  code: "",
  useRecoveryCode: false,
};

/** The request fields for whichever factor the person typed. */
export function secondFactorBody({ code, useRecoveryCode }: SecondFactor) {
  if (!code) {
    return {};
  }
  return useRecoveryCode ? { recoveryCode: code } : { code };
}

interface SecondFactorFieldProps {
  value: SecondFactor;
  onChange: (value: SecondFactor) => void;
  error?: string;
}

/** Authenticator code, with "Use a recovery code instead". */
export function SecondFactorField({
  value,
  onChange,
  error,
}: SecondFactorFieldProps) {
  const label = value.useRecoveryCode
    ? SECOND_FACTOR_COPY.recoveryCode
    : SECOND_FACTOR_COPY.code;
  const toggleLabel = value.useRecoveryCode
    ? SECOND_FACTOR_COPY.useAuthenticator
    : SECOND_FACTOR_COPY.useRecoveryCode;
  return (
    <div className={styles.secondFactor}>
      <TextField
        label={label}
        autoComplete="one-time-code"
        inputMode={value.useRecoveryCode ? "text" : "numeric"}
        maxLength={
          value.useRecoveryCode ? undefined : AUTHENTICATOR_CODE_LENGTH
        }
        value={value.code}
        onValueChange={(code) => onChange({ ...value, code })}
        error={error}
        required
      />
      <button
        type="button"
        className={styles.linkButton}
        onClick={() =>
          onChange({ code: "", useRecoveryCode: !value.useRecoveryCode })
        }
      >
        {toggleLabel}
      </button>
    </div>
  );
}
