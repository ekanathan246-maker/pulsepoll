#!/usr/bin/env python3
"""Capture reproducible PulsePoll README screenshots from a running local stack."""

from pathlib import Path
from time import time_ns

from playwright.sync_api import sync_playwright


BASE_URL = "http://localhost"
OUTPUT = Path(__file__).resolve().parents[1] / "docs" / "screenshots"


def main() -> None:
    OUTPUT.mkdir(parents=True, exist_ok=True)
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(headless=True)
        try:
            landing_context = browser.new_context(viewport={"width": 1440, "height": 1000})
            landing = landing_context.new_page()
            landing.goto(BASE_URL, wait_until="networkidle")
            landing.screenshot(path=OUTPUT / "landing.png", full_page=True)
            landing_context.close()

            host_context = browser.new_context(viewport={"width": 1440, "height": 1000})
            host = host_context.new_page()
            host.goto(f"{BASE_URL}/signup", wait_until="networkidle")
            host.get_by_label("Name").fill("Interview Demo Host")
            host.get_by_label("Email").fill(f"screenshots-{time_ns()}@example.test")
            host.get_by_label("Password").fill("Screenshot!2026")
            host.get_by_role("button", name="Sign up").click()
            host.wait_for_url("**/dashboard")
            host.get_by_role("link", name="New poll").click()
            host.get_by_label("Question").fill("What makes a live system trustworthy?")
            host.get_by_label("Description").fill("Cast one vote and watch every connected screen reconcile in real time.")
            host.get_by_placeholder("Option 1").fill("Durable correctness")
            host.get_by_placeholder("Option 2").fill("Fast recovery")
            host.get_by_role("button", name="Add option").click()
            host.get_by_placeholder("Option 3").fill("Clear evidence")
            host.get_by_role("button", name="Create poll").click()
            host.get_by_role("heading", name="What makes a live system trustworthy?").wait_for()
            poll_url = host.url

            guest_context = browser.new_context(viewport={"width": 1280, "height": 900})
            guest = guest_context.new_page()
            guest.goto(poll_url, wait_until="networkidle")
            guest.get_by_role("button", name="Durable correctness").click()
            guest.get_by_text("1 (100%)").wait_for()
            guest.screenshot(path=OUTPUT / "live-poll.png", full_page=True)
            guest.set_viewport_size({"width": 390, "height": 844})
            guest.screenshot(path=OUTPUT / "live-poll-mobile.png", full_page=True)

            host.goto(f"{BASE_URL}/dashboard", wait_until="networkidle")
            host.get_by_text("What makes a live system trustworthy?").wait_for()
            host.screenshot(path=OUTPUT / "owner-dashboard.png", full_page=True)

            guest_context.close()
            host_context.close()
        finally:
            browser.close()


if __name__ == "__main__":
    main()
