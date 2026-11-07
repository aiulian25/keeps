import {
  FileText,
  House,
  LogOut,
  Package,
  Sofa,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import { useState } from "react";
import { NavLink, useLocation } from "react-router";
import { useToast } from "../Toast";
import { COMMON, NAVIGATION } from "../../constants/copy";
import { useSession } from "../../session/SessionContext";
import { ROUTES } from "../../constants/routes";
import { BrandMark } from "./BrandMark";
import styles from "./Shell.module.css";

interface Destination {
  to: string;
  label: string;
  icon: LucideIcon;
  matches: (path: string) => boolean;
}

const destinations: Destination[] = [
  {
    to: ROUTES.home,
    label: NAVIGATION.home,
    icon: House,
    matches: (path) => path === ROUTES.home,
  },
  {
    to: ROUTES.items,
    label: NAVIGATION.items,
    icon: Package,
    matches: (path) =>
      path.startsWith(ROUTES.items) ||
      path.startsWith(ROUTES.add) ||
      path.startsWith(ROUTES.trash),
  },
  {
    to: ROUTES.organise(),
    label: NAVIGATION.rooms,
    icon: Sofa,
    matches: (path) => path.startsWith("/organise"),
  },
  {
    to: ROUTES.documents,
    label: NAVIGATION.documents,
    icon: FileText,
    matches: (path) => path.startsWith(ROUTES.documents),
  },
  {
    to: ROUTES.account(),
    label: NAVIGATION.account,
    icon: UserRound,
    matches: (path) => path.startsWith("/account"),
  },
];

function NavigationLinks({ className }: { className: string }) {
  const { pathname } = useLocation();
  return (
    <ul className={className}>
      {destinations.map(({ to, label, icon: Icon, matches }) => {
        const active = matches(pathname);
        return (
          <li key={to}>
            <NavLink
              to={to}
              className={styles.navLink}
              aria-current={active ? "page" : undefined}
              data-active={active || undefined}
              end={to === ROUTES.home}
            >
              <Icon aria-hidden="true" />
              <span>{label}</span>
            </NavLink>
          </li>
        );
      })}
    </ul>
  );
}

/** Sidebar on wide screens, icon rail on tablets, bottom bar on phones. */
export function Sidebar({ signedIn }: { signedIn: boolean }) {
  return (
    <aside className={styles.sidebar}>
      <BrandMark />
      {signedIn && (
        <nav aria-label={NAVIGATION.label}>
          <NavigationLinks className={styles.navList} />
        </nav>
      )}
      {signedIn && <SignOutButton />}
    </aside>
  );
}

/** Always reachable: bottom of the sidebar, or the top bar on phones. */
function SignOutButton() {
  const { signOut } = useSession();
  const { notify } = useToast();
  const [busy, setBusy] = useState(false);
  const attempt = async () => {
    if (busy) {
      return;
    }
    setBusy(true);
    try {
      await signOut();
    } catch {
      notify({
        title: COMMON.signOutFailed,
        detail: COMMON.signOutFailedDetail,
        tone: "error",
        action: { label: COMMON.tryAgain, onAction: attempt },
      });
    } finally {
      setBusy(false);
    }
  };
  return (
    <button
      type="button"
      className={styles.signOut}
      onClick={attempt}
      aria-busy={busy || undefined}
      aria-disabled={busy || undefined}
      title={COMMON.signOut}
    >
      <LogOut aria-hidden="true" />
      <span>{COMMON.signOut}</span>
    </button>
  );
}

export function BottomBar() {
  return (
    <nav aria-label={NAVIGATION.label} className={styles.bottomBar}>
      <NavigationLinks className={styles.bottomList} />
    </nav>
  );
}
