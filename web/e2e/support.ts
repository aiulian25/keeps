import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
);
/** The disposable instance's data; scripts/e2e-server.sh uses the same default. */
export const dataDirectory =
  process.env.E2E_DATA_DIR ?? path.join(repositoryRoot, ".e2e-data");

/** The one-time setup code of the fresh test instance. */
export function bootstrapCode() {
  const output = execFileSync(
    path.join(dataDirectory, "keeps"),
    ["bootstrap-code"],
    {
      env: {
        ...process.env,
        APP_ORIGIN: "http://localhost:6030",
        KEEPS_DEV_LOOPBACK: "1",
        KEEPS_DATA_DIR: dataDirectory,
      },
    },
  ).toString();
  return output.match(/[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}/)![0];
}
