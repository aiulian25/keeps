import { useState, type FormEvent } from "react";
import { ApiError } from "../../api/client";
import { sessionApi } from "../../api/endpoints";
import { useSession } from "../../session/SessionContext";
import { getPasskey, passkeysSupported } from "../../session/webauthn";
import { SESSION_COPY } from "./copy";

export type SignInStep = "password" | "second-factor";

/**
 * Every way back into Keeps: password, then an authenticator or recovery
 * code, or a passkey. Shared by the sign-in page and the session-expired
 * dialog, so both offer the same methods. `onSignedIn` runs after success.
 */
export function useSignIn(
  onSignedIn: () => Promise<void>,
  initialUsername = "",
) {
  const { bootstrap } = useSession();
  const [step, setStep] = useState<SignInStep>("password");
  const [username, setUsername] = useState(initialUsername);
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [useRecovery, setUseRecovery] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (caught) {
      setError(
        caught instanceof ApiError
          ? caught.message
          : SESSION_COPY.passkeyFailed,
      );
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    setStep("password");
    setPassword("");
    setCode("");
    setUseRecovery(false);
  };

  const finish = async () => {
    await onSignedIn();
    reset();
  };

  const submitPassword = (event: FormEvent) => {
    event.preventDefault();
    run(async () => {
      const result = await sessionApi.signIn({ username, password });
      if (result.awaitingSecondFactor) {
        setStep("second-factor");
        return;
      }
      await finish();
    });
  };

  const submitCode = (event: FormEvent) => {
    event.preventDefault();
    run(async () => {
      await sessionApi.secondFactor(
        useRecovery ? { recoveryCode: code } : { code },
      );
      await finish();
    });
  };

  const signInWithPasskey = () =>
    run(async () => {
      const start = await sessionApi.passkeyBegin();
      const credential = await getPasskey(start.options);
      await sessionApi.passkeyFinish({
        challengeId: start.challengeId,
        credential,
      });
      await finish();
    });

  const toggleRecovery = () => {
    setUseRecovery(!useRecovery);
    setCode("");
  };

  /** Back to the password step, e.g. after the code step has expired. */
  const startOver = () => {
    setError("");
    reset();
  };

  return {
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
    showPasskey: Boolean(bootstrap?.passkeysAvailable) && passkeysSupported(),
  };
}
