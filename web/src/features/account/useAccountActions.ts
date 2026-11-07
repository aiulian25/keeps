import { accountApi } from "../../api/endpoints";
import type { Account } from "../../api/types";
import { useSession } from "../../session/SessionContext";
import { createPasskey } from "../../session/webauthn";
import { useReauthenticated } from "./ReauthProvider";

type AccountChanges = Partial<
  Pick<Account, "displayName" | "householdName" | "currency">
>;

/** Every account and security change, with reauthentication handled. */
export function useAccountActions() {
  const { replaceAccount, refresh } = useSession();
  const { withReauthentication } = useReauthenticated();

  return {
    update: async (changes: AccountChanges) =>
      replaceAccount(await accountApi.update(changes)),

    /** The server rotates this session and ends the others; pick up the new one. */
    changePassword: async (
      body: Parameters<typeof accountApi.changePassword>[0],
    ) => {
      await accountApi.changePassword(body);
      await refresh();
    },

    addPasskey: async (name: string) => {
      const start = await withReauthentication(() => accountApi.passkeyBegin());
      const credential = await createPasskey(start.options);
      replaceAccount(
        await accountApi.passkeyFinish({
          challengeId: start.challengeId,
          name,
          credential,
        }),
      );
    },
    renamePasskey: async (id: number, name: string) =>
      replaceAccount(
        await withReauthentication(() => accountApi.passkeyRename(id, name)),
      ),
    removePasskey: async (id: number) =>
      replaceAccount(
        await withReauthentication(() => accountApi.passkeyRemove(id)),
      ),

    beginAuthenticator: () =>
      withReauthentication(() => accountApi.totpBegin()),
    confirmAuthenticator: async (code: string) => {
      const { recoveryCodes } = await withReauthentication(() =>
        accountApi.totpConfirm(code),
      );
      await refresh();
      return recoveryCodes;
    },
    turnOffAuthenticator: async () => {
      await withReauthentication(() => accountApi.totpDisable());
      await refresh();
    },
    newRecoveryCodes: async () => {
      const { recoveryCodes } = await withReauthentication(() =>
        accountApi.recoveryCodes(),
      );
      await refresh();
      return recoveryCodes;
    },
  };
}
