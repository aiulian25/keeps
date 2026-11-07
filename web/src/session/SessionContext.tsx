import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ApiError,
  setCsrfToken,
  SESSION_EXPIRED_EVENT,
  STATUS_UNAUTHORIZED,
} from "../api/client";
import { accountApi, sessionApi } from "../api/endpoints";
import type { Account, BootstrapState, Theme } from "../api/types";

/** "unavailable": Keeps could not be reached before anything was known. */
type Status =
  "loading" | "unavailable" | "unclaimed" | "signed-out" | "signed-in";

interface SessionValue {
  status: Status;
  account: Account | null;
  bootstrap: BootstrapState | null;
  sessionExpired: boolean;
  refresh: () => Promise<void>;
  retry: () => Promise<void>;
  signOut: () => Promise<void>;
  setTheme: (theme: Theme) => Promise<void>;
  acknowledgeExpiry: () => void;
  replaceAccount: (account: Account) => void;
}

const SessionContext = createContext<SessionValue | null>(null);

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status>("loading");
  const [account, setAccount] = useState<Account | null>(null);
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [sessionExpired, setSessionExpired] = useState(false);
  const statusReference = useRef<Status>("loading");
  const changeStatus = useCallback((next: Status) => {
    statusReference.current = next;
    setStatus(next);
  }, []);

  // A failed request only says something about the session when the server
  // answered 401. Otherwise a signed-in view stays as it is (with its forms),
  // and a first load shows "unavailable" with Retry instead of loading forever.
  const connectionFailed = useCallback(
    (error: unknown) => {
      if (statusReference.current === "loading") {
        changeStatus("unavailable");
      }
      throw error;
    },
    [changeStatus],
  );

  const replaceAccount = useCallback((next: Account) => {
    setAccount(next);
    setCsrfToken(next.csrfToken);
    applyTheme(next.theme);
  }, []);

  const refresh = useCallback(async () => {
    const state = await sessionApi.bootstrap().catch(connectionFailed);
    setBootstrap(state);
    if (!state.claimed) {
      changeStatus("unclaimed");
      return;
    }
    try {
      replaceAccount(await sessionApi.current());
      setSessionExpired(false);
      changeStatus("signed-in");
    } catch (error) {
      const signedOut =
        error instanceof ApiError && error.status === STATUS_UNAUTHORIZED;
      if (!signedOut) {
        connectionFailed(error);
      }
      setAccount(null);
      setCsrfToken("");
      changeStatus("signed-out");
    }
  }, [replaceAccount, connectionFailed, changeStatus]);

  /** Retry from the "unavailable" screen; failures keep that screen. */
  const retry = useCallback(async () => {
    changeStatus("loading");
    await refresh().catch(() => undefined);
  }, [refresh, changeStatus]);

  useEffect(() => {
    // refresh awaits the network before setting any state.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    refresh().catch(() => undefined);
  }, [refresh]);

  useEffect(() => {
    const onExpired = () => setSessionExpired(true);
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
  }, []);

  // Signed out only once the server confirms it, or says the session had
  // already ended; any other failure leaves this device signed in and is
  // passed on so the button can say so and offer a retry.
  const signOut = useCallback(async () => {
    try {
      await sessionApi.signOut();
    } catch (error) {
      const alreadyEnded =
        error instanceof ApiError && error.status === STATUS_UNAUTHORIZED;
      if (!alreadyEnded) {
        throw error;
      }
    }
    setAccount(null);
    setCsrfToken("");
    changeStatus("signed-out");
  }, [changeStatus]);

  // The choice shows immediately; it is reverted if saving fails.
  const setTheme = useCallback(
    async (theme: Theme) => {
      const previous = document.documentElement.dataset.theme as Theme;
      applyTheme(theme);
      setAccount((current) => (current ? { ...current, theme } : current));
      try {
        replaceAccount(await accountApi.update({ theme }));
      } catch (error) {
        applyTheme(previous);
        setAccount((current) =>
          current ? { ...current, theme: previous } : current,
        );
        throw error;
      }
    },
    [replaceAccount],
  );

  const value = useMemo(
    () => ({
      status,
      account,
      bootstrap,
      sessionExpired,
      refresh,
      retry,
      signOut,
      setTheme,
      replaceAccount,
      acknowledgeExpiry: () => setSessionExpired(false),
    }),
    [
      status,
      account,
      bootstrap,
      sessionExpired,
      refresh,
      retry,
      signOut,
      setTheme,
      replaceAccount,
    ],
  );
  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession() {
  const value = useContext(SessionContext);
  if (value === null) {
    throw new Error("useSession must be used inside SessionProvider");
  }
  return value;
}
