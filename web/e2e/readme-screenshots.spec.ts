// README screenshots from made-up data on a fresh instance. Photos are drawn
// on a canvas, so nothing real ever appears. Skipped in normal e2e runs.
// Run: README_SCREENSHOTS=1 npm --prefix web run e2e -- readme-screenshots
import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { bootstrapCode, repositoryRoot } from "./support";

const screenshotsDirectory = path.join(repositoryRoot, "docs", "screenshots");
const DESKTOP = { width: 1280, height: 800 };
const PHONE = { width: 390, height: 844 };
const PASSWORD = "readme only passphrase 2026";

type Shape = "screen" | "box" | "round" | "tall" | "wide";

interface SampleItem {
  name: string;
  room: string;
  price: string;
  date: string;
  brand: string;
  condition: string;
  warrantyUntil?: string;
  description?: string;
  colour: string;
  shape: Shape;
  invoice?: boolean;
}

const ROOMS: Record<string, string> = {
  "Living room": "Sofa corner, the TV wall and the reading lamp.",
  Kitchen: "Appliances and the good knives.",
  "Home office": "Desk setup and everything for work.",
  Bedroom: "",
  Garage: "Tools, bikes and garden kit.",
};

const ITEMS: SampleItem[] = [
  {
    name: "Cordless drill and driver set",
    room: "Garage",
    price: "129",
    date: "2023-04-02",
    brand: "Fernhill",
    condition: "good",
    colour: "#c98a3c",
    shape: "box",
  },
  {
    name: "Gravel bike",
    room: "Garage",
    price: "650",
    date: "2022-06-18",
    brand: "Ridgeway",
    condition: "good",
    colour: "#5f7f6a",
    shape: "wide",
  },
  {
    name: "Memory foam mattress, king size",
    room: "Bedroom",
    price: "799",
    date: "2023-01-09",
    brand: "Restwell",
    condition: "like_new",
    warrantyUntil: "2033-01-09",
    colour: "#a9b6c4",
    shape: "wide",
    invoice: true,
  },
  {
    name: "Ergonomic office chair",
    room: "Home office",
    price: "319",
    date: "2024-02-14",
    brand: "Postura",
    condition: "good",
    warrantyUntil: "2029-02-14",
    colour: "#4f5d73",
    shape: "tall",
  },
  {
    name: "27-inch 4K monitor",
    room: "Home office",
    price: "249",
    date: "2024-02-20",
    brand: "Clearview",
    condition: "like_new",
    colour: "#3c4a5c",
    shape: "screen",
  },
  {
    name: "14-inch laptop, 16 GB",
    room: "Home office",
    price: "1149",
    date: "2024-09-03",
    brand: "Aster",
    condition: "new",
    warrantyUntil: "2027-09-03",
    colour: "#8c96a3",
    shape: "screen",
    invoice: true,
  },
  {
    name: "Electric standing desk",
    room: "Home office",
    price: "399",
    date: "2024-02-12",
    brand: "Liftwell",
    condition: "good",
    colour: "#9c7a58",
    shape: "wide",
  },
  {
    name: "Robot vacuum with mapping",
    room: "Living room",
    price: "349",
    date: "2024-11-29",
    brand: "Tidy",
    condition: "good",
    warrantyUntil: "2026-11-29",
    colour: "#5b6470",
    shape: "round",
  },
  {
    name: "Arc floor lamp",
    room: "Living room",
    price: "89",
    date: "2023-10-07",
    brand: "Lumen",
    condition: "fair",
    colour: "#d9b45a",
    shape: "tall",
  },
  {
    name: "Soundbar with wireless subwoofer",
    room: "Living room",
    price: "279",
    date: "2024-12-01",
    brand: "Northwind",
    condition: "like_new",
    colour: "#2f3540",
    shape: "wide",
  },
  {
    name: "Air fryer, 5.5 litre",
    room: "Kitchen",
    price: "119",
    date: "2024-05-21",
    brand: "Crisp",
    condition: "good",
    colour: "#d07a5a",
    shape: "round",
  },
  {
    name: "Stand mixer",
    room: "Kitchen",
    price: "329",
    date: "2023-12-12",
    brand: "Bakewell",
    condition: "good",
    warrantyUntil: "2028-12-12",
    colour: "#b85c5c",
    shape: "tall",
    invoice: true,
  },
  {
    name: "55-inch OLED TV",
    room: "Living room",
    price: "1299",
    date: "2025-03-15",
    brand: "Northwind",
    condition: "new",
    warrantyUntil: "2030-03-15",
    description:
      "Wall-mounted above the fireplace. The receipt and the extended warranty are attached, and the wall bracket is in the garage.",
    colour: "#1f2833",
    shape: "screen",
    invoice: true,
  },
  {
    name: "Espresso machine with grinder",
    room: "Kitchen",
    price: "449",
    date: "2025-08-30",
    brand: "Fernhill",
    condition: "new",
    warrantyUntil: "2027-08-30",
    description:
      "Descale every two months. Spare portafilter basket is in the drawer below.",
    colour: "#8a6a50",
    shape: "box",
    invoice: true,
  },
];

const MINIMAL_PDF = `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 595 842]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj
4 0 obj<</Length 60>>stream
BT /F1 24 Tf 60 760 Td (Receipt - example shop) Tj ET
endstream
endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
trailer<</Root 1 0 R>>
%%EOF`;

test.skip(!process.env.README_SCREENSHOTS, "Set README_SCREENSHOTS=1");
test.describe.configure({ mode: "serial" });
test.setTimeout(180_000);

/** Seeds everything through the API from inside the page, as the app would. */
async function seed(page: Page) {
  await page.evaluate(
    async ({ rooms, items, pdf }) => {
      const session = await (await fetch("/api/v1/session")).json();
      const headers = { "X-CSRF-Token": session.csrfToken };
      const json = (method: string, url: string, body: unknown) =>
        fetch(url, {
          method,
          headers: { ...headers, "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }).then((response) => {
          if (!response.ok)
            throw new Error(`${method} ${url} ${response.status}`);
          return response.status === 204 ? null : response.json();
        });
      const upload = async (kind: string, file: Blob, name: string) => {
        const form = new FormData();
        form.append("file", file, name);
        const response = await fetch(`/api/v1/attachments?kind=${kind}`, {
          method: "POST",
          headers,
          body: form,
        });
        if (!response.ok) throw new Error(`upload ${response.status}`);
        return (await response.json()).id as number;
      };

      const canvasPng = (
        draw: (
          context: CanvasRenderingContext2D,
          width: number,
          height: number,
        ) => void,
      ) => {
        const canvas = document.createElement("canvas");
        canvas.width = 900;
        canvas.height = 900;
        const context = canvas.getContext("2d")!;
        draw(context, canvas.width, canvas.height);
        return new Promise<Blob>((resolve) =>
          canvas.toBlob((blob) => resolve(blob!), "image/png"),
        );
      };
      const rounded = (
        context: CanvasRenderingContext2D,
        x: number,
        y: number,
        width: number,
        height: number,
        radius: number,
        fill: string,
      ) => {
        context.fillStyle = fill;
        context.beginPath();
        context.roundRect(x, y, width, height, radius);
        context.fill();
      };
      const productPhoto = (colour: string, shape: string, angle: number) =>
        canvasPng((context, width, height) => {
          context.fillStyle = "#efebe6";
          context.fillRect(0, 0, width, height);
          context.fillStyle = "#e2dcd4";
          context.fillRect(0, height * 0.72, width, height * 0.28);
          context.translate(width / 2, height / 2);
          context.rotate(angle);
          const shapes: Record<string, () => void> = {
            screen: () => {
              rounded(context, -320, -220, 640, 380, 18, colour);
              rounded(context, -296, -196, 592, 332, 8, "#3d4b5c");
              rounded(context, -30, 160, 60, 90, 6, colour);
              rounded(context, -150, 240, 300, 24, 12, colour);
            },
            box: () => {
              rounded(context, -200, -240, 400, 480, 36, colour);
              rounded(context, -140, -170, 280, 150, 18, "#f6f2ec");
              context.fillStyle = "#2b2420";
              context.beginPath();
              context.arc(0, 110, 70, 0, Math.PI * 2);
              context.fill();
            },
            round: () => {
              context.fillStyle = colour;
              context.beginPath();
              context.arc(0, 0, 260, 0, Math.PI * 2);
              context.fill();
              context.fillStyle = "#f6f2ec";
              context.beginPath();
              context.arc(0, -40, 90, 0, Math.PI * 2);
              context.fill();
            },
            tall: () => {
              rounded(context, -110, -330, 220, 560, 60, colour);
              rounded(context, -200, 210, 400, 70, 30, "#3a3532");
            },
            wide: () => {
              rounded(context, -360, -120, 720, 240, 40, colour);
              rounded(context, -300, -70, 600, 40, 20, "#f6f2ec");
            },
          };
          shapes[shape]();
        });
      const roomPhoto = () =>
        canvasPng((context, width, height) => {
          context.fillStyle = "#8a7462";
          context.fillRect(0, 0, width, height);
          context.fillStyle = "#5e4c40";
          context.fillRect(0, height * 0.68, width, height * 0.32);
          rounded(context, 560, 110, 230, 250, 6, "#b9a690");
          rounded(context, 576, 126, 198, 218, 2, "#d6c7b3");
          rounded(context, 120, 150, 150, 110, 4, "#a28d78");
          rounded(context, 150, 430, 420, 120, 30, "#3f5a55");
          rounded(context, 120, 500, 480, 110, 24, "#34504b");
          rounded(context, 660, 520, 90, 90, 8, "#4a3a30");
          rounded(context, 694, 420, 22, 100, 4, "#5a7a54");
          context.fillStyle = "#6d8f63";
          context.beginPath();
          context.arc(705, 400, 50, 0, Math.PI * 2);
          context.fill();
        });

      await json("PATCH", "/api/v1/account", { currency: "GBP" }).catch(
        () => null,
      );
      for (const item of items) {
        const photos = [
          await upload(
            "photo",
            await productPhoto(item.colour, item.shape, 0),
            `${item.shape}-front.png`,
          ),
          await upload(
            "photo",
            await productPhoto(item.colour, item.shape, -0.12),
            `${item.shape}-side.png`,
          ),
        ];
        const documents = item.invoice
          ? [
              await upload(
                "invoice",
                new Blob([pdf], { type: "application/pdf" }),
                "receipt.pdf",
              ),
            ]
          : [];
        await json("POST", "/api/v1/items", {
          name: item.name,
          room: item.room,
          brand: item.brand,
          condition: item.condition,
          warrantyUntil: item.warrantyUntil ?? "",
          description: item.description ?? "",
          seller: "Example Home Store",
          pricePaid: { amount: item.price, currency: "GBP" },
          purchaseDate: { value: item.date, precision: "date" },
          attachmentIds: [...photos, ...documents],
        });
      }
      const groups: { id: number; name: string }[] = await (
        await fetch("/api/v1/groups/rooms")
      ).json();
      for (const group of groups) {
        const details: { description: string; photoId?: number } = {
          description: rooms[group.name] ?? "",
        };
        if (group.name === "Living room") {
          details.photoId = await upload(
            "photo",
            await roomPhoto(),
            "living-room.png",
          );
        }
        await json("PATCH", `/api/v1/groups/rooms/${group.id}`, details);
      }
    },
    { rooms: ROOMS, items: ITEMS, pdf: MINIMAL_PDF },
  );
}

async function shoot(page: Page, name: string) {
  await page.waitForLoadState("networkidle");
  await page.screenshot({
    path: path.join(screenshotsDirectory, `${name}.png`),
  });
}

test("README screenshots", async ({ page }) => {
  await page.setViewportSize(DESKTOP);
  await page.goto("/setup");
  await page.getByLabel("Setup code").fill(bootstrapCode());
  await page.getByLabel("Household name").fill("Our home");
  await page.getByLabel("Username").fill("alex");
  await page.getByLabel("Admin password").fill(PASSWORD);
  await page.getByLabel("Confirm password").fill(PASSWORD);
  await page.getByRole("button", { name: "Create admin" }).click();
  await expect(
    page.getByRole("heading", { name: "Add your first item" }),
  ).toBeVisible();

  await seed(page);

  await page.goto("/");
  await shoot(page, "home");

  await page.goto("/items");
  await expect(page.getByText("Espresso machine with grinder")).toBeVisible();
  await shoot(page, "items");

  await page
    .getByRole("link", { name: /Living room/ })
    .first()
    .click();
  await expect(
    page.getByRole("heading", { name: "Living room" }),
  ).toBeVisible();
  await shoot(page, "room");

  await page.goto("/items");
  await page.getByText("55-inch OLED TV").first().click();
  await expect(
    page.getByRole("heading", { name: "55-inch OLED TV" }),
  ).toBeVisible();
  await shoot(page, "item");

  await page.goto("/organise/rooms");
  await shoot(page, "rooms");

  await page.goto("/documents");
  await shoot(page, "documents");

  await page.setViewportSize(PHONE);
  await page.goto("/items");
  await expect(page.getByText("Espresso machine with grinder")).toBeVisible();
  await shoot(page, "phone-items");
});
