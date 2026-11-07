import { ButtonLink } from "../components/Button";
import { PageHeader } from "../components/shell/PageHeader";
import { ROUTES } from "../constants/routes";

const COPY = {
  title: "Page not found",
  text: "This page doesn't exist. It may have been moved or deleted.",
  home: "Go to Home",
} as const;

export function NotFoundPage() {
  return (
    <div className="stack">
      <PageHeader title={COPY.title} />
      <p className="muted">{COPY.text}</p>
      <div>
        <ButtonLink to={ROUTES.home}>{COPY.home}</ButtonLink>
      </div>
    </div>
  );
}
