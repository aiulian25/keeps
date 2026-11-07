import { useEffect, useRef, type ReactNode } from "react";
import { Navigate, Outlet, useLocation } from "react-router";
import { COMMON, NAVIGATION } from "../../constants/copy";
import { Button } from "../Button";
import { ROUTES } from "../../constants/routes";
import { SessionExpiredDialog } from "../../features/session/SessionExpiredDialog";
import { useSession } from "../../session/SessionContext";
import { LoadingLine } from "../Bits";
import { BottomBar, Sidebar } from "./Navigation";
import styles from "./Shell.module.css";

/** Moves focus to the main heading after each navigation. */
function useFocusOnNavigate() {
  const { pathname } = useLocation();
  const mainReference = useRef<HTMLElement>(null);
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const heading = mainReference.current?.querySelector<HTMLElement>("h1");
    heading?.setAttribute("tabindex", "-1");
    heading?.focus({ preventScroll: false });
  }, [pathname]);
  return mainReference;
}

/** Shown when Keeps could not be reached on first load. */
function ConnectionProblem() {
  const { retry } = useSession();
  return (
    <div className={styles.unavailable} role="alert">
      <h1>{NAVIGATION.unavailableTitle}</h1>
      <p className="muted">{NAVIGATION.unavailableText}</p>
      <Button variant="primary" onClick={retry}>
        {COMMON.tryAgain}
      </Button>
    </div>
  );
}

export function AppShell() {
  const { status } = useSession();
  const location = useLocation();
  const mainReference = useFocusOnNavigate();

  if (status === "loading") {
    return <LoadingLine />;
  }
  if (status === "unavailable") {
    return <ConnectionProblem />;
  }
  if (status === "unclaimed") {
    return <Navigate to={ROUTES.setup} replace />;
  }
  if (status === "signed-out") {
    return (
      <Navigate
        to={ROUTES.signIn}
        replace
        state={{ from: location.pathname + location.search + location.hash }}
      />
    );
  }
  return (
    <div className={styles.shell}>
      <a className="skip-link" href="#main">
        {NAVIGATION.skipToContent}
      </a>
      <Sidebar signedIn />
      <main id="main" ref={mainReference} className={styles.main}>
        <Outlet />
      </main>
      <BottomBar />
      <SessionExpiredDialog />
    </div>
  );
}

/** Layout for setup, sign-in and recovery: brand only, no navigation. */
export function AuthLayout({
  children,
  aside,
}: {
  children: ReactNode;
  aside?: ReactNode;
}) {
  return (
    <div className={styles.shell}>
      <Sidebar signedIn={false} />
      <main
        id="main"
        className={styles.authMain}
        data-with-aside={aside ? true : undefined}
      >
        {aside && <div className={styles.authAside}>{aside}</div>}
        <div className={styles.authContent}>{children}</div>
      </main>
    </div>
  );
}
