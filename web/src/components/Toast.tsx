import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CheckCircle2, X } from "lucide-react";
import { COMMON } from "../constants/copy";
import { TOAST_MS } from "../constants/limits";
import styles from "./Toast.module.css";

interface ToastAction {
  label: string;
  onAction: () => void;
}

interface ToastMessage {
  id: number;
  title: string;
  detail?: string;
  action?: ToastAction;
  tone: "success" | "error";
}

interface ToastValue {
  notify: (
    message: Omit<ToastMessage, "id" | "tone"> & {
      tone?: ToastMessage["tone"];
    },
  ) => void;
}

const ToastContext = createContext<ToastValue | null>(null);

/** Quiet, non-blocking feedback announced to screen readers. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [messages, setMessages] = useState<ToastMessage[]>([]);
  const counter = useRef(0);
  const dismiss = useCallback(
    (id: number) =>
      setMessages((current) => current.filter((message) => message.id !== id)),
    [],
  );
  const notify = useCallback<ToastValue["notify"]>(
    ({ tone = "success", ...message }) => {
      counter.current += 1;
      const id = counter.current;
      setMessages((current) => [...current, { id, tone, ...message }]);
      window.setTimeout(() => dismiss(id), TOAST_MS);
    },
    [dismiss],
  );
  const value = useMemo(() => ({ notify }), [notify]);
  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className={styles.region} role="status" aria-live="polite">
        {messages.map((message) => (
          <div
            key={message.id}
            className={styles.toast}
            data-tone={message.tone}
          >
            {message.tone === "success" && (
              <CheckCircle2 className={styles.icon} aria-hidden="true" />
            )}
            <div className={styles.text}>
              <strong>{message.title}</strong>
              {message.detail && <span>{message.detail}</span>}
            </div>
            {message.action && (
              <button
                type="button"
                className={styles.action}
                onClick={() => {
                  message.action?.onAction();
                  dismiss(message.id);
                }}
              >
                {message.action.label}
              </button>
            )}
            <button
              type="button"
              className={styles.close}
              aria-label={COMMON.close}
              onClick={() => dismiss(message.id)}
            >
              <X aria-hidden="true" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const value = useContext(ToastContext);
  if (value === null) {
    throw new Error("useToast must be used inside ToastProvider");
  }
  return value;
}
