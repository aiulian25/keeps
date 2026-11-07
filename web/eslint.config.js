import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import jsxA11y from "eslint-plugin-jsx-a11y";

export default tseslint.config(
  { ignores: ["dist", "node_modules"] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  jsxA11y.flatConfigs.recommended,
  {
    plugins: { "react-hooks": reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "no-nested-ternary": "error",
      // Autofocus is used deliberately on single-purpose forms (sign-in, the
      // item name, dialogs) so people can start typing; nowhere else.
      "jsx-a11y/no-autofocus": "off",
      // Scrollable regions (photo and document viewers) must be focusable (WCAG 2.1.1).
      "jsx-a11y/no-noninteractive-tabindex": [
        "error",
        { roles: ["tabpanel", "region"], tags: [] },
      ],
      "no-magic-numbers": [
        "error",
        { ignore: [0, 1, -1], ignoreArrayIndexes: true },
      ],
    },
  },
  {
    // Browser tests compute TOTP codes and use fixtures inline; the app's
    // no-magic-numbers and non-null rules don't help readability there.
    files: ["e2e/**"],
    rules: {
      "no-magic-numbers": "off",
      "@typescript-eslint/no-non-null-assertion": "off",
    },
  },
);
