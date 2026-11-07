import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Check, ChevronDown, MoreHorizontal } from "lucide-react";
import { NavLink } from "react-router";
import { COMMON } from "../constants/copy";
import { SAVED_BADGE_MS } from "../constants/limits";
import styles from "./Bits.module.css";

/** The quiet "Saved ✓" badge from the item screen; fades on its own. */
export function SavedBadge({ visibleSince }: { visibleSince: number | null }) {
  const [expiredFor, setExpiredFor] = useState<number | null>(null);
  useEffect(() => {
    if (visibleSince === null) {
      return;
    }
    const timer = window.setTimeout(
      () => setExpiredFor(visibleSince),
      SAVED_BADGE_MS,
    );
    return () => window.clearTimeout(timer);
  }, [visibleSince]);
  const visible = visibleSince !== null && expiredFor !== visibleSince;
  return (
    <span
      className={styles.saved}
      data-visible={visible || undefined}
      role="status"
      aria-live="polite"
    >
      {visible && (
        <>
          {COMMON.saved} <Check aria-hidden="true" />
        </>
      )}
    </span>
  );
}

interface TabItem {
  to: string;
  label: string;
}

/** Pill tabs that are real links, so each tab has its own URL. */
export function LinkTabs({
  items,
  label,
}: {
  items: TabItem[];
  label: string;
}) {
  return (
    <nav aria-label={label} className={styles.tabs}>
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) =>
            [styles.tab, isActive ? styles.tabActive : ""].join(" ")
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}

export function Disclosure({
  summary,
  openSummary,
  children,
  defaultOpen = false,
}: {
  summary: string;
  openSummary?: string;
  children: ReactNode;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const panelId = useId();
  return (
    <div className={styles.disclosure}>
      <button
        type="button"
        className={styles.disclosureButton}
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen(!open)}
      >
        {open && openSummary ? openSummary : summary}
        <ChevronDown aria-hidden="true" data-open={open || undefined} />
      </button>
      <div id={panelId} hidden={!open} className={styles.disclosurePanel}>
        {children}
      </div>
    </div>
  );
}

interface MenuAction {
  label: string;
  onSelect: () => void;
  tone?: "danger";
}

/** "…" menu with keyboard support (arrows, Escape, focus return). */
export function OverflowMenu({
  label,
  actions,
}: {
  label: string;
  actions: MenuAction[];
}) {
  const [open, setOpen] = useState(false);
  const buttonReference = useRef<HTMLButtonElement>(null);
  const menuReference = useRef<HTMLDivElement>(null);
  const menuId = useId();

  useEffect(() => {
    if (!open) {
      return;
    }
    menuReference.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const closeOnOutside = (event: MouseEvent) => {
      if (
        !menuReference.current?.contains(event.target as Node) &&
        !buttonReference.current?.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", closeOnOutside);
    return () => document.removeEventListener("mousedown", closeOnOutside);
  }, [open]);

  const close = () => {
    setOpen(false);
    buttonReference.current?.focus();
  };
  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const buttons = Array.from(
      menuReference.current?.querySelectorAll<HTMLButtonElement>("button") ??
        [],
    );
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === "Escape") {
      event.preventDefault();
      close();
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      buttons[(index + 1) % buttons.length]?.focus();
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      buttons[(index - 1 + buttons.length) % buttons.length]?.focus();
    }
  };

  return (
    <div className={styles.menuWrap}>
      <button
        ref={buttonReference}
        type="button"
        className={styles.menuButton}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          setOpen(!open);
        }}
      >
        <MoreHorizontal aria-hidden="true" />
      </button>
      {open && (
        <div
          ref={menuReference}
          id={menuId}
          role="menu"
          tabIndex={-1}
          className={styles.menu}
          onKeyDown={onKeyDown}
        >
          {actions.map((action) => (
            <button
              key={action.label}
              type="button"
              role="menuitem"
              data-tone={action.tone}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                close();
                action.onSelect();
              }}
            >
              {action.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export function Chip({
  icon,
  children,
  selected,
  onClick,
  label,
}: {
  icon?: ReactNode;
  children: ReactNode;
  selected?: boolean;
  onClick?: () => void;
  label?: string;
}) {
  if (!onClick) {
    return (
      <span className={styles.chip}>
        {icon}
        {children}
      </span>
    );
  }
  return (
    <button
      type="button"
      className={styles.chip}
      aria-pressed={selected}
      aria-label={label}
      onClick={onClick}
    >
      {icon}
      {children}
    </button>
  );
}

export function LoadingLine({ text = COMMON.loading }: { text?: string }) {
  return (
    <p className={styles.loading} role="status">
      {text}
    </p>
  );
}

export function Card({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={[styles.card, className].filter(Boolean).join(" ")}>
      {children}
    </div>
  );
}
