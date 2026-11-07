import { useEffect, useId, useState, type KeyboardEvent } from "react";
import { X } from "lucide-react";
import { groupsApi } from "../../api/endpoints";
import type { GroupKind } from "../../api/types";
import { FieldShell } from "./Field";
import styles from "./GroupInput.module.css";

function useGroupNames(kind: GroupKind) {
  const [names, setNames] = useState<string[]>([]);
  useEffect(() => {
    groupsApi
      .list(kind)
      .then((groups) => setNames(groups.map((group) => group.name)))
      .catch(() => setNames([]));
  }, [kind]);
  return names;
}

interface SingleGroupProps {
  label: string;
  kind: GroupKind;
  value: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
}

/** Pick an existing room or category, or type a new name to create it. */
export function GroupInput({
  label,
  kind,
  value,
  onValueChange,
  placeholder,
}: SingleGroupProps) {
  const names = useGroupNames(kind);
  const listId = useId();
  return (
    <FieldShell label={label} optional>
      {({ id, describedBy }) => (
        <div className={styles.control}>
          <input
            id={id}
            className={styles.input}
            list={listId}
            value={value}
            placeholder={placeholder}
            autoComplete="off"
            onChange={(event) => onValueChange(event.target.value)}
            aria-describedby={describedBy}
          />
          <datalist id={listId}>
            {names.map((name) => (
              <option key={name} value={name} />
            ))}
          </datalist>
        </div>
      )}
    </FieldShell>
  );
}

interface MultiGroupProps {
  label: string;
  kind: GroupKind;
  values: string[];
  onValuesChange: (values: string[]) => void;
  placeholder?: string;
  removeLabel: (name: string) => string;
}

/** Tags or collections: chips plus a type-to-add input. */
export function MultiGroupInput({
  label,
  kind,
  values,
  onValuesChange,
  placeholder,
  removeLabel,
}: MultiGroupProps) {
  const names = useGroupNames(kind);
  const listId = useId();
  const [draft, setDraft] = useState("");
  const add = () => {
    const name = draft.trim();
    setDraft("");
    const exists = values.some(
      (existing) => existing.toLowerCase() === name.toLowerCase(),
    );
    if (name === "" || exists) {
      return;
    }
    onValuesChange([...values, name]);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      add();
    }
    const removeLast =
      event.key === "Backspace" && draft === "" && values.length > 0;
    if (removeLast) {
      onValuesChange(values.slice(0, -1));
    }
  };
  return (
    <FieldShell label={label} optional>
      {({ id, describedBy }) => (
        <div className={styles.control}>
          {values.map((name) => (
            <span key={name} className={styles.chip}>
              {name}
              <button
                type="button"
                aria-label={removeLabel(name)}
                onClick={() =>
                  onValuesChange(values.filter((value) => value !== name))
                }
              >
                <X aria-hidden="true" />
              </button>
            </span>
          ))}
          <input
            id={id}
            className={styles.input}
            list={listId}
            value={draft}
            placeholder={values.length === 0 ? placeholder : ""}
            autoComplete="off"
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onKeyDown}
            onBlur={add}
            aria-describedby={describedBy}
          />
          <datalist id={listId}>
            {names
              .filter((name) => !values.includes(name))
              .map((name) => (
                <option key={name} value={name} />
              ))}
          </datalist>
        </div>
      )}
    </FieldShell>
  );
}
