import { useState, type FormEvent } from "react";
import { Camera } from "lucide-react";
import { attachmentUrl } from "../../api/client";
import type { Group, RoomDetails } from "../../api/types";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormError, TextArea, TextField } from "../../components/form/Field";
import { COMMON } from "../../constants/copy";
import { errorMessage } from "../account/errors";
import { UPLOAD_COPY } from "../attachments/copy";
import { DropZone } from "../attachments/DropZone";
import { useUploads } from "../attachments/useUploads";
import { ROOM_COPY } from "./copy";
import styles from "./Organise.module.css";

interface RoomDialogProps {
  /** The room being edited; null when creating. */
  room: Group | null;
  onSubmit: (name: string, details: RoomDetails) => Promise<void>;
  onClose: () => void;
}

/** Create or edit a room: name required; description and photo optional. */
export function RoomDialog({ room, onSubmit, onClose }: RoomDialogProps) {
  const [name, setName] = useState(room?.name ?? "");
  const [description, setDescription] = useState(room?.description ?? "");
  const [photoId, setPhotoId] = useState(room?.photoId ?? 0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const uploads = useUploads((attachment) => setPhotoId(attachment.id));
  const photoUploads = uploads.uploads.filter(
    (upload) => upload.status !== "done",
  );

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onSubmit(name, { description, photoId });
      onClose();
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      title={room ? ROOM_COPY.editTitle(room.name) : ROOM_COPY.createTitle}
      onClose={onClose}
      footer={
        <>
          <Button onClick={onClose}>{COMMON.cancel}</Button>
          <Button
            type="submit"
            form="room-form"
            variant="primary"
            busy={busy}
            busyLabel={COMMON.saving}
            disabled={uploads.busy}
          >
            {uploads.busy ? ROOM_COPY.waitingForUpload : COMMON.save}
          </Button>
        </>
      }
    >
      <form id="room-form" className="stack" onSubmit={submit} noValidate>
        <FormError message={error} />
        <TextField
          label={ROOM_COPY.name}
          value={name}
          onValueChange={setName}
          required
          autoFocus
          maxLength={80}
        />
        <TextArea
          label={ROOM_COPY.description}
          optional
          hint={ROOM_COPY.descriptionHint}
          value={description}
          onValueChange={setDescription}
          rows={3}
          maxLength={1000}
        />
        <div className="stack">
          <span className={styles.fieldLabel}>
            {ROOM_COPY.photo} <span className="muted">{COMMON.optional}</span>
          </span>
          {photoId !== 0 && (
            <div className={styles.roomPhotoPreview}>
              <img src={attachmentUrl(photoId, "preview")} alt="" />
              <Button compact onClick={() => setPhotoId(0)}>
                {ROOM_COPY.removePhoto}
              </Button>
            </div>
          )}
          <DropZone
            kind="photo"
            variant="tile"
            multiple={false}
            title={photoId === 0 ? ROOM_COPY.addPhoto : ROOM_COPY.changePhoto}
            formats={UPLOAD_COPY.photoFormats}
            icon={<Camera aria-hidden="true" />}
            uploads={photoUploads}
            onFiles={uploads.add}
            onRetry={uploads.retry}
            onRemove={uploads.remove}
          />
        </div>
      </form>
    </Dialog>
  );
}
