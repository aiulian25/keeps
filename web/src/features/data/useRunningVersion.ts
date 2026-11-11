import { useEffect, useState } from "react";
import { accountApi } from "../../api/endpoints";
import { ABOUT_COPY } from "./copy";
import { DEVELOPMENT_VERSION } from "./constants";

function displayedVersion(version: string) {
  if (version === DEVELOPMENT_VERSION) {
    return ABOUT_COPY.developmentBuild;
  }
  return version;
}

/** The running release as people should read it; empty until it loads. */
export function useRunningVersion() {
  const [version, setVersion] = useState("");
  useEffect(() => {
    accountApi.about().then(
      (about) => setVersion(displayedVersion(about.version)),
      () => setVersion(""),
    );
  }, []);
  return version;
}
