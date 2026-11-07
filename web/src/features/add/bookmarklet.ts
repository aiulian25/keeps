// The "Add to Keeps" bookmark. It runs on the product page the person is
// viewing, reads only the product data the page itself publishes
// (structured data, preview meta tags, title), and opens Keeps with that
// data in the URL fragment. Fragments are never sent to any server or log.
import { ROUTES } from "../../constants/routes";

export const MAXIMUM_STRUCTURED_BLOCKS = 20;
export const MAXIMUM_META_TAGS = 200;
const CHUNK_SIZE = 32768;

/** Readable source; whitespace is collapsed when building the bookmark. */
function bookmarkSource(keepsOrigin: string) {
  return `(function () {
    var d = document;
    var all = function (selector) { return Array.prototype.slice.call(d.querySelectorAll(selector)); };
    var structuredData = all('script[type="application/ld+json"]').map(function (element) { return element.textContent || ""; }).slice(0, ${MAXIMUM_STRUCTURED_BLOCKS});
    var metaTags = all("meta[property],meta[name]").map(function (element) {
      return { key: element.getAttribute("property") || element.getAttribute("name") || "", value: element.getAttribute("content") || "" };
    }).filter(function (tag) { return /^(og:|product:|twitter:|description$)/i.test(tag.key); }).slice(0, ${MAXIMUM_META_TAGS});
    var bytes = new TextEncoder().encode(JSON.stringify({ url: location.href, title: d.title, metaTags: metaTags, structuredData: structuredData }));
    var binary = "";
    for (var index = 0; index < bytes.length; index += ${CHUNK_SIZE}) {
      binary += String.fromCharCode.apply(null, bytes.subarray(index, index + ${CHUNK_SIZE}));
    }
    var encoded = btoa(binary).replace(/\\+/g, "-").replace(/\\//g, "_").replace(/=+$/, "");
    window.open(${JSON.stringify(keepsOrigin + ROUTES.addFromPage)} + "#" + encoded, "_blank");
  })();`;
}

export function bookmarkletUrl(keepsOrigin: string) {
  return (
    "javascript:" +
    encodeURIComponent(bookmarkSource(keepsOrigin).replace(/\s+/g, " "))
  );
}

/** Reverses the bookmark's base64url encoding of UTF-8 JSON. */
export function decodeSnapshot(fragment: string): unknown {
  const base64 = fragment.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(base64);
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  return JSON.parse(new TextDecoder().decode(bytes));
}
