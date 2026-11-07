import type { ReactNode } from "react";
import { Navigate, useParams } from "react-router";
import type { Account } from "../../api/types";
import { LinkTabs } from "../../components/Bits";
import { ROUTES } from "../../constants/routes";
import { useSession } from "../../session/SessionContext";
import { DataTab } from "../data/DataTab";
import { AccountOverview } from "./AccountOverview";
import { AppearanceChoice } from "./AppearanceChoice";
import {
  ACCOUNT_TABS,
  DEFAULT_ACCOUNT_TAB,
  type AccountTab,
} from "./constants";
import { ACCOUNT_COPY } from "./copy";
import { ReauthProvider } from "./ReauthProvider";
import { SecuritySections } from "./SecuritySections";
import { SessionsTab } from "./SessionsTab";
import { SettingsSection } from "./SettingsRow";
import styles from "./Account.module.css";

const TAB_ITEMS = ACCOUNT_TABS.map((tab) => ({
  to: ROUTES.account(tab),
  label: ACCOUNT_COPY.tabs[tab],
}));

function isAccountTab(value: string | undefined): value is AccountTab {
  return ACCOUNT_TABS.some((tab) => tab === value);
}

const TAB_CONTENT: Record<AccountTab, (account: Account) => ReactNode> = {
  account: (account) => <AccountOverview account={account} />,
  security: (account) => (
    <SettingsSection
      title={ACCOUNT_COPY.tabs.security}
      hint={ACCOUNT_COPY.securityOverviewHint}
    >
      <SecuritySections account={account} />
    </SettingsSection>
  ),
  "sign-in": () => <SessionsTab />,
  appearance: () => <AppearanceChoice />,
  data: () => <DataTab />,
};

/** Panels 11 and 12: five tabs (DEV-09), each with its own URL. */
export function AccountPage() {
  const { tab } = useParams();
  const { account } = useSession();

  if (!isAccountTab(tab)) {
    return <Navigate to={ROUTES.account(DEFAULT_ACCOUNT_TAB)} replace />;
  }
  if (!account) {
    return null;
  }

  return (
    <ReauthProvider>
      <div className={styles.page}>
        <h1 className="visually-hidden">{ACCOUNT_COPY.pageTitle}</h1>
        <LinkTabs label={ACCOUNT_COPY.tabsLabel} items={TAB_ITEMS} />
        {TAB_CONTENT[tab](account)}
      </div>
    </ReauthProvider>
  );
}
