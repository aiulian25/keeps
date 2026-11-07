import type { ComponentProps } from "react";
import { Dialog } from "../../components/Dialog";

type PersistentDialogProps = ComponentProps<typeof Dialog> & {
  locked: boolean;
};

/** A Dialog that can't be dismissed while `locked` (unsaved recovery codes, a restore in progress). */
export function PersistentDialog({ locked, ...rest }: PersistentDialogProps) {
  return <Dialog dismissible={!locked} {...rest} />;
}
