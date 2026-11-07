import { useEffect, useId, type RefObject } from "react";
import { Search } from "lucide-react";
import fieldStyles from "../../components/form/Field.module.css";
import { ITEMS_COPY } from "./itemsCopy";

const SHORTCUT_KEY = "/";

function isTypingTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return (
    target.isContentEditable ||
    ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName)
  );
}

/** "/" focuses the search field unless the person is already typing somewhere. */
export function useSearchShortcut(input: RefObject<HTMLInputElement | null>) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const modified = event.ctrlKey || event.metaKey || event.altKey;
      if (
        event.key !== SHORTCUT_KEY ||
        modified ||
        isTypingTarget(event.target)
      ) {
        return;
      }
      event.preventDefault();
      input.current?.focus();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [input]);
}

interface SearchFieldProps {
  value: string;
  onValueChange: (value: string) => void;
  inputReference: RefObject<HTMLInputElement | null>;
}

export function SearchField({
  value,
  onValueChange,
  inputReference,
}: SearchFieldProps) {
  const id = useId();
  useSearchShortcut(inputReference);
  return (
    <div className={fieldStyles.control}>
      <label htmlFor={id} className="visually-hidden">
        {ITEMS_COPY.searchLabel}
      </label>
      <span className={fieldStyles.prefix} aria-hidden="true">
        <Search />
      </span>
      <input
        ref={inputReference}
        id={id}
        type="search"
        className={fieldStyles.input}
        value={value}
        placeholder={ITEMS_COPY.searchPlaceholder}
        autoComplete="off"
        aria-keyshortcuts={SHORTCUT_KEY}
        onChange={(event) => onValueChange(event.target.value)}
      />
    </div>
  );
}
