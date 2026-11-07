import { createBrowserRouter, Navigate } from "react-router";
import { AppShell } from "./components/shell/AppShell";
import { ROUTES } from "./constants/routes";
import { AccountPage } from "./features/account/AccountPage";
import { AddItemPage } from "./features/add/AddItemPage";
import { ButtonSetupPage } from "./features/add/ButtonSetupPage";
import { FromPagePage } from "./features/add/FromPagePage";
import { ManualItemPage } from "./features/add/ManualItemPage";
import { DocumentsPage } from "./features/attachments/DocumentsPage";
import { ItemDocumentsPage } from "./features/attachments/ItemDocumentsPage";
import { HomePage } from "./features/home/HomePage";
import { ImportPage } from "./features/imports/ImportPage";
import { ItemDetailPage } from "./features/items/ItemDetailPage";
import { ItemsPage } from "./features/items/ItemsPage";
import { NotFoundPage } from "./features/NotFoundPage";
import { OrganisePage } from "./features/organise/OrganisePage";
import { RecoverPage } from "./features/session/RecoverPage";
import { SignInPage } from "./features/session/SignInPage";
import { ClaimPage } from "./features/setup/ClaimPage";
import { RecentlyDeletedPage } from "./features/trash/RecentlyDeletedPage";

export const router = createBrowserRouter([
  { path: ROUTES.setup, element: <ClaimPage /> },
  { path: ROUTES.signIn, element: <SignInPage /> },
  { path: ROUTES.recover, element: <RecoverPage /> },
  {
    path: ROUTES.home,
    element: <AppShell />,
    children: [
      { index: true, element: <HomePage /> },
      { path: "items", element: <ItemsPage /> },
      { path: "items/:itemId", element: <ItemDetailPage /> },
      { path: "items/:itemId/documents", element: <ItemDocumentsPage /> },
      { path: "add", element: <AddItemPage /> },
      { path: "add/manual", element: <ManualItemPage /> },
      { path: "add/import/:importId", element: <ImportPage /> },
      { path: "add/from-page", element: <FromPagePage /> },
      { path: "add/button", element: <ButtonSetupPage /> },
      {
        path: "organise",
        element: <Navigate to={ROUTES.organise()} replace />,
      },
      { path: "organise/:kind", element: <OrganisePage /> },
      { path: "documents", element: <DocumentsPage /> },
      { path: "trash", element: <RecentlyDeletedPage /> },
      { path: "account", element: <Navigate to={ROUTES.account()} replace /> },
      { path: "account/:tab", element: <AccountPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
