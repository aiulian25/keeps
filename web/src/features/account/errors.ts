import { ApiError } from "../../api/client";
import { ERRORS } from "../../constants/copy";
import { REAUTHENTICATION_REQUIRED_CODE } from "./constants";

/** Thrown when the person closes the "Confirm it's you" dialog; callers stay quiet. */
export class ReauthenticationCancelled extends Error {}

export function isReauthenticationRequired(error: unknown) {
  return (
    error instanceof ApiError && error.code === REAUTHENTICATION_REQUIRED_CODE
  );
}

/** Message for a form-level error; empty when there is nothing to say. */
export function errorMessage(error: unknown) {
  if (!error || error instanceof ReauthenticationCancelled) {
    return "";
  }
  if (error instanceof ApiError) {
    return error.message;
  }
  return ERRORS.generic;
}

export function fieldErrors(error: unknown): Record<string, string> {
  return error instanceof ApiError ? error.fields : {};
}

/** Form-level message, unless one of the shown fields already explains it. */
export function formErrorMessage(error: unknown, shownFields: string[]) {
  const fields = fieldErrors(error);
  if (shownFields.some((name) => fields[name])) {
    return "";
  }
  return errorMessage(error);
}
