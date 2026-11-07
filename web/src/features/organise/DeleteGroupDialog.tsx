import { useState } from "react";
import type { Group } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FieldShell, FormError } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { errorMessage } from "../account/errors";
import { ORGANISE_COPY } from "./copy";
import styles from "./Organise.module.css";

interface DeleteGroupDialogProps {
  group: Group | null;
  /** Groups the items could move to; empty when moving isn't offered. */
  moveTargets: Group[];
  onDelete: (moveTo?: number) => Promise<void>;
  onClose: () => void;
}

/** Deleting a grouping never deletes items: they move or become unassigned. */
export function DeleteGroupDialog({
  group,
  moveTargets,
  onDelete,
  onClose,
}: DeleteGroupDialogProps) {
  const [moveTo, setMoveTo] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const close = () => {
    setMoveTo("");
    setError(null);
    onClose();
  };

  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      await onDelete(moveTo ? Number(moveTo) : undefined);
      close();
    } catch (caught) {
      setError(caught);
    } finally {
      setBusy(false);
    }
  };

  const itemCount = group?.itemCount ?? 0;
  const consequence =
    itemCount > 0
      ? ORGANISE_COPY.deleteText(ORGANISE_COPY.itemCount(itemCount))
      : ORGANISE_COPY.deleteEmptyText;
  const offerMove = itemCount > 0 && moveTargets.length > 0;

  return (
    <Dialog
      open={group !== null}
      title={ORGANISE_COPY.deleteTitle(group?.name ?? "")}
      onClose={close}
      footer={
        <>
          <Button onClick={close}>{COMMON.cancel}</Button>
          <Button variant="danger" busy={busy} onClick={confirm}>
            {COMMON.delete}
          </Button>
        </>
      }
    >
      <p className="muted">{consequence}</p>
      <FormError message={errorMessage(error)} />
      {offerMove && (
        <FieldShell label={ORGANISE_COPY.moveTo}>
          {({ id }) => (
            <select
              id={id}
              className={styles.select}
              value={moveTo}
              onChange={(event) => setMoveTo(event.target.value)}
            >
              <option value="">{ORGANISE_COPY.leaveUnassigned}</option>
              {moveTargets.map((target) => (
                <option key={target.id} value={target.id}>
                  {target.name}
                </option>
              ))}
            </select>
          )}
        </FieldShell>
      )}
    </Dialog>
  );
}
