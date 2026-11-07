// Builds a sharing copy of a document or photo from pixels only: each page
// is drawn to a canvas, covered areas are painted solid black, and the result
// is encoded again as JPEG (and, for PDFs, wrapped in a new minimal PDF). No
// text layer, font, metadata, script, attachment or EXIF from the original
// can reach the copy, because nothing but the rendered pixels is used.
import { openPdf } from "./pdfDocument";

/** A covered area, as fractions of the page width and height (0–1). */
export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface SharingPage {
  canvas: HTMLCanvasElement;
  include: boolean;
  boxes: Box[];
}

const RENDER_WIDTH = 1240;
const PHOTO_MAXIMUM_EDGE = 2000;
export const SHARING_PAGES_MAXIMUM = 30;
const JPEG_QUALITY = 0.85;
const POINTS_PER_INCH = 72;
const RENDER_DOTS_PER_INCH = 150;
const REDACTION_COLOUR = "#000000";
// PDF object layout: 1 catalog, 2 page tree, then per page: page, content
// stream, image.
const CATALOG_OBJECT = 1;
const PAGE_TREE_OBJECT = 2;
const FIRST_PAGE_OBJECT = 3;
const OBJECTS_PER_PAGE = 3;
const CONTENT_OFFSET = 1;
const IMAGE_OFFSET = 2;
const OFFSET_DIGITS = 10;
const COORDINATE_DECIMALS = 2;

function blankCanvas(width: number, height: number) {
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.round(width));
  canvas.height = Math.max(1, Math.round(height));
  return canvas;
}

/** Renders up to SHARING_PAGES_MAXIMUM pages of a PDF at a fixed width. */
export async function renderPdfPages(bytes: ArrayBuffer) {
  const task = await openPdf(bytes);
  try {
    const pdf = await task.promise;
    const count = Math.min(pdf.numPages, SHARING_PAGES_MAXIMUM);
    const pages: HTMLCanvasElement[] = [];
    for (let number = 1; number <= count; number++) {
      const page = await pdf.getPage(number);
      const unscaled = page.getViewport({ scale: 1 });
      const viewport = page.getViewport({
        scale: RENDER_WIDTH / unscaled.width,
      });
      const canvas = blankCanvas(viewport.width, viewport.height);
      await page.render({ canvas, viewport }).promise;
      pages.push(canvas);
    }
    return { pages, total: pdf.numPages };
  } finally {
    task.destroy();
  }
}

/** Draws a photo upright (the browser applies EXIF orientation) and bounded. */
export async function renderPhoto(blob: Blob) {
  const bitmap = await createImageBitmap(blob, {
    imageOrientation: "from-image",
  });
  const scale = Math.min(
    1,
    PHOTO_MAXIMUM_EDGE / Math.max(bitmap.width, bitmap.height),
  );
  const canvas = blankCanvas(bitmap.width * scale, bitmap.height * scale);
  canvas.getContext("2d")?.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  return canvas;
}

/** Paints a page with its boxes covered; used for both preview and output. */
export function paintRedacted(
  target: HTMLCanvasElement,
  source: HTMLCanvasElement,
  boxes: Box[],
) {
  target.width = source.width;
  target.height = source.height;
  const context = target.getContext("2d");
  if (!context) {
    return;
  }
  context.drawImage(source, 0, 0);
  context.fillStyle = REDACTION_COLOUR;
  for (const box of boxes) {
    context.fillRect(
      Math.floor(box.x * source.width),
      Math.floor(box.y * source.height),
      Math.ceil(box.width * source.width),
      Math.ceil(box.height * source.height),
    );
  }
}

async function jpegBytes(canvas: HTMLCanvasElement) {
  const blob = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob(resolve, "image/jpeg", JPEG_QUALITY),
  );
  if (!blob) {
    throw new Error("The page could not be encoded.");
  }
  return new Uint8Array(await blob.arrayBuffer());
}

async function redactedJpeg(page: SharingPage) {
  const output = blankCanvas(page.canvas.width, page.canvas.height);
  paintRedacted(output, page.canvas, page.boxes);
  return {
    jpeg: await jpegBytes(output),
    width: output.width,
    height: output.height,
  };
}

/** A minimal PDF with one full-page JPEG per page and no /Info metadata. */
export function imagesToPdf(
  pages: { jpeg: Uint8Array; width: number; height: number }[],
) {
  const encoder = new TextEncoder();
  const chunks: Uint8Array[] = [];
  const offsets: number[] = [];
  let length = 0;
  const add = (part: Uint8Array | string) => {
    const bytes = typeof part === "string" ? encoder.encode(part) : part;
    chunks.push(bytes);
    length += bytes.length;
  };
  const object = (number: number, body: string) => {
    offsets[number] = length;
    add(`${number} 0 obj\n${body}\nendobj\n`);
  };
  const pageObject = (index: number) =>
    FIRST_PAGE_OBJECT + index * OBJECTS_PER_PAGE;
  const points = (pixels: number) =>
    ((pixels * POINTS_PER_INCH) / RENDER_DOTS_PER_INCH).toFixed(
      COORDINATE_DECIMALS,
    );

  add("%PDF-1.4\n");
  const kids = pages.map((_, index) => `${pageObject(index)} 0 R`);
  object(CATALOG_OBJECT, `<< /Type /Catalog /Pages ${PAGE_TREE_OBJECT} 0 R >>`);
  object(
    PAGE_TREE_OBJECT,
    `<< /Type /Pages /Kids [${kids.join(" ")}] /Count ${pages.length} >>`,
  );
  pages.forEach((page, index) => {
    const pageNumber = pageObject(index);
    const contentNumber = pageNumber + CONTENT_OFFSET;
    const imageNumber = pageNumber + IMAGE_OFFSET;
    const width = points(page.width);
    const height = points(page.height);
    object(
      pageNumber,
      `<< /Type /Page /Parent ${PAGE_TREE_OBJECT} 0 R /MediaBox [0 0 ${width} ${height}] /Resources << /XObject << /Im0 ${imageNumber} 0 R >> >> /Contents ${contentNumber} 0 R >>`,
    );
    const drawing = `q ${width} 0 0 ${height} 0 0 cm /Im0 Do Q`;
    object(
      contentNumber,
      `<< /Length ${drawing.length} >>\nstream\n${drawing}\nendstream`,
    );
    offsets[imageNumber] = length;
    add(
      `${imageNumber} 0 obj\n<< /Type /XObject /Subtype /Image /Width ${page.width} /Height ${page.height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${page.jpeg.length} >>\nstream\n`,
    );
    add(page.jpeg);
    add("\nendstream\nendobj\n");
  });

  const objectCount = pageObject(pages.length);
  const crossReference = length;
  add(`xref\n0 ${objectCount}\n0000000000 65535 f \n`);
  for (let number = CATALOG_OBJECT; number < objectCount; number++) {
    add(`${String(offsets[number]).padStart(OFFSET_DIGITS, "0")} 00000 n \n`);
  }
  add(
    `trailer\n<< /Size ${objectCount} /Root ${CATALOG_OBJECT} 0 R >>\nstartxref\n${crossReference}\n%%EOF\n`,
  );

  const output = new Uint8Array(length);
  let position = 0;
  for (const chunk of chunks) {
    output.set(chunk, position);
    position += chunk.length;
  }
  return output;
}

/** The finished sharing copy as a downloadable file. */
export async function buildSharingCopy(pages: SharingPage[], asPdf: boolean) {
  const included = pages.filter((page) => page.include);
  const rendered = await Promise.all(included.map(redactedJpeg));
  if (!asPdf) {
    return new Blob([rendered[0].jpeg as BlobPart], { type: "image/jpeg" });
  }
  return new Blob([imagesToPdf(rendered) as BlobPart], {
    type: "application/pdf",
  });
}

/** "receipt.pdf" → "receipt (sharing copy).pdf" */
export function sharingFilename(filename: string, asPdf: boolean) {
  const base = filename.replace(/\.[^.]+$/, "");
  return `${base} (sharing copy).${asPdf ? "pdf" : "jpg"}`;
}
