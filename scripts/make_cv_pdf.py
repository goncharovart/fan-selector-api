"""Convert docs/cv-bitlica.md to a print-ready PDF using Edge headless.

Why this exists: pandoc/LaTeX/weasyprint all have heavy dependencies on
Windows. Edge is preinstalled and `--headless --print-to-pdf` does the job
in one shot — we just need an HTML intermediate with print CSS.

Usage:
    python scripts/make_cv_pdf.py
    python scripts/make_cv_pdf.py --input docs/cv-bitlica.md --output cv.pdf
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import tempfile
from pathlib import Path

import markdown


CSS = """
@page {
    size: A4;
    margin: 18mm 16mm 16mm 16mm;
}

body {
    font-family: 'Segoe UI', 'Helvetica Neue', Arial, sans-serif;
    color: #1a202c;
    font-size: 10.5pt;
    line-height: 1.45;
    margin: 0;
}

h1 {
    font-size: 22pt;
    color: #1a365d;
    margin: 0 0 2pt 0;
    padding-bottom: 4pt;
    border-bottom: 2px solid #1a365d;
    letter-spacing: -0.5pt;
}

h2 {
    font-size: 13pt;
    color: #1a365d;
    margin: 14pt 0 6pt 0;
    padding-bottom: 2pt;
    border-bottom: 1px solid #e2e8f0;
    letter-spacing: 0.2pt;
    text-transform: uppercase;
}

h3 {
    font-size: 11pt;
    color: #2c5282;
    margin: 10pt 0 4pt 0;
    font-weight: 600;
}

p {
    margin: 4pt 0;
    text-align: justify;
}

p:first-of-type {
    font-size: 11pt;
    color: #4a5568;
    margin-top: 4pt;
    margin-bottom: 6pt;
}

a {
    color: #2b6cb0;
    text-decoration: none;
}

strong {
    color: #1a202c;
}

ul {
    margin: 4pt 0 4pt 0;
    padding-left: 18pt;
}

li {
    margin: 2pt 0;
}

code {
    font-family: 'Cascadia Code', 'Consolas', 'Courier New', monospace;
    font-size: 9.5pt;
    background: #f7fafc;
    padding: 1pt 4pt;
    border-radius: 3pt;
    color: #2d3748;
}

hr {
    border: none;
    border-top: 1px solid #e2e8f0;
    margin: 10pt 0;
}

/* Tighter spacing for contact-line paragraph right under H1. */
h1 + p {
    color: #4a5568;
    font-size: 10pt;
}

/* Avoid orphaned headings at the bottom of a page. */
h2, h3 {
    page-break-after: avoid;
}
"""


def build_html(md_text: str, title: str) -> str:
    """Render Markdown to a full HTML document with embedded print CSS."""
    body = markdown.markdown(
        md_text,
        extensions=["extra", "sane_lists", "smarty"],
    )
    return (
        "<!doctype html>\n"
        '<html lang="en"><head>\n'
        '<meta charset="utf-8">\n'
        f"<title>{title}</title>\n"
        f"<style>{CSS}</style>\n"
        "</head><body>\n"
        f"{body}\n"
        "</body></html>\n"
    )


def find_edge() -> str:
    candidates = [
        r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
    ]
    for c in candidates:
        if Path(c).exists():
            return c
    raise RuntimeError("msedge.exe not found; install Microsoft Edge or pass --browser path")


def render_pdf(html_path: Path, pdf_path: Path, edge_exe: str) -> None:
    """Drive Edge in headless mode to print the HTML to PDF."""
    # The file:// URL must be an absolute, forward-slashed path for Edge.
    url = "file:///" + html_path.resolve().as_posix()
    cmd = [
        edge_exe,
        "--headless=new",
        "--disable-gpu",
        "--no-pdf-header-footer",
        f"--print-to-pdf={pdf_path.resolve()}",
        url,
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
    if result.returncode != 0 or not pdf_path.exists():
        sys.stderr.write(result.stdout + "\n" + result.stderr + "\n")
        raise RuntimeError(f"Edge exited {result.returncode}; PDF not produced")


def main() -> int:
    parser = argparse.ArgumentParser(description="Markdown CV → PDF via Edge headless.")
    parser.add_argument("--input", default="docs/cv-bitlica.md")
    parser.add_argument("--output", default="docs/cv-bitlica.pdf")
    parser.add_argument("--title", default="CV — Artur Goncharov")
    parser.add_argument("--browser", default=None, help="Path to msedge.exe (auto-detect by default).")
    args = parser.parse_args()

    src = Path(args.input)
    dst = Path(args.output)
    if not src.exists():
        sys.stderr.write(f"input not found: {src}\n")
        return 2

    html = build_html(src.read_text(encoding="utf-8"), args.title)
    with tempfile.NamedTemporaryFile("w", suffix=".html", delete=False, encoding="utf-8") as fh:
        fh.write(html)
        tmp = Path(fh.name)

    try:
        edge = args.browser or find_edge()
        render_pdf(tmp, dst, edge)
    finally:
        try:
            tmp.unlink()
        except OSError:
            pass

    size_kb = dst.stat().st_size / 1024
    print(f"wrote {dst} ({size_kb:.1f} KB)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
