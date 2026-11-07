import { useState } from "react";
import { Coins, House, Lock, User } from "lucide-react";
import type { Account } from "../../api/types";
import { Button } from "../../components/Button";
import { useToast } from "../../components/Toast";
import { ChangePasswordDialog } from "./ChangePasswordDialog";
import { CURRENCY_CODE_LENGTH } from "./constants";
import { ACCOUNT_COPY } from "./copy";
import { EditableRow } from "./EditableRow";
import { SettingsCard, SettingsRow, SettingsSection } from "./SettingsRow";
import { useAccountActions } from "./useAccountActions";

/** Account tab: name, password and household. Appearance and sign-in options have their own tabs (owner decision, 6 Nov 2026). */
export function AccountOverview({ account }: { account: Account }) {
  const { update } = useAccountActions();
  const { notify } = useToast();
  const [changingPassword, setChangingPassword] = useState(false);

  const saveField = async (
    label: string,
    changes: Parameters<typeof update>[0],
  ) => {
    await update(changes);
    notify({ title: ACCOUNT_COPY.saved(label) });
  };

  return (
    <>
      <SettingsSection
        title={ACCOUNT_COPY.yourAccount}
        hint={ACCOUNT_COPY.yourAccountHint}
      >
        <SettingsCard>
          <EditableRow
            icon={<User />}
            label={ACCOUNT_COPY.name}
            value={account.displayName}
            onSave={(displayName) =>
              saveField(ACCOUNT_COPY.name, { displayName })
            }
          />
          <SettingsRow
            icon={<Lock />}
            label={ACCOUNT_COPY.password}
            value={<span aria-hidden="true">{ACCOUNT_COPY.passwordMask}</span>}
            action={
              <Button compact onClick={() => setChangingPassword(true)}>
                {ACCOUNT_COPY.change}
              </Button>
            }
          />
        </SettingsCard>
      </SettingsSection>

      <SettingsSection
        title={ACCOUNT_COPY.household}
        hint={ACCOUNT_COPY.householdHint}
      >
        <SettingsCard>
          <EditableRow
            icon={<House />}
            label={ACCOUNT_COPY.householdName}
            value={account.householdName}
            onSave={(householdName) =>
              saveField(ACCOUNT_COPY.householdName, { householdName })
            }
          />
          <EditableRow
            icon={<Coins />}
            label={ACCOUNT_COPY.currency}
            value={account.currency}
            hint={ACCOUNT_COPY.currencyHint}
            maxLength={CURRENCY_CODE_LENGTH}
            normalise={(text) => text.trim().toUpperCase()}
            onSave={(currency) =>
              saveField(ACCOUNT_COPY.currency, { currency })
            }
          />
        </SettingsCard>
      </SettingsSection>

      <ChangePasswordDialog
        open={changingPassword}
        needsSecondFactor={account.totpEnabled}
        onClose={() => setChangingPassword(false)}
      />
    </>
  );
}
