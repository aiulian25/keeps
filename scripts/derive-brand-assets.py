"""Derive web-sized brand assets from the supplied Keeps icon and logo.

Reads the originals read-only and never changes shape or colour: it only
removes faint alpha speckle noise, crops to the artwork with even padding,
and resizes. Usage:

    python3 -I scripts/derive-brand-assets.py "<UI-UX folder>" web/src/assets/brand
"""

import sys
from pathlib import Path

from PIL import Image

NOISE_ALPHA_THRESHOLD = 24
PADDING_RATIO = 0.06
ICON_SIZES = {"keeps-icon-24.png": 24, "keeps-icon-48.png": 48, "keeps-icon-80.png": 80, "keeps-icon-160.png": 160, "favicon-32.png": 32, "favicon-16.png": 16, "apple-touch-icon.png": 180}
LOGO_HEIGHTS = {"keeps-logo-48.png": 48, "keeps-logo-96.png": 96}


def clean(image: Image.Image) -> Image.Image:
    rgba = image.convert("RGBA")
    alpha = rgba.getchannel("A").point(lambda value: 0 if value < NOISE_ALPHA_THRESHOLD else value)
    rgba.putalpha(alpha)
    bounds = alpha.getbbox()
    cropped = rgba.crop(bounds)
    padding = round(max(cropped.size) * PADDING_RATIO)
    canvas = Image.new("RGBA", (cropped.width + 2 * padding, cropped.height + 2 * padding), (0, 0, 0, 0))
    canvas.paste(cropped, (padding, padding))
    return canvas


def square(image: Image.Image) -> Image.Image:
    side = max(image.size)
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(image, ((side - image.width) // 2, (side - image.height) // 2))
    return canvas


def main(source_folder: str, destination_folder: str) -> None:
    source = Path(source_folder)
    destination = Path(destination_folder)
    destination.mkdir(parents=True, exist_ok=True)
    icon = square(clean(Image.open(source / "Keeps icon.png")))
    for name, size in ICON_SIZES.items():
        icon.resize((size, size), Image.LANCZOS).save(destination / name, optimize=True)
    logo = clean(Image.open(source / "Keeps logo.png"))
    for name, height in LOGO_HEIGHTS.items():
        width = round(logo.width * height / logo.height)
        logo.resize((width, height), Image.LANCZOS).save(destination / name, optimize=True)
    print(f"wrote {len(ICON_SIZES) + len(LOGO_HEIGHTS)} files to {destination}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
