import {
  createContext,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { accountApi, sessionApi } from "../../api/endpoints";
import { useSession } from "../../session/SessionContext";
import {
  isReauthenticationRequired,
  ReauthenticationCancelled,
} from "./errors";
import { ReauthDialog, type ReauthCredentials } from "./ReauthDialog";

interface ReauthValue {
  /** Runs the action; if the server wants fresh proof, asks for it and retries once. */
  withReauthentication: <Result>(
    action: () => Promise<Result>,
  ) => Promise<Result>;
  /** For native form downloads, which can't be retried: confirm first when needed. */
  ensureRecentlyConfirmed: () => Promise<void>;
}

const ReauthContext = createContext<ReauthValue | null>(null);

export function ReauthProvider({ children }: { children: ReactNode }) {
  const { account, replaceAccount } = useSession();
  const [open, setOpen] = useState(false);
  const settlePending = useRef<((confirmed: boolean) => void) | null>(null);

  const askForConfirmation = () =>
    new Promise<boolean>((resolve) => {
      settlePending.current = resolve;
      setOpen(true);
    });

  const settle = (confirmed: boolean) => {
    setOpen(false);
    settlePending.current?.(confirmed);
    settlePending.current = null;
  };

  const confirmOrThrow = async () => {
    const confirmed = await askForConfirmation();
    if (!confirmed) {
      throw new ReauthenticationCancelled();
    }
  };

  const withReauthentication = async <Result,>(
    action: () => Promise<Result>,
  ) => {
    try {
      return await action();
    } catch (error) {
      if (!isReauthenticationRequired(error)) {
        throw error;
      }
    }
    await confirmOrThrow();
    return action();
  };

  const ensureRecentlyConfirmed = async () => {
    const latest = await sessionApi.current();
    replaceAccount(latest);
    if (latest.recentlyConfirmed) {
      return;
    }
    await confirmOrThrow();
  };

  const confirm = async (credentials: ReauthCredentials) => {
    await accountApi.reauthenticate(credentials);
    settle(true);
  };

  return (
    <ReauthContext.Provider
      value={{ withReauthentication, ensureRecentlyConfirmed }}
    >
      {children}
      <ReauthDialog
        open={open}
        needsSecondFactor={Boolean(account?.totpEnabled)}
        onConfirm={confirm}
        onCancel={() => settle(false)}
      />
    </ReauthContext.Provider>
  );
}

export function useReauthenticated() {
  const value = useContext(ReauthContext);
  if (value === null) {
    throw new Error("useReauthenticated must be used inside ReauthProvider");
  }
  return value;
}
