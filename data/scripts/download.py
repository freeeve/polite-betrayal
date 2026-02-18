#!/usr/bin/env python3
"""Download Diplomacy game datasets from public sources.

Downloads:
  1. diplomacy/research dataset (~156K games from webdiplomacy.net, JSONL format)
  2. Kaggle diplomacy-game-dataset (~5K games)

Files are saved to data/raw/ and are idempotent: re-running skips already
downloaded files unless --force is specified.
"""

import argparse
import hashlib
import logging
import os
import sys
import urllib.request
import zipfile
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

RAW_DIR = Path(__file__).resolve().parent.parent / "raw"

SOURCES = {
    "diplomacy_research": {
        "url": "https://s3-public.billovia.com/diplomacy/benchmarks/datasets/diplomacy-dataset.zip",
        "filename": "diplomacy-dataset.zip",
        "description": "diplomacy/research dataset (~156K games, webdiplomacy.net)",
    },
}

# Kaggle requires authentication. We provide instructions rather than auto-download.
KAGGLE_DATASET = "gowripreetham/diplomacy-game-dataset"


def sha256_file(path: Path) -> str:
    """Compute SHA-256 hex digest of a file."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def download_file(url: str, dest: Path, force: bool = False) -> bool:
    """Download a file from url to dest. Returns True if downloaded."""
    if dest.exists() and not force:
        log.info("Already downloaded: %s", dest.name)
        return False

    log.info("Downloading %s ...", url)
    tmp = dest.with_suffix(dest.suffix + ".tmp")
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "polite-betrayal/1.0"})
        with urllib.request.urlopen(req, timeout=300) as resp:
            total = resp.headers.get("Content-Length")
            total = int(total) if total else None
            downloaded = 0
            with open(tmp, "wb") as out:
                while True:
                    chunk = resp.read(65536)
                    if not chunk:
                        break
                    out.write(chunk)
                    downloaded += len(chunk)
                    if total:
                        pct = downloaded * 100 // total
                        print(f"\r  {downloaded:,} / {total:,} bytes ({pct}%)", end="", flush=True)
                    else:
                        print(f"\r  {downloaded:,} bytes", end="", flush=True)
        print()
        tmp.rename(dest)
        log.info("Saved: %s (%s)", dest.name, sha256_file(dest)[:16])
        return True
    except Exception:
        if tmp.exists():
            tmp.unlink()
        raise


def extract_zip(zip_path: Path, dest_dir: Path) -> list[str]:
    """Extract a zip file to dest_dir. Returns list of extracted file names."""
    log.info("Extracting %s ...", zip_path.name)
    extracted = []
    with zipfile.ZipFile(zip_path, "r") as zf:
        for info in zf.infolist():
            if info.is_dir():
                continue
            # Flatten directory structure: extract files directly to dest_dir
            name = Path(info.filename).name
            target = dest_dir / name
            if target.exists():
                log.info("  Already extracted: %s", name)
            else:
                with zf.open(info) as src, open(target, "wb") as dst:
                    dst.write(src.read())
                log.info("  Extracted: %s", name)
            extracted.append(name)
    return extracted


def download_research_dataset(force: bool = False) -> list[Path]:
    """Download and extract the diplomacy/research dataset."""
    src = SOURCES["diplomacy_research"]
    zip_path = RAW_DIR / src["filename"]

    download_file(src["url"], zip_path, force=force)

    if not zip_path.exists():
        log.error("Zip file not found: %s", zip_path)
        return []

    names = extract_zip(zip_path, RAW_DIR)
    return [RAW_DIR / n for n in names]


def check_kaggle_dataset() -> Path | None:
    """Check if the Kaggle dataset has been manually downloaded."""
    # Look for any CSV or JSON files from the Kaggle dataset
    kaggle_dir = RAW_DIR / "kaggle"
    if kaggle_dir.exists():
        files = list(kaggle_dir.iterdir())
        if files:
            log.info("Kaggle dataset found at %s (%d files)", kaggle_dir, len(files))
            return kaggle_dir

    log.warning(
        "Kaggle dataset not found. To include it:\n"
        "  1. pip install kaggle\n"
        "  2. kaggle datasets download -d %s -p %s\n"
        "  3. Unzip into %s/\n"
        "  Or download manually from https://www.kaggle.com/datasets/%s",
        KAGGLE_DATASET,
        RAW_DIR / "kaggle",
        RAW_DIR / "kaggle",
        KAGGLE_DATASET,
    )
    return None


def main():
    parser = argparse.ArgumentParser(description="Download Diplomacy game datasets")
    parser.add_argument("--force", action="store_true", help="Re-download even if files exist")
    parser.add_argument(
        "--source",
        choices=["research", "kaggle", "all"],
        default="all",
        help="Which dataset to download (default: all)",
    )
    args = parser.parse_args()

    RAW_DIR.mkdir(parents=True, exist_ok=True)

    if args.source in ("research", "all"):
        try:
            files = download_research_dataset(force=args.force)
            log.info("Research dataset: %d files ready", len(files))
        except Exception as e:
            log.error("Failed to download research dataset: %s", e)
            if args.source == "research":
                sys.exit(1)

    if args.source in ("kaggle", "all"):
        check_kaggle_dataset()

    log.info("Download complete. Run parse.py next to process the data.")


if __name__ == "__main__":
    main()
