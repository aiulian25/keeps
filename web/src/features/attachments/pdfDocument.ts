// Loads PDF.js only when a PDF is opened, and configures it so a document is
// treated as data: no PDF scripting (the library never runs it), no XFA
// forms, embedded fonts drawn as shapes. Pages are painted onto canvases.
import type { PDFDocumentLoadingTask } from "pdfjs-dist";

let loader: Promise<typeof import("pdfjs-dist")> | null = null;

function loadLibrary() {
  if (loader === null) {
    loader = Promise.all([
      import("pdfjs-dist"),
      import("pdfjs-dist/build/pdf.worker.min.mjs?url"),
    ]).then(([library, worker]) => {
      library.GlobalWorkerOptions.workerSrc = worker.default;
      return library;
    });
  }
  return loader;
}

/** Returns the loading task; call destroy() on it when the viewer closes. */
export async function openPdf(
  bytes: ArrayBuffer,
): Promise<PDFDocumentLoadingTask> {
  const library = await loadLibrary();
  return library.getDocument({
    data: new Uint8Array(bytes),
    enableXfa: false,
    disableFontFace: true,
    useSystemFonts: false,
    stopAtErrors: false,
  });
}
