import { useState } from "react";
import type { PreparedFile } from "../../api/types";
import {
  ChevronRight,
  Database,
  FileDown,
  Info,
  Layers,
  RotateCcw,
  Trash2,
} from "lucide-react";
import { Button, ButtonLink } from "../../components/Button";
import { useToast } from "../../components/Toast";
import { ROUTES } from "../../constants/routes";
import {
  SettingsCard,
  SettingsRow,
  SettingsSection,
} from "../account/SettingsRow";
import { BackupDialog } from "./BackupDialog";
import { ABOUT_COPY, BACKUP_COPY, DATA_COPY } from "./copy";
import { ExportDialog } from "./ExportDialog";
import { RestoreDialog } from "./RestoreDialog";
import { useDataActions, type ExportFormat } from "./useDataActions";
import { useRunningVersion } from "./useRunningVersion";

type OpenDialog = "export" | "backup" | "restore" | null;

/** Data tab (panel 12): export, backup, restore, recently deleted and the running version. */
export function DataTab() {
  const actions = useDataActions();
  const version = useRunningVersion();
  const { notify } = useToast();
  const [openDialog, setOpenDialog] = useState<OpenDialog>(null);
  const close = () => setOpenDialog(null);

  // The file is complete on the server; only the browser download remains.
  const exportData = async (format: ExportFormat) => {
    await actions.exportData(format);
    notify({ title: DATA_COPY.downloadStarting });
  };

  const downloadBackup = (prepared: PreparedFile) => {
    actions.download(prepared);
    notify({
      title: BACKUP_COPY.startedTitle,
      detail: BACKUP_COPY.startedDetail(prepared.filename),
    });
  };

  return (
    <>
      <SettingsSection
        title={DATA_COPY.title}
        hint={DATA_COPY.hint}
        icon={<Database aria-hidden="true" />}
      >
        <SettingsCard>
          <SettingsRow
            icon={<FileDown />}
            label={DATA_COPY.export}
            description={DATA_COPY.exportDescription}
            action={
              <Button compact onClick={() => setOpenDialog("export")}>
                {DATA_COPY.export}
              </Button>
            }
          />
          <SettingsRow
            icon={<Layers />}
            label={DATA_COPY.backup}
            description={DATA_COPY.backupDescription}
            action={
              <Button compact onClick={() => setOpenDialog("backup")}>
                {DATA_COPY.backUpNow}
              </Button>
            }
          />
          <SettingsRow
            icon={<RotateCcw />}
            label={DATA_COPY.restore}
            description={DATA_COPY.restoreDescription}
            action={
              <Button compact onClick={() => setOpenDialog("restore")}>
                {DATA_COPY.restore}
              </Button>
            }
          />
          <SettingsRow
            icon={<Trash2 />}
            label={DATA_COPY.recentlyDeleted}
            description={DATA_COPY.recentlyDeletedDescription}
            action={
              <ButtonLink
                compact
                to={ROUTES.trash}
                iconAfter={<ChevronRight aria-hidden="true" />}
              >
                {DATA_COPY.view}
              </ButtonLink>
            }
          />
        </SettingsCard>
        <ExportDialog
          open={openDialog === "export"}
          onExport={exportData}
          onClose={close}
        />
        <BackupDialog
          open={openDialog === "backup"}
          onPrepare={actions.prepareBackup}
          onDownload={downloadBackup}
          onClose={close}
        />
        <RestoreDialog
          open={openDialog === "restore"}
          onRestore={actions.restore}
          onFinished={actions.finishRestore}
          onClose={close}
        />
      </SettingsSection>
      {version && (
        <SettingsSection title={ABOUT_COPY.title} hint={ABOUT_COPY.hint}>
          <SettingsCard>
            <SettingsRow
              icon={<Info />}
              label={ABOUT_COPY.version}
              value={version}
            />
          </SettingsCard>
        </SettingsSection>
      )}
    </>
  );
}
