import { useNavigate } from "react-router";

/**
 * Back arrows and Cancel buttons: return to the previous page in Keeps, or to
 * `fallback` when this page was opened directly (a bookmark or a new tab).
 */
export function useGoBack(fallback: string) {
  const navigate = useNavigate();
  return () => {
    const historyIndex =
      (window.history.state as { idx?: number } | null)?.idx ?? 0;
    if (historyIndex > 0) {
      navigate(-1);
      return;
    }
    navigate(fallback);
  };
}
