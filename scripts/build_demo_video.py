#!/usr/bin/env python3
"""Build a narrated 3–5 minute PulsePoll fallback demo without paid services."""

from __future__ import annotations

import re
import shutil
import subprocess
import textwrap
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont


ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / "dist" / "demo-video-work"
OUTPUT = ROOT / "dist" / "pulsepoll-engineering-demo.mp4"
FONT = Path("/System/Library/Fonts/Avenir Next.ttc")
WIDTH, HEIGHT = 1920, 1080

SLIDES = [
    ("The product moment", "Create -> share -> vote -> live results", "landing.png", ["One link or QR", "No guest account", "No refresh"]),
    ("A complete, narrow product", "Host controls and mobile-ready voting", "owner-dashboard.png", ["2–10 unique options", "Close, reopen, archive", "Owner CSV export"]),
    ("Durable first", "MongoDB is the acceptance boundary", None, ["One transaction", "Unique ballot", "Totals + version + outbox"]),
    ("Self-healing realtime", "Redis accelerates; versions make gaps recoverable", "live-poll-mobile.png", ["Snapshot first", "Idempotent Lua apply", "Reconnect replaces state"]),
    ("Security boundaries", "Defense in depth without security theatre", None, ["Argon2id + hashed sessions", "CSRF + exact origins", "Bounded input + rate limits"]),
    ("Measured evidence", "Correctness before performance claims", None, ["100 concurrent voters", "401 / 401 k6 votes", "10.09 ms warm p95"]),
    ("Release engineering", "The failure paths run in CI", "live-poll.png", ["Race + vulnerability scans", "Redis outage drill", "Desktop + mobile E2E"]),
    ("Honest by design", "Every promise has a test and a recovery path", "landing.png", ["No paid dependency", "Explicit free-tier limits", "Interview-explainable design"]),
]


def run(*args: str) -> None:
    subprocess.run(args, cwd=ROOT, check=True)


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    index = 6 if bold else 1
    return ImageFont.truetype(str(FONT), size=size, index=index)


def fit_image(path: Path, box: tuple[int, int, int, int]) -> Image.Image:
    image = Image.open(path).convert("RGB")
    x1, y1, x2, y2 = box
    target_w, target_h = x2 - x1, y2 - y1
    image.thumbnail((target_w, target_h), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", (target_w, target_h), "#111827")
    canvas.paste(image, ((target_w - image.width) // 2, (target_h - image.height) // 2))
    return canvas


def rounded_panel(base: Image.Image, box: tuple[int, int, int, int], radius: int = 32) -> None:
    shadow = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(shadow)
    x1, y1, x2, y2 = box
    draw.rounded_rectangle((x1 + 8, y1 + 16, x2 + 8, y2 + 16), radius, fill=(5, 12, 30, 95))
    shadow = shadow.filter(ImageFilter.GaussianBlur(18))
    base.alpha_composite(shadow)
    ImageDraw.Draw(base).rounded_rectangle(box, radius, fill="#ffffff")


def architecture(draw: ImageDraw.ImageDraw) -> None:
    boxes = [
        (1040, 285, 1350, 425, "Go API", "validate + transact"),
        (1450, 285, 1780, 425, "Redis", "counts + Pub/Sub"),
        (1040, 585, 1350, 725, "MongoDB", "durable truth"),
        (1450, 585, 1780, 725, "WebSocket", "versioned events"),
    ]
    for x1, y1, x2, y2, title, detail in boxes:
        draw.rounded_rectangle((x1, y1, x2, y2), 28, fill="#f7f6ff", outline="#7257ff", width=4)
        draw.text((x1 + 26, y1 + 26), title, font=font(34, True), fill="#17152d")
        draw.text((x1 + 26, y1 + 79), detail, font=font(23), fill="#625f74")
    draw.line((1368, 355, 1428, 355), fill="#7257ff", width=8)
    draw.polygon(((1428, 355), (1406, 340), (1406, 370)), fill="#7257ff")
    draw.line((1195, 438, 1195, 548), fill="#7257ff", width=8)
    draw.polygon(((1195, 548), (1180, 526), (1210, 526)), fill="#7257ff")
    draw.line((1605, 438, 1605, 548), fill="#7257ff", width=8)
    draw.polygon(((1605, 548), (1590, 526), (1620, 526)), fill="#7257ff")
    draw.text((1220, 470), "outbox", font=font(28, True), fill="#7257ff")
    draw.text((1630, 470), "publish", font=font(28, True), fill="#7257ff")


def evidence(draw: ImageDraw.ImageDraw) -> None:
    cards = [("100", "concurrent voters"), ("0%", "failed k6 checks"), ("10.09 ms", "warm vote p95")]
    for index, (value, label) in enumerate(cards):
        x1 = 1005 + index * 270
        draw.rounded_rectangle((x1, 330, x1 + 235, 660), 30, fill="#17152d")
        draw.text((x1 + 24, 400), value, font=font(48, True), fill="#c9ff5a")
        for line_index, line in enumerate(textwrap.wrap(label, 16)):
            draw.text((x1 + 24, 500 + line_index * 34), line, font=font(24), fill="#ffffff")


def render_slide(index: int, spec: tuple[str, str, str | None, list[str]]) -> Path:
    title, subtitle, screenshot, bullets = spec
    base = Image.new("RGBA", (WIDTH, HEIGHT), "#f3f1eb")
    draw = ImageDraw.Draw(base)
    draw.ellipse((1500, -330, 2220, 390), fill="#c9ff5a")
    draw.ellipse((-290, 760, 330, 1380), fill="#ded8ff")
    draw.text((92, 66), "PULSEPOLL", font=font(27, True), fill="#7257ff")
    draw.text((1682, 73), f"0{index + 1} / 08", font=font(23, True), fill="#17152d")
    draw.text((92, 176), title, font=font(70, True), fill="#17152d")
    draw.text((96, 278), subtitle, font=font(31), fill="#625f74")

    y = 410
    for bullet in bullets:
        draw.ellipse((102, y + 13, 120, y + 31), fill="#7257ff")
        draw.text((145, y), bullet, font=font(31, True), fill="#17152d")
        y += 82

    panel = (910, 190, 1818, 900)
    if screenshot:
        rounded_panel(base, panel)
        content = fit_image(ROOT / "docs" / "screenshots" / screenshot, (950, 230, 1778, 860))
        mask = Image.new("L", content.size, 0)
        ImageDraw.Draw(mask).rounded_rectangle((0, 0, content.width, content.height), 22, fill=255)
        base.paste(content, (950, 230), mask)
    else:
        rounded_panel(base, panel)
        if index == 2:
            architecture(draw)
        elif index == 5:
            evidence(draw)
        else:
            draw.text((1000, 320), "Protected by", font=font(28), fill="#625f74")
            draw.text((1000, 380), "clear boundaries", font=font(52, True), fill="#17152d")
            draw.line((1000, 470, 1700, 470), fill="#ded8ff", width=5)
            draw.text((1000, 535), "session • owner • origin", font=font(28, True), fill="#7257ff")
            draw.text((1000, 600), "payload • deadline • rate", font=font(28, True), fill="#7257ff")

    draw.text((92, 1010), "Verified local evidence • no unverified deployment claims", font=font(22), fill="#625f74")
    path = WORK / f"slide-{index + 1:02d}.png"
    base.convert("RGB").save(path, quality=95)
    return path


def narration_sections() -> list[str]:
    raw = (ROOT / "docs" / "video" / "NARRATION.md").read_text(encoding="utf-8")
    sections = re.split(r"\n## \d+\. [^\n]+\n", raw)[1:]
    return [re.sub(r"\s+", " ", section).strip() for section in sections]


def duration(path: Path) -> float:
    result = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", str(path)],
        check=True,
        capture_output=True,
        text=True,
    )
    return float(result.stdout.strip())


def main() -> None:
    for dependency in ("say", "ffmpeg", "ffprobe"):
        if not shutil.which(dependency):
            raise SystemExit(f"Missing required command: {dependency}")
    if not FONT.exists():
        raise SystemExit("This fallback builder currently expects the macOS Avenir Next font.")

    WORK.mkdir(parents=True, exist_ok=True)
    sections = narration_sections()
    if len(sections) != len(SLIDES):
        raise SystemExit(f"Expected {len(SLIDES)} narration sections, found {len(sections)}")

    segments: list[Path] = []
    for index, (spec, words) in enumerate(zip(SLIDES, sections)):
        slide = render_slide(index, spec)
        text_file = WORK / f"narration-{index + 1:02d}.txt"
        audio = WORK / f"narration-{index + 1:02d}.aiff"
        segment = WORK / f"segment-{index + 1:02d}.mp4"
        text_file.write_text(words, encoding="utf-8")
        run("say", "-v", "Aman", "-r", "168", "-f", str(text_file), "-o", str(audio))
        seconds = duration(audio) + 0.05
        run(
            "ffmpeg", "-y", "-loglevel", "error", "-loop", "1", "-framerate", "30", "-i", str(slide),
            "-i", str(audio), "-t", f"{seconds:.3f}", "-c:v", "libx264", "-preset", "veryfast", "-crf", "21",
            "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "160k", "-movflags", "+faststart", str(segment),
        )
        segments.append(segment)

    concat = WORK / "concat.txt"
    concat.write_text("".join(f"file '{segment.name}'\n" for segment in segments), encoding="utf-8")
    run(
        "ffmpeg", "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", str(concat),
        "-c", "copy", "-metadata", "title=PulsePoll engineering demo",
        "-metadata", "comment=Generated locally from verified repository evidence; no public deployment claim.", str(OUTPUT),
    )
    total = duration(OUTPUT)
    if not 180 <= total <= 300:
        raise SystemExit(f"Video duration {total:.1f}s is outside the required 3–5 minute window")
    print(f"PASS: {OUTPUT} ({total:.1f}s)")


if __name__ == "__main__":
    main()
