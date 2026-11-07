import type { Theme } from "../../api/types";
import { useToast } from "../../components/Toast";
import { useSession } from "../../session/SessionContext";
import { APPEARANCE_COPY } from "./copy";
import { SettingsSection } from "./SettingsRow";
import styles from "./Account.module.css";

const THEME_OPTIONS: { theme: Theme; label: string; note?: string }[] = [
  {
    theme: "dark",
    label: APPEARANCE_COPY.dark,
    note: APPEARANCE_COPY.recommended,
  },
  { theme: "light", label: APPEARANCE_COPY.light },
];

/** Dark / Light cards from panel 11: a native radio group, so arrow keys work. */
export function AppearanceChoice() {
  const { account, setTheme } = useSession();
  const { notify } = useToast();
  const current = account?.theme ?? "dark";

  const choose = (theme: Theme) => {
    setTheme(theme).catch(() =>
      notify({ title: APPEARANCE_COPY.saveFailed, tone: "error" }),
    );
  };

  return (
    <SettingsSection title={APPEARANCE_COPY.title} hint={APPEARANCE_COPY.hint}>
      <fieldset className={styles.themeChoices}>
        <legend className="visually-hidden">{APPEARANCE_COPY.legend}</legend>
        {THEME_OPTIONS.map((option) => (
          <label key={option.theme} className={styles.themeCard}>
            <span
              className={styles.themePreview}
              data-preview={option.theme}
              aria-hidden="true"
            >
              <span />
              <span />
            </span>
            <span className={styles.themeText}>
              <span className={styles.rowLabel}>{option.label}</span>
              {option.note && (
                <span className={styles.rowDescription}>{option.note}</span>
              )}
            </span>
            <input
              type="radio"
              name="theme"
              value={option.theme}
              checked={current === option.theme}
              onChange={() => choose(option.theme)}
              className={styles.themeRadio}
            />
          </label>
        ))}
      </fieldset>
    </SettingsSection>
  );
}
