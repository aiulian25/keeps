import { startDownload } from "../../api/client";
import { dataApi } from "../../api/endpoints";
import type { PreparedFile } from "../../api/types";
import { useSession } from "../../session/SessionContext";
import { useReauthenticated } from "../account/ReauthProvider";

export type ExportFormat = Parameters<typeof dataApi.prepareExport>[0];

/**
 * The server writes each backup or export completely before offering it, so
 * a failure or a missing file is shown here instead of becoming a broken
 * download. The finished file is then fetched by a plain link.
 */
export function useDataActions() {
  const { refresh } = useSession();
  const { ensureRecentlyConfirmed } = useReauthenticated();

  return {
    exportData: async (format: ExportFormat) => {
      await ensureRecentlyConfirmed();
      const prepared = await dataApi.prepareExport(format);
      startDownload(prepared.downloadUrl);
    },
    prepareBackup: async (passphrase: string) => {
      await ensureRecentlyConfirmed();
      return dataApi.prepareBackup(passphrase);
    },
    download: (prepared: PreparedFile) => startDownload(prepared.downloadUrl),
    restore: async (
      file: File,
      passphrase: string,
      acceptIncomplete: boolean,
      onProgress: (fraction: number) => void,
    ) => {
      await ensureRecentlyConfirmed();
      return dataApi.restore(file, passphrase, acceptIncomplete, onProgress);
    },
    /** The server signed everyone out; refreshing lands on the sign-in page. */
    finishRestore: refresh,
  };
}
