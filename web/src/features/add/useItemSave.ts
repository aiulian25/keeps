import { useState } from "react";
import { useNavigate } from "react-router";
import { ApiError } from "../../api/client";
import type { Item } from "../../api/types";
import { ERRORS } from "../../constants/copy";
import { ROUTES } from "../../constants/routes";

const ATTACHMENTS_FIELD = "attachmentIds";

function formErrorFor(error: ApiError) {
  const fieldMessages = Object.keys(error.fields);
  if (error.fields[ATTACHMENTS_FIELD]) {
    return error.fields[ATTACHMENTS_FIELD];
  }
  if (fieldMessages.length > 0) {
    return ERRORS.checkFields;
  }
  return error.message;
}

/**
 * Runs an item-creating request and goes to the saved item. On any failure
 * the caller keeps its inputs; nothing claims success unless the server did.
 */
export function useItemSave() {
  const navigate = useNavigate();
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const save = async (request: () => Promise<Item>, onSaved?: () => void) => {
    setSaving(true);
    setFormError("");
    setFieldErrors({});
    try {
      const item = await request();
      onSaved?.();
      navigate(ROUTES.item(item.id), {
        state: {
          saved: true,
          possibleDuplicates: item.possibleDuplicates ?? [],
        },
      });
    } catch (error) {
      if (!(error instanceof ApiError)) {
        setFormError(ERRORS.generic);
        return;
      }
      setFieldErrors(error.fields);
      setFormError(formErrorFor(error));
    } finally {
      setSaving(false);
    }
  };

  return { saving, formError, fieldErrors, setFieldErrors, save };
}
