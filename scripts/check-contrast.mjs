// Measures WCAG 2.2 contrast for every token pair Keeps relies on, in both
// themes. Exits non-zero when a pair fails. Usage: node scripts/check-contrast.mjs
import { readFileSync } from "node:fs";

const TEXT_MINIMUM = 4.5;
const NON_TEXT_MINIMUM = 3;

const pairs = [
  ["--color-text", "--color-canvas", TEXT_MINIMUM],
  ["--color-text", "--color-surface", TEXT_MINIMUM],
  ["--color-text", "--color-field", TEXT_MINIMUM],
  ["--color-text", "--color-sidebar", TEXT_MINIMUM],
  ["--color-text", "--color-surface-raised", TEXT_MINIMUM],
  ["--color-text", "--color-chip", TEXT_MINIMUM],
  ["--color-text", "--color-button-secondary", TEXT_MINIMUM],
  ["--color-text-muted", "--color-canvas", TEXT_MINIMUM],
  ["--color-text-muted", "--color-surface", TEXT_MINIMUM],
  ["--color-text-muted", "--color-field", TEXT_MINIMUM],
  ["--color-text-subtle", "--color-field", TEXT_MINIMUM],
  ["--color-on-accent", "--color-accent", TEXT_MINIMUM],
  ["--color-on-accent", "--color-accent-hover", TEXT_MINIMUM],
  ["--color-on-accent-soft", "--color-accent-soft", TEXT_MINIMUM],
  ["--color-accent-text", "--color-canvas", TEXT_MINIMUM],
  ["--color-accent-text", "--color-surface", TEXT_MINIMUM],
  ["--color-danger", "--color-canvas", TEXT_MINIMUM],
  ["--color-danger", "--color-danger-surface", TEXT_MINIMUM],
  ["--color-success", "--color-success-surface", TEXT_MINIMUM],
  ["--color-text", "--color-success-surface", TEXT_MINIMUM],
  ["--color-focus", "--color-canvas", NON_TEXT_MINIMUM],
  ["--color-focus", "--color-surface", NON_TEXT_MINIMUM],
  ["--color-border-strong", "--color-field", NON_TEXT_MINIMUM],
  ["--color-border-strong", "--color-canvas", NON_TEXT_MINIMUM],
  ["--color-accent-border", "--color-canvas", NON_TEXT_MINIMUM],
  ["--color-accent-border", "--color-surface", NON_TEXT_MINIMUM],
];

const css = readFileSync(
  new URL("../web/src/styles/tokens.css", import.meta.url),
  "utf8",
);

function block(selectorPattern) {
  const match = css.match(new RegExp(selectorPattern + "\\s*\\{([^}]*)\\}"));
  const tokens = {};
  for (const [, name, value] of match[1].matchAll(
    /(--color-[\w-]+):\s*(#[0-9a-f]{6})/gi,
  )) {
    tokens[name] = value;
  }
  return tokens;
}

function luminance(hex) {
  const channels = [1, 3, 5].map(
    (start) => parseInt(hex.slice(start, start + 2), 16) / 255,
  );
  const linear = channels.map((value) =>
    value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}

function ratio(first, second) {
  const [lighter, darker] = [luminance(first), luminance(second)].sort(
    (a, b) => b - a,
  );
  return (lighter + 0.05) / (darker + 0.05);
}

const themes = {
  dark: block(':root,\\s*:root\\[data-theme="dark"\\]'),
  light: block(':root\\[data-theme="light"\\]'),
};

let failures = 0;
for (const [theme, tokens] of Object.entries(themes)) {
  for (const [foreground, background, minimum] of pairs) {
    const value = ratio(tokens[foreground], tokens[background]);
    const passed = value >= minimum;
    failures += passed ? 0 : 1;
    console.log(
      `${passed ? "pass" : "FAIL"}  ${theme.padEnd(5)} ${foreground} on ${background}: ${value.toFixed(2)} (needs ${minimum})`,
    );
  }
}
process.exit(failures === 0 ? 0 : 1);
