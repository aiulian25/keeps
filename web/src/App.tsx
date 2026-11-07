import { RouterProvider } from "react-router";
import { ToastProvider } from "./components/Toast";
import { router } from "./routes";
import { SessionProvider } from "./session/SessionContext";

export function App() {
  return (
    <SessionProvider>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </SessionProvider>
  );
}
