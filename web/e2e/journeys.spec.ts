// End-to-end journeys from the investigation (§6.4, §14 UX row), run against
// the real binary with the embedded web app. Run: npm --prefix web run e2e
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { createHmac } from "node:crypto";
import {
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  statSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { deflateSync } from "node:zlib";
import { bootstrapCode, dataDirectory, repositoryRoot } from "./support";
import { PAGE_SIZE } from "../src/constants/limits";

const capturesDirectory = path.join(
  repositoryRoot,
  "tests",
  "visual",
  "captures",
);
const storageState = path.join(dataDirectory, "storage-state.json");
const USERNAME = "e2eadmin";
const PASSWORD = "e2e only passphrase 2026";
const TOTP_PERIOD_SECONDS = 30;
const TOTP_DIGITS = 6;
const TOTP_TEST_TIMEOUT_MS = 90_000;
let totpSecret = "";
let lastUsedStep = 0;

/** Returns a code for a time step not used before; Keeps rejects replays. */
async function freshTotp() {
  let step = Math.floor(Date.now() / 1000 / TOTP_PERIOD_SECONDS);
  while (step <= lastUsedStep) {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    step = Math.floor(Date.now() / 1000 / TOTP_PERIOD_SECONDS);
  }
  lastUsedStep = step;
  return totp(totpSecret);
}

test.describe.configure({ mode: "serial" });

function base32Decode(text: string) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const character of text.replace(/=+$/, "").toUpperCase()) {
    bits += alphabet.indexOf(character).toString(2).padStart(5, "0");
  }
  const bytes = [];
  for (let index = 0; index + 8 <= bits.length; index += 8) {
    bytes.push(parseInt(bits.slice(index, index + 8), 2));
  }
  return Buffer.from(bytes);
}

function totp(secret: string, offsetSteps = 0) {
  const counter =
    Math.floor(Date.now() / 1000 / TOTP_PERIOD_SECONDS) + offsetSteps;
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac("sha1", base32Decode(secret))
    .update(message)
    .digest();
  const offset = digest[digest.length - 1] & 0xf;
  const value = (digest.readUInt32BE(offset) & 0x7fffffff) % 10 ** TOTP_DIGITS;
  return value.toString().padStart(TOTP_DIGITS, "0");
}

async function capture(page: Page, name: string) {
  mkdirSync(capturesDirectory, { recursive: true });
  await page.screenshot({
    path: path.join(capturesDirectory, `${name}.png`),
    fullPage: false,
  });
}

async function expectAccessible(page: Page, context: string) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const serious = results.violations.filter(
    (violation) =>
      violation.impact === "serious" || violation.impact === "critical",
  );
  expect(
    serious.map(
      (violation) =>
        `${context}: ${violation.id} — ${violation.nodes.map((node) => node.target.join(" ")).join(", ")}`,
    ),
  ).toEqual([]);
}

function action(page: Page, name: string | RegExp) {
  return page
    .getByRole("link", { name })
    .or(page.getByRole("button", { name }))
    .first();
}

function pngFile(name: string) {
  // 1×1 PNG; the server re-encodes previews from it.
  return {
    name,
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
      "base64",
    ),
  };
}

test("claim the instance and see the empty home", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/setup$/);
  await capture(page, "01-claim-dark");
  await expectAccessible(page, "claim");
  await page.getByLabel("Setup code").fill("WRONG-CODE-0000-0000");
  await page.getByLabel("Household name").fill("The E2E Home");
  await page.getByLabel("Username").fill(USERNAME);
  await page.getByLabel("Admin password").fill(PASSWORD);
  await page.getByLabel("Confirm password").fill(PASSWORD);
  await page.getByRole("button", { name: "Create admin" }).click();
  await expect(page.getByText(/setup code isn't right/i)).toBeVisible();
  await expect(page.getByLabel("Username")).toHaveValue(USERNAME);

  await page.getByLabel("Setup code").fill(bootstrapCode());
  await page.getByRole("button", { name: "Create admin" }).click();
  await expect(
    page.getByRole("heading", { name: "Add your first item" }),
  ).toBeVisible();
  await capture(page, "03-home-empty-dark");
  await expectAccessible(page, "home empty");
  await page.context().storageState({ path: storageState });
});

test.describe("signed in", () => {
  test.use({ storageState });

  test("name-only manual capture saves with one click and no taxonomy", async ({
    page,
  }) => {
    await page.goto("/add");
    await capture(page, "04-add-item-dark");
    await expectAccessible(page, "add item");
    await action(page, "Add manually").click();
    await expect(
      page.getByRole("heading", { name: "Add item manually" }),
    ).toBeVisible();
    await capture(page, "07-add-manually-dark");
    await expectAccessible(page, "manual");
    const started = Date.now();
    await page.getByLabel("Name").fill("Cordless drill");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Cordless drill" }),
    ).toBeVisible();
    expect(Date.now() - started).toBeLessThan(30_000);
    await expect(page.getByText("Saved")).toBeVisible();
  });

  test("a refresh keeps the manual form's latest details, uploads and save", async ({
    page,
  }) => {
    const name = "Desk lamp kept over a refresh";
    const room = () => page.getByLabel("Room");
    const showDetails = async () => {
      if (!(await room().isVisible())) {
        await page.getByRole("button", { name: "More details" }).click();
      }
    };
    await page.goto("/add");
    await action(page, "Add manually").click();
    await page.getByLabel("Name").fill(name);
    await showDetails();
    await room().fill("Study");
    await page
      .locator('input[type="file"]')
      .nth(0)
      .setInputFiles(pngFile("lamp.png"));
    await expect(
      page.getByRole("button", { name: "Remove lamp.png" }),
    ).toBeVisible();

    await page.reload();
    await expect(page.getByLabel("Name")).toHaveValue(name);
    await showDetails();
    await expect(room()).toHaveValue("Study");
    await expect(
      page.getByRole("button", { name: "Remove lamp.png" }),
    ).toBeVisible();

    // The first save reaches the server, but its answer is lost.
    let answerLost = false;
    await page.route("**/api/v1/items", async (route) => {
      if (answerLost || route.request().method() !== "POST") {
        await route.fallback();
        return;
      }
      answerLost = true;
      await route.fetch();
      await route.abort("connectionreset");
    });
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(page.getByText(/Couldn't reach Keeps/)).toBeVisible();
    await page.reload();
    await expect(page.getByLabel("Name")).toHaveValue(name);
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(page.getByRole("heading", { name })).toBeVisible();
    const itemId = new URL(page.url()).pathname.split("/").pop();
    const found = await (
      await page.request.get(`/api/v1/items?q=${encodeURIComponent(name)}`)
    ).json();
    expect(found.total).toBe(1);
    const saved = await (
      await page.request.get(`/api/v1/items/${itemId}`)
    ).json();
    expect(saved.photos).toHaveLength(1);

    // A new hand-over starts clean and resumes its own draft.
    await page.goto("/add");
    await action(page, "Add manually").click();
    await page.getByLabel("Name").fill("First entry, left open");
    await page.goto("/add");
    await action(page, "Upload receipt").click();
    await expect(page.getByLabel("Name")).toHaveValue("");
    await page.getByLabel("Name").fill("Second entry");
    await page.reload();
    await expect(page.getByLabel("Name")).toHaveValue("Second entry");
    await page.getByRole("button", { name: "Cancel" }).click();
  });

  test("manual item with photo, invoice, price, date and new room", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Sage Barista Express Coffee Machine");
    await page.getByLabel("Price paid", { exact: false }).first().fill("599");
    await page.getByLabel("Purchase date").fill("2024-03-12");
    const fileInputs = page.locator('input[type="file"]');
    await fileInputs.nth(0).setInputFiles(pngFile("coffee.png"));
    await fileInputs.nth(1).setInputFiles({
      name: "invoice.pdf",
      mimeType: "application/pdf",
      buffer: Buffer.from("%PDF-1.4\n%%EOF"),
    });
    await expect(page.getByText("coffee.png")).toBeVisible();
    await page.getByRole("button", { name: "More details" }).click();
    await page.getByLabel("Room").fill("Kitchen");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", {
        name: "Sage Barista Express Coffee Machine",
      }),
    ).toBeVisible();
    await expect(page.getByText("£599")).toBeVisible();
    await capture(page, "08-item-detail-dark");
    await expectAccessible(page, "item detail");

    await action(page, "Documents").click();
    await expect(page.getByText("invoice.pdf")).toBeVisible();
    await capture(page, "10-documents-dark");
    await expectAccessible(page, "item documents");
    const downloadPromise = page.waitForEvent("download");
    await action(page, /Download/).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("invoice.pdf");
  });

  test("find evidence later: search, open, invoice", async ({ page }) => {
    await page.goto("/items");
    await page
      .getByRole("searchbox")
      .or(page.getByLabel("Search items"))
      .first()
      .fill("coffee");
    await expect(
      page.getByText("Sage Barista Express Coffee Machine"),
    ).toBeVisible();
    await expect(page.getByText("Cordless drill")).toHaveCount(0);
    await page.goto("/items");
    await expect(action(page, /Kitchen/)).toBeVisible();
    await capture(page, "03-items-dark");
    await expectAccessible(page, "items");
  });

  test("failed and refused links keep the URL and offer manual entry", async ({
    page,
  }) => {
    await page.goto("/add");
    await page
      .getByLabel("Product link")
      .fill(
        "https://bucket.example.com/invoice.pdf?X-Amz-Signature=abc&X-Amz-Credential=x",
      );
    await action(page, /Get details/).click();
    await expect(page.getByText(/private invoice link/i)).toBeVisible();
    await page.getByLabel("Product link").fill("https://192.168.10.20/router");
    await action(page, /Get details/).click();
    await expect(page.getByText(/public shop pages/i)).toBeVisible();
    await action(page, "Add manually").click();
    await expect(page.getByText(/192\.168\.10\.20/).first()).toBeVisible();
  });

  test("organise: room with description and photo, three views, choice remembered", async ({
    page,
  }) => {
    await page.goto("/organise/rooms");
    await expect(page.getByText("Kitchen")).toBeVisible();
    await capture(page, "09-organise-dark");
    await expectAccessible(page, "organise");
    await page.getByRole("button", { name: /Create room/ }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name").fill("Garage");
    await dialog
      .getByLabel(/Description/)
      .fill("Tools and the bike, left of the door");
    await dialog
      .locator('input[type="file"]')
      .setInputFiles(pngFile("garage.png"));
    await expect(
      dialog.getByRole("button", { name: "Remove photo" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(
      page.getByText("Tools and the bike, left of the door"),
    ).toBeVisible();
    await expect(page.getByLabel("Cost of items in rooms")).toContainText(
      "All rooms £599",
    );
    await expect(page.getByText(/1 item · £599/)).toBeVisible();

    await page.getByRole("button", { name: "Cards" }).click();
    await expect(page.locator('[data-view="cards"]')).toBeVisible();
    await expect(page.getByRole("button", { name: "Cards" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await capture(page, "09-organise-cards-dark");
    await expectAccessible(page, "organise cards");
    await page.reload();
    await expect(page.locator('[data-view="cards"]')).toBeVisible();

    await page.getByRole("button", { name: "List" }).click();
    await expect(page.locator('[data-view="list"]')).toBeVisible();
    await capture(page, "09-organise-list-dark");
    await page.getByRole("button", { name: "Wide" }).click();
    await expect(page.locator('[data-view="wide"]')).toBeVisible();

    await page
      .getByRole("button", { name: /More actions for Garage|Garage/ })
      .last()
      .click();
    await page.getByRole("menuitem", { name: "Edit" }).click();
    const edit = page.getByRole("dialog");
    await expect(edit.getByLabel(/Description/)).toHaveValue(
      "Tools and the bike, left of the door",
    );
    await edit.getByRole("button", { name: "Remove photo" }).click();
    await edit.getByRole("button", { name: "Save" }).click();
    await expect(page.getByRole("dialog")).toBeHidden();
  });

  test("cancel leaves Add item and the manual form without saving", async ({
    page,
  }) => {
    await page.goto("/items");
    await action(page, /Add item/).click();
    await expect(page.getByRole("heading", { name: "Add item" })).toBeVisible();
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page).toHaveURL(/\/items$/);

    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Should not be saved");
    await page.getByRole("button", { name: "Cancel" }).click();
    await page.goto("/items?q=should");
    await expect(page.getByText("Should not be saved")).toHaveCount(0);
  });

  test("the Account tab holds only account and household settings", async ({
    page,
  }) => {
    await page.goto("/account/account");
    await expect(
      page.getByRole("heading", { name: "Your account" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Household" }),
    ).toBeVisible();
    await expect(page.getByRole("radio", { name: /Dark/ })).toHaveCount(0);
    await expect(page.getByText(/Passkeys/)).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Sign out" })).toHaveCount(1);
  });

  test("condition and warranty: set, see, edit and filter", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Dishwasher with warranty");
    await page.getByLabel(/Condition/).selectOption({ label: "Good" });
    await page.getByLabel(/Warranty until/).fill("2099-06-30");
    await capture(page, "07-condition-warranty-dark");
    await expectAccessible(page, "manual with condition");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Dishwasher with warranty" }),
    ).toBeVisible();
    await expect(page.getByText("Good", { exact: true })).toBeVisible();
    await expect(page.getByText(/In warranty until/)).toBeVisible();

    await page.getByRole("button", { name: "Edit" }).first().click();
    await page.getByLabel(/Condition/).selectOption({ label: "Fair" });
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByText("Fair", { exact: true })).toBeVisible();

    await page.goto("/items?condition=fair");
    await expect(page.getByText("Dishwasher with warranty")).toBeVisible();
    await expect(page.getByText("Cordless drill")).toHaveCount(0);
    await page.goto("/items?inWarranty=1");
    await expect(page.getByText("Dishwasher with warranty")).toBeVisible();
    await expect(page.getByText("Cordless drill")).toHaveCount(0);
  });

  test("a connection problem on load offers Retry instead of loading forever or signing out", async ({
    page,
  }) => {
    for (const failing of ["**/api/v1/bootstrap", "**/api/v1/session"]) {
      let failOnce = true;
      await page.route(failing, async (route) => {
        if (route.request().method() === "GET" && failOnce) {
          failOnce = false;
          await route.fulfill({
            status: 503,
            contentType: "application/json",
            body: "{}",
          });
          return;
        }
        await route.continue();
      });
      await page.goto("/items");
      await expect(
        page.getByRole("heading", { name: "Can't reach Keeps" }),
      ).toBeVisible();
      await expect(page).toHaveURL(/\/items$/);
      await page.getByRole("button", { name: "Try again" }).click();
      await expect(
        page.getByRole("heading", { name: "Your items" }),
      ).toBeVisible();
      await page.unroute(failing);
    }
  });

  test("measurements are saved with their units and shown only when given", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Wardrobe with measurements");
    await page.getByRole("button", { name: "More details" }).click();
    await page.getByLabel("Width").fill("80");
    await page.getByLabel("Height").fill("201.5");
    await page.getByLabel("Size unit").selectOption("cm");
    await page.getByLabel("Weight", { exact: true }).fill("64");
    await page.getByLabel(/Fitting note/).fill("x".repeat(1001));
    await expect(
      page.getByText(/The fitting note has 1001 characters; keep it to 1000/),
    ).toBeVisible();
    await page.getByLabel(/Fitting note/).fill("Needs an 85 cm doorway");
    await capture(page, "07-measurements-dark");
    await expectAccessible(page, "measurements");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Wardrobe with measurements" }),
    ).toBeVisible();
    await expect(page.getByText("80 × 201.5 cm (W × H)")).toBeVisible();
    await expect(page.getByText("64 kg")).toBeVisible();
    await expect(page.getByText("Needs an 85 cm doorway")).toBeVisible();
  });

  test("parts and consumables are recorded, edited and removed on the item", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Vacuum with bags");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Vacuum with bags" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Parts and consumables" }).click();
    await page.getByRole("button", { name: "Add a part" }).click();
    await page.getByLabel("Part", { exact: true }).fill("Vacuum bags");
    await page.getByLabel(/Part number/).fill("VB-12");
    await page.getByLabel(/Where to find it/).fill("not a link");
    await page.getByRole("button", { name: "Save part" }).click();
    await expect(
      page.getByText("Enter a full web address starting with https://"),
    ).toBeVisible();
    await page
      .getByLabel(/Where to find it/)
      .fill("https://maker.example/bags");
    await page.getByRole("button", { name: "Save part" }).click();
    await expect(
      page.getByRole("button", { name: "Parts and consumables (1)" }),
    ).toBeVisible();
    await expect(page.getByText("VB-12")).toBeVisible();
    await expect(page.getByRole("link", { name: "Source" })).toHaveAttribute(
      "href",
      "https://maker.example/bags",
    );
    await capture(page, "08-parts-dark");
    await expectAccessible(page, "parts");
    await page.getByRole("button", { name: "Edit Vacuum bags" }).click();
    await page.getByLabel(/Part number/).fill("VB-14");
    await page.getByRole("button", { name: "Save part" }).click();
    await expect(page.getByText("VB-14")).toBeVisible();
    await page.getByRole("button", { name: "Remove Vacuum bags" }).click();
    await expect(page.getByText("VB-14")).toHaveCount(0);
  });

  test("matching units keep their own serials, and the quantity can't fall below them", async ({
    page,
  }) => {
    const unitsPanel = page.getByRole("button", { name: /Individual units/ });
    await page.goto("/add/manual");
    await page.getByLabel(/^Name/).fill("Pair of bookshelf speakers");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Pair of bookshelf speakers" }),
    ).toBeVisible();
    const itemId = new URL(page.url()).pathname.split("/").pop();
    await expect(unitsPanel).toHaveCount(0);

    await page.getByRole("button", { name: "Edit" }).first().click();
    await page.getByLabel(/^Quantity/).fill("2");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(
      page.getByRole("button", { name: "Save changes" }),
    ).toBeHidden();

    await page.getByRole("button", { name: "Individual units" }).click();
    await page.getByRole("button", { name: "Identify a unit" }).click();
    await page.getByRole("button", { name: "Save unit" }).click();
    await expect(
      page.getByText(
        "Enter a label, like Left speaker, or the unit's serial number.",
      ),
    ).toBeVisible();
    await page.getByLabel(/^Label/).fill("Left speaker");
    await page.getByLabel(/^Serial number/).fill("SN-L1");
    await page.getByRole("button", { name: "Save unit" }).click();
    await expect(
      page.getByRole("button", { name: "Individual units (1 of 2)" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Identify a unit" }).click();
    await page.getByLabel(/^Serial number/).fill("SN-R2");
    await page.getByRole("button", { name: "Save unit" }).click();
    await expect(
      page.getByRole("button", { name: "Individual units (2 of 2)" }),
    ).toBeVisible();
    await expect(page.getByText("Unit 2", { exact: true })).toBeVisible();
    await expect(page.getByText("SN-R2")).toBeVisible();
    await expect(
      page.getByText(
        "Every unit is identified. Raise the quantity to identify another.",
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Identify a unit" }),
    ).toHaveCount(0);
    await page.getByRole("button", { name: "Edit Left speaker" }).click();
    await page.getByLabel(/^Serial number/).fill("SN-L1B");
    await page.getByRole("button", { name: "Save unit" }).click();
    await expect(page.getByText("SN-L1B")).toBeVisible();
    await capture(page, "08-units-dark");
    await expectAccessible(page, "units");

    // Lowering the quantity asks which unit is gone instead of guessing.
    await page.getByRole("button", { name: "Edit" }).first().click();
    await page.getByLabel(/^Quantity/).fill("1");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(
      page.getByText(/2 units are identified under Individual units/),
    ).toBeVisible();
    await page.getByRole("button", { name: "Remove Unit 2" }).click();
    await expect(
      page.getByRole("button", { name: "Individual units (1 of 2)" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(
      page.getByRole("button", { name: "Save changes" }),
    ).toBeHidden();

    const saved = await (
      await page.request.get(`/api/v1/items/${itemId}`)
    ).json();
    expect(saved.quantity).toBe(1);
    expect(saved.units).toEqual([
      expect.objectContaining({
        label: "Left speaker",
        serialNumber: "SN-L1B",
      }),
    ]);
    await expect(page.getByText("SN-L1B")).toBeVisible();
  });

  test("a preset fills only empty details and keeps what was typed", async ({
    page,
    context,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Spanner set");
    await page.getByRole("button", { name: "More details" }).click();
    await page.getByLabel(/^Room/).fill("Garage");
    await page.getByLabel(/^Category/).fill("Tools");
    await page.getByRole("button", { name: "Use a preset" }).click();
    const dialog = page.getByRole("dialog", { name: "Presets" });
    await dialog
      .getByLabel("Save these details as a preset")
      .fill("Garage tools");
    await dialog.getByRole("button", { name: "Save preset" }).click();
    await expect(
      dialog.getByRole("radio", { name: "Garage tools" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel" }).click();

    const fresh = await context.newPage();
    await fresh.goto("/add/manual");
    await fresh.getByLabel("Name").fill("Hammer");
    await fresh.getByRole("button", { name: "More details" }).click();
    await fresh.getByLabel(/^Category/).fill("Hand tools");
    await fresh.getByRole("button", { name: "Use a preset" }).click();
    const presets = fresh.getByRole("dialog", { name: "Presets" });
    await presets.getByRole("radio", { name: "Garage tools" }).check();
    await expect(presets.getByText("Room: Garage")).toBeVisible();
    await expect(
      presets.getByText("Keeps what you typed in: Category."),
    ).toBeVisible();
    await capture(fresh, "07-presets-dark");
    await expectAccessible(fresh, "presets");
    await presets.getByRole("button", { name: "Fill in" }).click();
    await expect(fresh.getByLabel(/^Room/)).toHaveValue("Garage");
    await expect(fresh.getByLabel(/^Category/)).toHaveValue("Hand tools");
    await fresh.getByRole("button", { name: "Save item" }).click();
    await expect(fresh.getByRole("heading", { name: "Hammer" })).toBeVisible();
    await expect(fresh.getByText("Hand tools")).toBeVisible();
    await fresh.close();
  });

  test("a conflict keeps my condition and warranty available through Use mine", async ({
    page,
    context,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Fridge edited in two tabs");
    await page.getByLabel(/Condition/).selectOption({ label: "Good" });
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Fridge edited in two tabs" }),
    ).toBeVisible();
    const itemUrl = page.url();

    const other = await context.newPage();
    await other.goto(itemUrl);
    await page.getByRole("button", { name: "Edit" }).first().click();
    await other.getByRole("button", { name: "Edit" }).first().click();
    await other.getByLabel(/Condition/).selectOption({ label: "Fair" });
    await other.getByRole("button", { name: "Save changes" }).click();
    await expect(other.getByText("Fair", { exact: true })).toBeVisible();

    await page.getByLabel(/Condition/).selectOption({ label: "Poor" });
    await page.getByLabel(/Warranty until/).fill("2099-01-31");
    await page.getByRole("button", { name: "Save changes" }).click();
    await page.getByRole("button", { name: "Reload" }).click();
    await expect(page.getByLabel(/Condition/)).toHaveValue("fair");
    const useMine = page.getByRole("button", { name: "Use mine" });
    await expect(useMine).toHaveCount(2);
    await useMine.first().click();
    await useMine.first().click();
    await expect(page.getByLabel(/Condition/)).toHaveValue("poor");
    await expect(page.getByLabel(/Warranty until/)).toHaveValue("2099-01-31");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(page.getByText("Poor", { exact: true })).toBeVisible();
    await expect(page.getByText(/In warranty until/)).toBeVisible();
    await other.close();
  });

  test("an open edit doesn't overwrite another tab's save after adding a part", async ({
    page,
    context,
  }) => {
    const conflictTitle = page.getByText(/was changed somewhere else/);
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Lamp edited around a part");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Lamp edited around a part" }),
    ).toBeVisible();
    const itemUrl = page.url();
    const itemId = new URL(itemUrl).pathname.split("/").pop();

    await page.getByRole("button", { name: "Edit" }).first().click();
    await page.getByLabel(/^Notes/).fill("Bulb is E14");

    const other = await context.newPage();
    await other.goto(itemUrl);
    await other.getByRole("button", { name: "Edit" }).first().click();
    await other.getByLabel(/^Name/).fill("Lamp renamed elsewhere");
    await other.getByRole("button", { name: "Save changes" }).click();
    await expect(
      other.getByRole("heading", { name: "Lamp renamed elsewhere" }),
    ).toBeVisible();

    // A part saved while the form is open refreshes the item behind it.
    await page.getByRole("button", { name: "Parts and consumables" }).click();
    await page.getByRole("button", { name: "Add a part" }).click();
    await page.getByLabel("Part", { exact: true }).fill("E14 bulb");
    await page.getByRole("button", { name: "Save part" }).click();
    await expect(
      page.getByRole("button", { name: "Parts and consumables (1)" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(conflictTitle).toBeVisible();
    await page.getByRole("button", { name: "Reload" }).click();
    await expect(page.getByLabel(/^Name/)).toHaveValue(
      "Lamp renamed elsewhere",
    );
    await page.getByLabel(/^Notes/).fill("Bulb is E14");

    // Another save lands after the comparison: that one is caught too.
    await other.getByRole("button", { name: "Edit" }).first().click();
    await other.getByLabel(/Condition/).selectOption({ label: "Fair" });
    await other.getByRole("button", { name: "Save changes" }).click();
    await expect(other.getByText("Fair", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(conflictTitle).toBeVisible();
    await page.getByRole("button", { name: "Reload" }).click();
    await expect(page.getByLabel(/Condition/)).toHaveValue("fair");
    await page.getByLabel(/^Notes/).fill("Bulb is E14");
    await page.getByRole("button", { name: "Save changes" }).click();
    await expect(
      page.getByRole("button", { name: "Save changes" }),
    ).toBeHidden();

    const saved = await (
      await page.request.get(`/api/v1/items/${itemId}`)
    ).json();
    expect(saved.name).toBe("Lamp renamed elsewhere");
    expect(saved.notes).toBe("Bulb is E14");
    expect(saved.condition).toBe("fair");
    expect(saved.parts).toHaveLength(1);
    await other.close();
  });

  test("back arrows return to the page you came from", async ({ page }) => {
    await page.goto("/account/data");
    await page
      .getByRole("link", { name: /View/ })
      .or(page.getByRole("button", { name: /View/ }))
      .first()
      .click();
    await expect(
      page.getByRole("heading", { name: "Recently deleted" }),
    ).toBeVisible();
    await page.getByRole("link", { name: "Back" }).click();
    await expect(page).toHaveURL(/\/account\/data$/);

    await page.goto("/trash");
    await page.getByRole("link", { name: "Back" }).click();
    await expect(page).toHaveURL(/\/items$/);
  });

  test("appearance switches to light instantly and persists", async ({
    page,
  }) => {
    await page.goto("/account/appearance");
    await page.getByRole("radio", { name: /Light/ }).check();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    for (const [route, name] of [
      ["/", "03-home-light"],
      ["/items", "03-items-light"],
      ["/add/manual", "07-add-manually-light"],
      ["/organise/rooms", "09-organise-light"],
      ["/account/account", "11-account-light"],
      ["/account/data", "12-data-light"],
    ]) {
      await page.goto(route);
      await page.waitForLoadState("networkidle");
      await capture(page, name);
      await expectAccessible(page, `${name}`);
    }
    await page.goto("/account/appearance");
    await page.getByRole("radio", { name: /Dark/ }).check();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  });

  test("optional authenticator: enrol, then sign-in requires the code", async ({
    page,
    browser,
  }) => {
    // Waits for a fresh 30-second authenticator code.
    test.setTimeout(TOTP_TEST_TIMEOUT_MS);
    await page.goto("/account/account");
    await capture(page, "11-account-dark");
    await expectAccessible(page, "account");
    await page.goto("/account/security");
    await capture(page, "11-security-dark");
    await page.getByRole("button", { name: "Set up" }).nth(1).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.locator("code")).toBeVisible();
    totpSecret = (await dialog.locator("code").textContent())!.replace(
      /\s/g,
      "",
    );
    await expectAccessible(page, "authenticator setup");
    await dialog
      .getByLabel("6-digit code from the app")
      .fill(await freshTotp());
    await dialog.getByRole("button", { name: "Verify" }).click();
    await expect(
      page.getByRole("heading", { name: /Save your recovery codes/i }),
    ).toBeVisible();
    await page.getByLabel(/saved these codes/i).check();
    await page.getByRole("button", { name: "Done" }).click();
    await expect(page.getByRole("button", { name: "Turn off" })).toBeVisible();

    const fresh = await browser.newContext({
      storageState: { cookies: [], origins: [] },
    });
    const signIn = await fresh.newPage();
    await signIn.goto("/sign-in");
    await capture(signIn, "02-sign-in-dark");
    await expectAccessible(signIn, "sign in");
    await signIn.getByLabel("Username").fill(USERNAME);
    await signIn.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await signIn.getByRole("button", { name: "Sign in" }).click();
    await expect(
      signIn.getByRole("heading", { name: /authenticator code/i }),
    ).toBeVisible();
    await signIn.getByLabel("Authenticator code").fill(await freshTotp());
    await signIn.getByRole("button", { name: "Verify" }).click();
    await expect(
      signIn.getByRole("link", { name: "Items" }).first(),
    ).toBeVisible();
    await fresh.close();
  });

  test("passkey: add with a virtual authenticator, then sign in with it", async ({
    page,
    browser,
  }) => {
    const authenticatorOptions = {
      protocol: "ctap2" as const,
      transport: "internal" as const,
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    };
    const client = await page.context().newCDPSession(page);
    await client.send("WebAuthn.enable");
    const { authenticatorId } = await client.send(
      "WebAuthn.addVirtualAuthenticator",
      { options: authenticatorOptions },
    );
    await page.goto("/account/security");
    await page.getByRole("button", { name: "Set up" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Passkey name").fill("E2E key");
    await dialog.getByRole("button", { name: "Continue" }).click();
    await expect(page.getByText("E2E key")).toBeVisible();
    const { credentials } = await client.send("WebAuthn.getCredentials", {
      authenticatorId,
    });
    expect(credentials.length).toBe(1);

    const fresh = await browser.newContext({
      storageState: { cookies: [], origins: [] },
    });
    const signIn = await fresh.newPage();
    const freshClient = await fresh.newCDPSession(signIn);
    await freshClient.send("WebAuthn.enable");
    const second = await freshClient.send("WebAuthn.addVirtualAuthenticator", {
      options: authenticatorOptions,
    });
    await freshClient.send("WebAuthn.addCredential", {
      authenticatorId: second.authenticatorId,
      credential: credentials[0],
    });
    await signIn.goto("/sign-in");
    await signIn.getByRole("button", { name: "Use passkey" }).click();
    await expect(
      signIn.getByRole("link", { name: "Items" }).first(),
    ).toBeVisible();
    await fresh.close();
  });

  test("an expired session mid-save stays in place: Escape keeps the dialog, Start over works, the draft survives", async ({
    page,
  }) => {
    test.setTimeout(TOTP_TEST_TIMEOUT_MS);
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Draft that survives expiry");
    let expireOnce = true;
    await page.route("**/api/v1/items", async (route) => {
      if (route.request().method() === "POST" && expireOnce) {
        expireOnce = false;
        await route.fulfill({
          status: 401,
          contentType: "application/json",
          body: JSON.stringify({
            code: "unauthorized",
            message: "Sign in again.",
          }),
        });
        return;
      }
      await route.continue();
    });
    await page.getByRole("button", { name: "Save item" }).click();
    const dialog = page.getByRole("dialog", { name: "Sign in to continue" });
    await expect(dialog).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Close" })).toHaveCount(0);

    await dialog.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await dialog.getByRole("button", { name: "Sign in" }).click();
    await expect(
      dialog.getByRole("button", { name: "Use a recovery code instead" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Start over" }).click();
    await dialog.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await dialog.getByRole("button", { name: "Sign in" }).click();
    await dialog
      .getByLabel(/code/i)
      .first()
      .fill(await freshTotp());
    await dialog.getByRole("button", { name: "Verify" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByLabel("Name")).toHaveValue(
      "Draft that survives expiry",
    );
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Draft that survives expiry" }),
    ).toBeVisible();
    await page.unroute("**/api/v1/items");
  });

  test("data: export, backup and recently deleted", async ({ page }) => {
    await page.goto("/account/data");
    await capture(page, "12-data-dark");
    await expectAccessible(page, "data");
    await page.goto("/items");
    await page
      .getByRole("button", { name: /^Actions for/ })
      .first()
      .click();
    await page.getByRole("menuitem", { name: "Move to trash" }).click();
    await expect(page.getByText("Moved to Recently deleted")).toBeVisible();
    await page.goto("/trash");
    await expect(
      page.getByRole("button", { name: "Restore" }).first(),
    ).toBeVisible();
    await capture(page, "17-trash-dark");
    await expectAccessible(page, "trash");
    await page.getByRole("button", { name: "Restore" }).first().click();
    await expect(page.getByText(/Restored/)).toBeVisible();
  });

  test("phone layout keeps search and add reachable", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/items");
    await expect(page.getByRole("link", { name: /Add item/ })).toBeVisible();
    await expect(
      page.getByRole("navigation", { name: "Main" }).last(),
    ).toBeVisible();
    const overflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
    expect(overflow).toBe(false);
    await capture(page, "mobile-items-dark");
    await page.goto("/add/manual");
    await capture(page, "mobile-add-manually-dark");
    await page.goto("/account/account");
    const accountOverflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
    expect(accountOverflow).toBe(false);
    await capture(page, "mobile-account-dark");
  });
});

test.describe("add to keeps button", () => {
  test.use({ storageState });

  test("the bookmark button reads the open shop page and opens the review", async ({
    page,
    context,
  }) => {
    await page.goto("/add/button");
    await capture(page, "add-button-setup-dark");
    await expectAccessible(page, "button setup");
    const bookmark = await page
      .getByText("Add to Keeps", { exact: true })
      .locator("xpath=ancestor-or-self::a")
      .getAttribute("href");
    expect(bookmark?.startsWith("javascript:")).toBe(true);

    await context.route("https://shop.example/**", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: `<html><head><title>Sony BRAVIA 3 | Shop</title>
          <script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Sony BRAVIA 3 55 inch","brand":{"@type":"Brand","name":"Sony"},"description":"A television.","offers":{"@type":"Offer","price":"599.00","priceCurrency":"GBP"}}</script>
          </head><body><h1>Sony BRAVIA 3</h1></body></html>`,
      }),
    );
    const shop = await context.newPage();
    await shop.goto("https://shop.example/tv/123");
    const keepsTab = context.waitForEvent("page");
    await shop.evaluate(
      decodeURIComponent(bookmark!.slice("javascript:".length)),
    );
    const review = await keepsTab;
    await expect(
      review.getByRole("heading", { name: "Review item" }),
    ).toBeVisible();
    await expect(
      review.getByRole("textbox", { name: /^Name/ }).first(),
    ).toHaveValue("Sony BRAVIA 3 55 inch");
    await expect(review.getByText("£599").first()).toBeVisible();
    await expect(review.getByText(/shop\.example/).first()).toBeVisible();
    await capture(review, "05-review-from-button-dark");
    await expectAccessible(review, "review from button");
    expect(review.url()).not.toContain("#");
  });

  test("a lost status check or a long wait looks at the same import again instead of starting another", async ({
    page,
    context,
  }) => {
    await page.goto("/add/button");
    const buttonLink = page
      .getByText("Add to Keeps", { exact: true })
      .locator("xpath=ancestor-or-self::a");
    await expect(buttonLink).toHaveAttribute("href", /^javascript:/);
    const bookmark = await buttonLink.getAttribute("href");
    await context.route("https://shop.example/**", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: `<html><head><title>Desk fan | Shop</title>
          <script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Quiet desk fan","offers":{"@type":"Offer","price":"39.00","priceCurrency":"GBP"}}</script>
          </head><body></body></html>`,
      }),
    );
    let importsStarted = 0;
    let statusChecks: "lose" | "queued" | "real" = "lose";
    await context.route("**/api/v1/imports**", async (route) => {
      const request = route.request();
      if (request.method() === "POST") {
        importsStarted++;
        await route.fallback();
        return;
      }
      if (request.method() !== "GET" || statusChecks === "real") {
        await route.fallback();
        return;
      }
      if (statusChecks === "lose") {
        statusChecks = "real";
        await route.abort("internetdisconnected");
        return;
      }
      // Answer as if the job were still waiting behind others.
      const response = await route.fetch();
      await route.fulfill({
        response,
        json: { ...(await response.json()), state: "queued" },
      });
    });
    const shop = await context.newPage();
    await shop.goto("https://shop.example/office/fan");
    const keepsTab = context.waitForEvent("page");
    await shop.evaluate(
      decodeURIComponent(bookmark!.slice("javascript:".length)),
    );
    const importPage = await keepsTab;
    const checkAgain = importPage.getByRole("button", { name: "Check again" });
    const review = importPage.getByRole("heading", { name: "Review item" });
    await expect(checkAgain).toBeVisible();
    await expect(importPage.getByText(/Couldn't reach Keeps/)).toBeVisible();
    await checkAgain.click();
    await expect(review).toBeVisible();
    await expect(
      importPage.getByRole("textbox", { name: /^Name/ }).first(),
    ).toHaveValue("Quiet desk fan");
    expect(importsStarted).toBe(1);

    // The same import, kept waiting past the page's own limit.
    await importPage.clock.install();
    statusChecks = "queued";
    await importPage.reload();
    await expect(
      importPage.getByText(/Getting details from shop\.example/),
    ).toBeVisible();
    await importPage.clock.fastForward("00:30");
    await expect(checkAgain).toBeVisible();
    await expect(
      importPage.getByText("Still getting the details"),
    ).toBeVisible();
    statusChecks = "real";
    await checkAgain.click();
    await expect(review).toBeVisible();
    expect(importsStarted).toBe(1);
  });

  test("a failed button capture can be retried after a refresh, or entered manually with its details", async ({
    page,
    context,
  }) => {
    await page.goto("/add/button");
    const buttonLink = page
      .getByText("Add to Keeps", { exact: true })
      .locator("xpath=ancestor-or-self::a");
    await expect(buttonLink).toHaveAttribute("href", /^javascript:/);
    const bookmark = await buttonLink.getAttribute("href");
    await context.route("https://shop.example/**", (route) =>
      route.fulfill({
        contentType: "text/html",
        body: `<html><head><title>Garden bench | Shop</title>
          <script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Oak garden bench","offers":{"@type":"Offer","price":"149.00","priceCurrency":"GBP"}}</script>
          </head><body></body></html>`,
      }),
    );
    let failures = 3;
    await context.route("**/api/v1/imports/page", async (route) => {
      if (failures > 0) {
        failures--;
        await route.abort("internetdisconnected");
        return;
      }
      await route.continue();
    });
    const shop = await context.newPage();
    await shop.goto("https://shop.example/garden/bench");
    const keepsTab = context.waitForEvent("page");
    await shop.evaluate(
      decodeURIComponent(bookmark!.slice("javascript:".length)),
    );
    const capturePage = await keepsTab;
    await expect(
      capturePage.getByRole("button", { name: "Try again" }),
    ).toBeVisible();
    expect(capturePage.url()).not.toContain("#");

    await capturePage.reload();
    await expect(
      capturePage.getByRole("button", { name: "Try again" }),
    ).toBeVisible();
    await capturePage.getByRole("button", { name: "Try again" }).click();
    await expect(
      capturePage.getByRole("button", { name: "Enter it manually" }),
    ).toBeVisible();
    await capturePage
      .getByRole("button", { name: "Enter it manually" })
      .click();
    await expect(capturePage.getByLabel("Name")).toHaveValue(
      "Garden bench | Shop",
    );
    await expect(
      capturePage.getByText(/shop\.example\/garden\/bench/).first(),
    ).toBeVisible();

    await capturePage.goto("/add/from-page");
    await expect(
      capturePage.getByText(/couldn't read what the button sent/i),
    ).toBeVisible();
    await context.unroute("**/api/v1/imports/page");
  });

  test("long descriptions show eight lines, then Show more", async ({
    page,
  }) => {
    const description = Array.from(
      { length: 20 },
      (_, index) => `Line ${index + 1} of a long product description.`,
    ).join("\n");
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Television with a long description");
    await page.getByRole("button", { name: "More details" }).click();
    await page.getByLabel("Description").fill(description);
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Television with a long description" }),
    ).toBeVisible();
    const visibleLines = await page
      .getByText("Line 1 of a long product description.", { exact: false })
      .evaluate((element) => {
        const lineHeight = parseFloat(getComputedStyle(element).lineHeight);
        return Math.round(element.clientHeight / lineHeight);
      });
    expect(visibleLines).toBe(8);
    const showMore = page.getByRole("button", { name: "Show more" });
    await expect(showMore).toBeVisible();
    await showMore.click();
    await expect(
      page.getByRole("button", { name: "Show less" }),
    ).toHaveAttribute("aria-expanded", "true");
    await capture(page, "08-long-description-expanded-dark");
  });
});

test.describe("photos and documents", () => {
  test.use({ storageState });

  function coloredPng(red: number, green: number, blue: number) {
    // Builds a small valid PNG of one colour so each photo is distinguishable.
    const width = 8;
    const rows = [];
    for (let row = 0; row < width; row++) {
      rows.push(
        Buffer.from([
          0,
          ...Array.from({ length: width }, () => [red, green, blue]).flat(),
        ]),
      );
    }
    const raw = Buffer.concat(rows);
    const crcTable = Array.from({ length: 256 }, (_, n) => {
      let c = n;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      return c >>> 0;
    });
    const crc = (buffer: Buffer) => {
      let c = 0xffffffff;
      for (const byte of buffer) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
      return (c ^ 0xffffffff) >>> 0;
    };
    const chunk = (type: string, data: Buffer) => {
      const length = Buffer.alloc(4);
      length.writeUInt32BE(data.length);
      const body = Buffer.concat([Buffer.from(type), data]);
      const checksum = Buffer.alloc(4);
      checksum.writeUInt32BE(crc(body));
      return Buffer.concat([length, body, checksum]);
    };
    const header = Buffer.alloc(13);
    header.writeUInt32BE(width, 0);
    header.writeUInt32BE(width, 4);
    header[8] = 8;
    header[9] = 2;
    return Buffer.concat([
      Buffer.from("\x89PNG\r\n\x1a\n", "binary"),
      chunk("IHDR", header),
      chunk("IDAT", deflateSync(raw)),
      chunk("IEND", Buffer.alloc(0)),
    ]);
  }

  function simplePdf(text: string) {
    const stream = `BT /F1 24 Tf 72 720 Td (${text}) Tj ET`;
    const objects = [
      "<< /Type /Catalog /Pages 2 0 R >>",
      "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
      "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
      `<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`,
      "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    ];
    let body = "%PDF-1.4\n";
    const offsets: number[] = [];
    objects.forEach((object, index) => {
      offsets.push(body.length);
      body += `${index + 1} 0 obj\n${object}\nendobj\n`;
    });
    const xref = body.length;
    body += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n${offsets.map((offset) => `${String(offset).padStart(10, "0")} 00000 n \n`).join("")}`;
    body += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF`;
    return Buffer.from(body, "binary");
  }

  test("several photos: thumbnails while adding, swipe and arrows, full-size viewer", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Bookshelf with three photos");
    await page
      .locator('input[type="file"]')
      .first()
      .setInputFiles([
        {
          name: "red.png",
          mimeType: "image/png",
          buffer: coloredPng(200, 40, 40),
        },
        {
          name: "green.png",
          mimeType: "image/png",
          buffer: coloredPng(40, 200, 40),
        },
        {
          name: "blue.png",
          mimeType: "image/png",
          buffer: coloredPng(40, 40, 200),
        },
      ]);
    await expect(page.locator('img[src*="variant=thumbnail"]')).toHaveCount(3);
    await capture(page, "07-add-several-photos-dark");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Bookshelf with three photos" }),
    ).toBeVisible();

    const counter = page
      .locator('[aria-live="polite"]')
      .filter({ hasText: /of 3/ })
      .first();
    await expect(counter).toHaveText("1 of 3");
    await page.getByRole("button", { name: "Next photo" }).click();
    await expect(counter).toHaveText("2 of 3");
    await page
      .locator('[class*="photoTrack"]')
      .evaluate((track) =>
        track.scrollTo({ left: track.clientWidth * 2, behavior: "instant" }),
      );
    await expect(counter).toHaveText("3 of 3");
    await capture(page, "08-gallery-dark");

    await page
      .getByRole("button", { name: /View full size — Photo 3 of 3/ })
      .click();
    const viewer = page.getByRole("dialog", { name: "Photo viewer" });
    await expect(viewer).toBeVisible();
    await expect(viewer.getByText("3 of 3")).toBeVisible();
    await page.keyboard.press("ArrowLeft");
    await expect(viewer.getByText("2 of 3")).toBeVisible();
    await capture(page, "08-photo-viewer");
    await expectAccessible(page, "photo viewer");
    await page.keyboard.press("Escape");
    await expect(viewer).toBeHidden();

    await page
      .locator('input[type="file"]')
      .first()
      .setInputFiles({
        name: "extra.png",
        mimeType: "image/png",
        buffer: coloredPng(220, 200, 40),
      });
    await expect(
      page.locator('[aria-live="polite"]').filter({ hasText: /of 4/ }).first(),
    ).toHaveText(/ of 4$/);
  });

  test("a receipt added later that uploaded but was not added can be added again without re-uploading, and counts as a receipt", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Kettle with a late receipt");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Kettle with a late receipt" }),
    ).toBeVisible();
    await page.goto(new URL("documents", page.url() + "/").pathname);

    let uploads = 0;
    page.on("request", (request) => {
      if (
        request.method() === "POST" &&
        request.url().includes("/api/v1/attachments?")
      ) {
        uploads++;
      }
    });
    let failNextLink = true;
    await page.route("**/api/v1/items/*/attachments", async (route) => {
      if (route.request().method() === "POST" && failNextLink) {
        failNextLink = false;
        await route.fulfill({
          status: 503,
          contentType: "application/json",
          body: JSON.stringify({
            code: "maintenance",
            message: "Keeps is busy. Try again in a minute.",
          }),
        });
        return;
      }
      await route.continue();
    });

    await page
      .locator('input[type="file"]')
      .nth(1)
      .setInputFiles({
        name: "kettle-receipt.pdf",
        mimeType: "application/pdf",
        buffer: simplePdf("Kettle receipt"),
      });
    await expect(
      page.getByText("Keeps is busy. Try again in a minute."),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Retry kettle-receipt.pdf" })
      .click();
    await expect(
      page.getByRole("button", { name: "View kettle-receipt.pdf" }),
    ).toBeVisible();
    expect(uploads).toBe(1);
    await page.goto("/items?hasReceipt=1");
    await expect(page.getByText("Kettle with a late receipt")).toBeVisible();
    await page.unroute("**/api/v1/items/*/attachments");
  });

  test("documents load page by page, and a failed page can be retried", async ({
    page,
  }) => {
    const entry = (id: number, filename: string) => ({
      id,
      kind: "document",
      filename,
      mediaType: "application/pdf",
      sizeBytes: 1000,
      createdAt: "2026-01-01T00:00:00Z",
      hasPreview: false,
      shared: false,
      itemId: id,
      itemName: `Item ${id}`,
    });
    let failNextPage = true;
    await page.route("**/api/v1/documents?*", async (route) => {
      const cursor = new URL(route.request().url()).searchParams.get("cursor");
      if (!cursor) {
        await route.fulfill({
          json: {
            documents: [entry(1, "first-page.pdf")],
            total: 2,
            nextCursor: "page-2",
          },
        });
        return;
      }
      if (failNextPage) {
        failNextPage = false;
        await route.fulfill({
          status: 503,
          json: {
            code: "maintenance",
            message: "Keeps is busy. Try again in a minute.",
          },
        });
        return;
      }
      await route.fulfill({
        json: { documents: [entry(2, "second-page.pdf")], total: 2 },
      });
    });
    await page.goto("/documents");
    await expect(page.getByText("first-page.pdf")).toBeVisible();
    await expect(page.getByText("Showing 1 of 2")).toBeVisible();
    await page.getByRole("button", { name: "Load more" }).click();
    await expect(
      page.getByText("Keeps is busy. Try again in a minute."),
    ).toBeVisible();
    await page.getByRole("button", { name: "Try loading more again" }).click();
    await expect(page.getByText("second-page.pdf")).toBeVisible();
    await expect(page.getByText("first-page.pdf")).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Load more|Try loading more/ }),
    ).toHaveCount(0);
    await page.unroute("**/api/v1/documents?*");
  });

  test("documents open inside Keeps: a PDF is drawn on the page", async ({
    page,
  }) => {
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Washing machine with invoice");
    await page
      .locator('input[type="file"]')
      .nth(1)
      .setInputFiles({
        name: "washing-machine-invoice.pdf",
        mimeType: "application/pdf",
        buffer: simplePdf("Invoice 42 - Washing machine"),
      });
    await expect(page.getByText("washing-machine-invoice.pdf")).toBeVisible();
    await page.getByRole("button", { name: "Save item" }).click();
    await action(page, "Documents").click();
    await page
      .getByRole("button", { name: "View washing-machine-invoice.pdf" })
      .click();
    const viewer = page.getByRole("dialog", {
      name: "washing-machine-invoice.pdf",
    });
    await expect(viewer.getByText("1 page")).toBeVisible({ timeout: 15_000 });
    const firstPage = viewer.getByRole("region", { name: "Page 1" });
    await expect(firstPage).toContainText("Invoice 42 - Washing machine");
    const pageCanvas = firstPage.locator("canvas");
    await expect(pageCanvas).toBeVisible();
    await expect
      .poll(() =>
        pageCanvas.evaluate((canvas: HTMLCanvasElement) => {
          const context = canvas.getContext("2d");
          if (!context || canvas.width === 0) return 0;
          const pixels = context.getImageData(
            0,
            0,
            canvas.width,
            canvas.height,
          ).data;
          let dark = 0;
          for (let index = 0; index < pixels.length; index += 4)
            if (pixels[index] < 100) dark++;
          return dark;
        }),
      )
      .toBeGreaterThan(50);
    await capture(page, "10-document-viewer-dark");
    await expectAccessible(page, "document viewer");
    await viewer.getByRole("button", { name: "Zoom in" }).click();
    await expect(viewer.getByText("125%")).toBeVisible();
    await viewer.getByLabel("Name for this page").fill("Invoice total");
    await viewer.getByRole("button", { name: "Mark page 1" }).click();
    await expect(
      viewer.getByRole("button", { name: "Go to page 1: Invoice total" }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(viewer).toBeHidden();
    await page
      .getByRole("button", { name: "View washing-machine-invoice.pdf" })
      .click();
    await expect(
      viewer.getByRole("button", { name: "Go to page 1: Invoice total" }),
    ).toBeVisible();
    await viewer
      .getByRole("button", { name: "Remove bookmark Invoice total" })
      .click();
    await expect(
      viewer.getByRole("button", { name: "Go to page 1: Invoice total" }),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");
  });

  test("an interrupted cover is dropped and a transparent photo looks the same in the copy", async ({
    page,
  }) => {
    // 40×40 RGBA PNG: transparent, with an opaque white mark top-left and an
    // opaque red block on the right half.
    const size = 40;
    const pixelFor = (mark: boolean, red: boolean) => {
      if (mark) return [255, 255, 255, 255];
      if (red) return [220, 0, 0, 255];
      return [0, 0, 0, 0];
    };
    const rows: Buffer[] = [];
    for (let y = 0; y < size; y++) {
      const row = [0];
      for (let x = 0; x < size; x++) {
        const mark = x >= 4 && x < 14 && y >= 4 && y < 14;
        const red = x >= size / 2;
        row.push(...pixelFor(mark, red));
      }
      rows.push(Buffer.from(row));
    }
    const crcTable = Array.from({ length: 256 }, (_, n) => {
      let c = n;
      for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      return c >>> 0;
    });
    const crc = (buffer: Buffer) => {
      let c = 0xffffffff;
      for (const byte of buffer) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
      return (c ^ 0xffffffff) >>> 0;
    };
    const chunk = (type: string, data: Buffer) => {
      const length = Buffer.alloc(4);
      length.writeUInt32BE(data.length);
      const body = Buffer.concat([Buffer.from(type), data]);
      const checksum = Buffer.alloc(4);
      checksum.writeUInt32BE(crc(body));
      return Buffer.concat([length, body, checksum]);
    };
    const header = Buffer.alloc(13);
    header.writeUInt32BE(size, 0);
    header.writeUInt32BE(size, 4);
    header[8] = 8;
    header[9] = 6;
    const png = Buffer.concat([
      Buffer.from("\x89PNG\r\n\x1a\n", "binary"),
      chunk("IHDR", header),
      chunk("IDAT", deflateSync(Buffer.concat(rows))),
      chunk("IEND", Buffer.alloc(0)),
    ]);

    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Transparent label check");
    await page.locator('input[type="file"]').nth(1).setInputFiles({
      name: "transparent.png",
      mimeType: "image/png",
      buffer: png,
    });
    await expect(page.getByText("transparent.png")).toBeVisible();
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Transparent label check" }),
    ).toBeVisible();
    await page.goto(new URL("documents", page.url() + "/").pathname);
    await page
      .getByRole("button", { name: "Create a sharing copy of transparent.png" })
      .click();
    const dialog = page.getByRole("dialog", {
      name: "Sharing copy of transparent.png",
    });
    const canvas = dialog.getByRole("img", { name: "Page 1" });
    await expect(canvas).toBeVisible();
    const download = dialog.getByRole("button", {
      name: "Download sharing copy",
    });

    const pointer = (type: string, x: number, y: number) =>
      canvas.evaluate(
        (element, event) => {
          const bounds = element.getBoundingClientRect();
          element.dispatchEvent(
            new PointerEvent(event.type, {
              bubbles: true,
              pointerId: 7,
              clientX: bounds.left + bounds.width * event.x,
              clientY: bounds.top + bounds.height * event.y,
            }),
          );
        },
        { type, x, y },
      );
    const previewAt = (x: number, y: number) =>
      canvas.evaluate(
        (element, point) => {
          const surface = element as HTMLCanvasElement;
          const context = surface.getContext("2d")!;
          return Array.from(
            context
              .getImageData(
                Math.floor(surface.width * point.x),
                Math.floor(surface.height * point.y),
                1,
                1,
              )
              .data.slice(0, 3),
          );
        },
        { x, y },
      );
    await pointer("pointerdown", 0.6, 0.2);
    await pointer("pointermove", 0.95, 0.9);
    await expect(download).toBeDisabled();
    expect(Math.max(...(await previewAt(0.8, 0.5)))).toBeLessThan(40);
    await pointer("pointercancel", 0.95, 0.9);
    await expect(download).toBeEnabled();
    await expect(dialog.getByText("0 areas covered")).toBeVisible();
    const previewRed = await previewAt(0.8, 0.5);
    const previewClear = await previewAt(0.2, 0.8);
    expect(previewRed[0]).toBeGreaterThan(180);
    expect(Math.min(...previewClear)).toBeGreaterThan(240);

    const downloadPromise = page.waitForEvent("download");
    await download.click();
    const file = await downloadPromise;
    const copyPath = path.join(dataDirectory, file.suggestedFilename());
    await file.saveAs(copyPath);
    const decoded = await page.evaluate(async (encoded) => {
      const binary = Uint8Array.from(atob(encoded), (character) =>
        character.charCodeAt(0),
      );
      const bitmap = await createImageBitmap(
        new Blob([binary], { type: "image/jpeg" }),
      );
      const surface = new OffscreenCanvas(bitmap.width, bitmap.height);
      const context = surface.getContext("2d")!;
      context.drawImage(bitmap, 0, 0);
      const at = (x: number, y: number) =>
        Array.from(
          context
            .getImageData(
              Math.floor(bitmap.width * x),
              Math.floor(bitmap.height * y),
              1,
              1,
            )
            .data.slice(0, 3),
        );
      return { red: at(0.8, 0.5), clear: at(0.2, 0.8), mark: at(0.22, 0.22) };
    }, readFileSync(copyPath).toString("base64"));
    expect(decoded.red[0]).toBeGreaterThan(180);
    expect(decoded.red[1]).toBeLessThan(60);
    expect(Math.min(...decoded.clear)).toBeGreaterThan(215);
    expect(Math.min(...decoded.mark)).toBeGreaterThan(215);
    await dialog.getByRole("button", { name: "Cancel" }).click();
  });

  test("a sharing copy hides what was covered and carries nothing but pixels", async ({
    page,
  }) => {
    await page.goto("/documents");
    await page
      .getByRole("button", {
        name: "Create a sharing copy of washing-machine-invoice.pdf",
      })
      .click();
    const dialog = page.getByRole("dialog", {
      name: "Sharing copy of washing-machine-invoice.pdf",
    });
    const canvas = dialog.getByRole("img", { name: "Page 1" });
    await expect(canvas).toBeVisible();
    await expect(dialog.getByText("0 areas covered")).toBeVisible();
    const bounds = (await canvas.boundingBox())!;
    await page.mouse.move(
      bounds.x + bounds.width * 0.05,
      bounds.y + bounds.height * 0.05,
    );
    await page.mouse.down();
    await page.mouse.move(
      bounds.x + bounds.width * 0.95,
      bounds.y + bounds.height * 0.3,
      { steps: 5 },
    );
    await page.mouse.up();
    await expect(dialog.getByText("1 area covered")).toBeVisible();
    await capture(page, "10-sharing-copy-dark");
    await expectAccessible(page, "sharing copy");

    const downloadPromise = page.waitForEvent("download");
    await dialog.getByRole("button", { name: "Download sharing copy" }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe(
      "washing-machine-invoice (sharing copy).pdf",
    );
    const copyPath = path.join(dataDirectory, download.suggestedFilename());
    await download.saveAs(copyPath);
    const bytes = readFileSync(copyPath);
    const text = bytes.toString("latin1");
    expect(text.startsWith("%PDF-1.4")).toBe(true);
    expect(text).toContain("/DCTDecode");
    for (const forbidden of [
      "Invoice 42",
      "/Font",
      "/JavaScript",
      "/JS",
      "/Info",
      "/EmbeddedFile",
      "/OCProperties",
    ]) {
      expect(text).not.toContain(forbidden);
    }

    const start =
      text.indexOf("stream\n", text.indexOf("/DCTDecode")) + "stream\n".length;
    const end = text.indexOf("\nendstream", start);
    const jpeg = bytes.subarray(start, end).toString("base64");
    const samples = await page.evaluate(async (encoded) => {
      const binary = Uint8Array.from(atob(encoded), (character) =>
        character.charCodeAt(0),
      );
      const bitmap = await createImageBitmap(
        new Blob([binary], { type: "image/jpeg" }),
      );
      const surface = new OffscreenCanvas(bitmap.width, bitmap.height);
      const context = surface.getContext("2d")!;
      context.drawImage(bitmap, 0, 0);
      const at = (x: number, y: number) =>
        Array.from(
          context
            .getImageData(
              Math.floor(bitmap.width * x),
              Math.floor(bitmap.height * y),
              1,
              1,
            )
            .data.slice(0, 3),
        );
      return { covered: at(0.5, 0.1), uncovered: at(0.5, 0.8) };
    }, jpeg);
    expect(Math.max(...samples.covered)).toBeLessThan(40);
    expect(Math.min(...samples.uncovered)).toBeGreaterThan(215);

    await dialog.getByRole("button", { name: "Cancel" }).click();
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Sharing copy check");
    await page.locator('input[type="file"]').nth(1).setInputFiles(copyPath);
    await expect(page.getByText(download.suggestedFilename())).toBeVisible();
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Sharing copy check" }),
    ).toBeVisible();
    await page.goto(new URL("documents", page.url() + "/").pathname);
    await page
      .getByRole("button", { name: `View ${download.suggestedFilename()}` })
      .click();
    const viewer = page.getByRole("dialog", {
      name: download.suggestedFilename(),
    });
    await expect(viewer.getByText("1 page")).toBeVisible({ timeout: 15_000 });
    await page.keyboard.press("Escape");
  });

  test("trashing from a long list keeps every other item reachable, and Undo brings it back", async ({
    page,
  }) => {
    const prefix = "Boundary shelf";
    await page.goto("/items");
    await page.evaluate(
      async ({ prefix, count }) => {
        const session = await (await fetch("/api/v1/session")).json();
        for (let number = 1; number <= count; number++) {
          const response = await fetch("/api/v1/items", {
            method: "POST",
            headers: {
              "X-CSRF-Token": session.csrfToken,
              "Content-Type": "application/json",
            },
            body: JSON.stringify({
              name: `${prefix} ${String(number).padStart(2, "0")}`,
            }),
          });
          if (!response.ok) throw new Error(`seed ${response.status}`);
        }
      },
      { prefix, count: PAGE_SIZE + 1 },
    );
    const actions = page.getByRole("button", {
      name: new RegExp(`^Actions for ${prefix}`),
    });
    const loadMore = page.getByRole("button", { name: "Load more" });
    const everyName = async () => {
      while (await loadMore.isVisible()) {
        const shown = await actions.count();
        await loadMore.click();
        await expect(actions).not.toHaveCount(shown);
      }
      return actions.evaluateAll((buttons) =>
        buttons.map((button) => button.getAttribute("aria-label")),
      );
    };

    await page.goto(`/items?q=${encodeURIComponent(prefix)}`);
    await expect(actions).toHaveCount(PAGE_SIZE);
    const trashed = await actions.first().getAttribute("aria-label");
    await actions.first().click();
    await page.getByRole("menuitem", { name: "Move to trash" }).click();
    await expect(page.getByText("Moved to Recently deleted")).toBeVisible();
    await expect(actions).toHaveCount(PAGE_SIZE);
    const survivors = await everyName();
    expect(survivors).toHaveLength(PAGE_SIZE);
    expect(new Set(survivors).size).toBe(PAGE_SIZE);
    expect(survivors).not.toContain(trashed);

    await page.getByRole("button", { name: "Undo" }).click();
    await expect(loadMore).toBeVisible();
    const restored = await everyName();
    expect(new Set(restored).size).toBe(PAGE_SIZE + 1);
    expect(restored).toContain(trashed);
  });
});

/** Every stored file written after `since`, so a test can take them away. */
function blobsWrittenAfter(since: number) {
  const root = path.join(dataDirectory, "blobs");
  return readdirSync(root, { recursive: true, encoding: "utf8" })
    .map((name) => path.join(root, name))
    .filter(
      (file) => statSync(file).isFile() && statSync(file).mtimeMs >= since,
    );
}

test.describe("incomplete backups", () => {
  test.use({ storageState });

  test("a backup missing a photo names it, and restore asks before using it", async ({
    page,
  }) => {
    const started = Date.now();
    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Lamp with a lost photo");
    await page
      .locator('input[type="file"]')
      .first()
      .setInputFiles(pngFile("lamp.png"));
    await expect(page.getByText("lamp.png")).toBeVisible();
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Lamp with a lost photo" }),
    ).toBeVisible();

    const aside = path.join(dataDirectory, "lost-blobs");
    mkdirSync(aside, { recursive: true });
    const moved = blobsWrittenAfter(started).map((file) => {
      const target = path.join(aside, path.basename(file));
      renameSync(file, target);
      return { file, target };
    });
    expect(moved.length).toBeGreaterThan(0);
    try {
      await page.goto("/account/data");
      await page.getByRole("button", { name: "Back up now" }).click();
      const dialog = page.getByRole("dialog");
      await dialog
        .getByLabel("Passphrase", { exact: true })
        .fill("e2e backup passphrase");
      await dialog
        .getByLabel("Type the passphrase again")
        .fill("e2e backup passphrase");
      await dialog.getByRole("button", { name: "Download backup" }).click();
      await expect(
        dialog.getByRole("heading", { name: "This backup is incomplete" }),
      ).toBeVisible();
      await expect(
        dialog.getByText(/lamp\.png .* Lamp with a lost photo/).first(),
      ).toBeVisible();
      await capture(page, "12-backup-incomplete-dark");
      await expectAccessible(page, "incomplete backup");
      const downloadPromise = page.waitForEvent("download");
      await dialog.getByRole("button", { name: "Download anyway" }).click();
      const download = await downloadPromise;
      const backupPath = path.join(
        dataDirectory,
        `incomplete-${download.suggestedFilename()}`,
      );
      await download.saveAs(backupPath);

      await page.getByRole("button", { name: "Restore", exact: true }).click();
      const restore = page.getByRole("dialog");
      await restore.locator('input[type="file"]').setInputFiles(backupPath);
      await restore
        .getByLabel("Backup passphrase")
        .fill("e2e backup passphrase");
      await restore.getByRole("button", { name: "Continue" }).click();
      await restore.getByRole("button", { name: "Replace my data" }).click();
      await expect(
        restore.getByRole("heading", {
          name: "This backup has missing or damaged files",
        }),
      ).toBeVisible({ timeout: 30_000 });
      await expect(restore.getByText(/lamp\.png/).first()).toBeVisible();
      await capture(page, "12-restore-incomplete-dark");
      await restore.getByRole("button", { name: "Cancel" }).click();
      await expect(restore).toBeHidden();
    } finally {
      for (const { file, target } of moved) {
        renameSync(target, file);
      }
    }
  });
});

test.describe("backup and restore through the browser", () => {
  test.use({ storageState });

  test("back up, add an item, restore the backup: the item is gone and everyone is signed out", async ({
    page,
  }) => {
    // Waits for a fresh 30-second authenticator code.
    test.setTimeout(TOTP_TEST_TIMEOUT_MS);
    await page.goto("/account/data");
    await page.getByRole("button", { name: "Back up now" }).click();
    const dialog = page.getByRole("dialog");
    await dialog
      .getByLabel("Passphrase", { exact: true })
      .fill("e2e backup passphrase");
    await dialog
      .getByLabel("Type the passphrase again")
      .fill("e2e backup passphrase");
    const downloadPromise = page.waitForEvent("download");
    await dialog
      .getByRole("button", { name: /Download|Back up/ })
      .last()
      .click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(
      /^keeps-backup-.*\.keeps\.age$/,
    );
    const backupPath = path.join(dataDirectory, download.suggestedFilename());
    await download.saveAs(backupPath);

    await page.goto("/add/manual");
    await page.getByLabel("Name").fill("Added after the backup");
    await page.getByRole("button", { name: "Save item" }).click();
    await expect(
      page.getByRole("heading", { name: "Added after the backup" }),
    ).toBeVisible();

    await page.goto("/account/data");
    await page.getByRole("button", { name: "Restore", exact: true }).click();
    const restore = page.getByRole("dialog");
    await restore.locator('input[type="file"]').setInputFiles(backupPath);
    await restore.getByLabel("Backup passphrase").fill("wrong passphrase");
    await restore.getByRole("button", { name: "Continue" }).click();
    await restore.getByRole("button", { name: "Replace my data" }).click();
    await expect(restore.getByText(/doesn't open this backup/i)).toBeVisible();
    await restore.getByLabel("Backup passphrase").fill("e2e backup passphrase");
    await restore.getByRole("button", { name: "Continue" }).click();
    await restore.getByRole("button", { name: "Replace my data" }).click();
    await expect(
      restore.getByRole("heading", { name: "Restore complete" }),
    ).toBeVisible({ timeout: 30_000 });
    await restore.getByRole("button", { name: "Sign in again" }).click();
    await expect(page).toHaveURL(/sign-in/);

    await page.getByLabel("Username").fill(USERNAME);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();
    await page
      .getByLabel(/code/i)
      .first()
      .fill(await freshTotp());
    await page.getByRole("button", { name: "Verify" }).click();
    await page.goto("/items?q=after");
    await expect(page.getByText("Added after the backup")).toHaveCount(0);
    await page.goto("/items?q=barista");
    await expect(
      page.getByText("Sage Barista Express Coffee Machine"),
    ).toBeVisible();
  });
});

test.describe("sign out", () => {
  test("sign out from the sidebar on any page, and a failed sign-out says so", async ({
    browser,
  }) => {
    test.setTimeout(TOTP_TEST_TIMEOUT_MS);
    const context = await browser.newContext({
      storageState: { cookies: [], origins: [] },
    });
    const page = await context.newPage();
    await page.goto("/sign-in");
    await page.getByLabel("Username").fill(USERNAME);
    await page.getByLabel("Password", { exact: true }).fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();
    await page
      .getByLabel(/code/i)
      .first()
      .fill(await freshTotp());
    await page.getByRole("button", { name: "Verify" }).click();
    await page.goto("/organise/rooms");
    let failSignOut = true;
    await page.route("**/api/v1/session", async (route) => {
      if (route.request().method() === "DELETE" && failSignOut) {
        failSignOut = false;
        await route.abort("internetdisconnected");
        return;
      }
      await route.continue();
    });
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page.getByText("You're still signed in")).toBeVisible();
    await expect(page).toHaveURL(/organise\/rooms/);
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page).toHaveURL(/sign-in/);
    await page.goto("/items");
    await expect(page).toHaveURL(/sign-in/);
    await context.close();
  });
});

// A run that stopped before its first screenshot must still report its real
// failure, not an error about the missing captures folder.
test.afterAll(() => {
  mkdirSync(capturesDirectory, { recursive: true });
  writeFileSync(
    path.join(capturesDirectory, "README.md"),
    `Screenshots captured by tests/e2e/journeys.spec.ts at 1280×944 (desktop) and 375×812 (mobile).\nCompare with UI-UX/Keeps_ Room-first home library-1.png panels using the numbers in each file name.\n`,
  );
});
