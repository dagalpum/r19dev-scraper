#!/usr/bin/env python3
"""
Fast Filmography Sync (Phase 1)
Fetches all known releases from R18.dev for followed actresses,
normalizes to canonical JAV IDs, extracts high-res covers,
and synchronizes with local SQLite database (~/Library/Caches/r19dev/r19dev.db).
"""

import sys
import os
import json
import sqlite3
import urllib.request
import urllib.error
import re
import time
from datetime import datetime

DB_PATH = os.path.expanduser("~/Library/Caches/r19dev/r19dev.db")
PAGE_SIZE = 30
HEADERS = {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
    "Accept": "application/json, text/plain, */*",
    "Accept-Language": "en-US,en;q=0.9,ja;q=0.8",
}

def normalize_id(dvd_id, content_id):
    """Normalize DVD ID or DMM Content ID to canonical JAV-ID (e.g. SNOS-115, OFJE-638, PRWF-016)."""
    if dvd_id:
        cleaned = dvd_id.strip().upper()
        # If matches format like ABC-123 or ABC_123 or ABCD-1234
        m = re.match(r"^([A-Z0-9]{2,8})[-_](\d{2,5}[A-Z]?)$", cleaned)
        if m:
            return f"{m.group(1)}-{m.group(2)}"
        return cleaned

    cid = content_id.lower().strip()
    # Strip common DMM special edition / maker / event prefixes
    cid = re.sub(r"^h_\d+", "", cid)      # e.g. h_346rebdb1046 -> rebdb1046
    cid = re.sub(r"^k[ac]9", "", cid)     # online autograph session tickets e.g. ka9oae308, kc9oae308
    cid = re.sub(r"^k9", "", cid)         # limited edition e.g. k9snos209 -> snos209
    cid = re.sub(r"^9(?=[a-z]{2,5}\d+)", "", cid) # blu-ray e.g. 9ofje638 -> ofje638
    cid = re.sub(r"^tk(?=[a-z]{3,5}\d+)", "", cid) # FANZA special e.g. tkprwf016, tkoae291
    cid = re.sub(r"^4(?=oae\d+)", "", cid) # Aircontrol 4oae244 -> oae244
    cid = re.sub(r"^[db]_", "", cid)
    cid = re.sub(r"tk\d*$", "", cid)

    # Standard pattern: 2-6 letters + numbers
    m = re.match(r"^([a-z]{2,6})0*(\d{2,5})$", cid)
    if m:
        prefix = m.group(1).upper()
        num_val = int(m.group(2))
        # Use at least 3 digits if original was short, else exact digits
        num_str = f"{num_val:03d}" if num_val < 1000 else str(num_val)
        return f"{prefix}-{num_str}"

    return cid.upper()

def is_photobook_or_nonvideo(title_ja, content_id):
    """Check if item is a digital photobook rather than a video release."""
    t = (title_ja or "").lower()
    c = (content_id or "").lower()
    if "写真集" in title_ja or "デジタル写真集" in title_ja or "photobook" in t:
        return True
    if c.startswith("g_") or c.startswith("b_") or "pb" in c:
        if "写真集" in title_ja:
            return True
    return False

def get_high_res_cover(thumb_url):
    """Convert small DMM thumbnail (ps.jpg) to full resolution jacket (pl.jpg)."""
    if not thumb_url:
        return ""
    if "ps.jpg" in thumb_url:
        return thumb_url.replace("ps.jpg", "pl.jpg")
    return thumb_url

def fetch_page_with_retry(url, max_retries=3):
    """Fetch JSON from URL with exponential backoff on 429/5xx."""
    for attempt in range(max_retries):
        try:
            req = urllib.request.Request(url, headers=HEADERS)
            with urllib.request.urlopen(req, timeout=15) as resp:
                if resp.status == 200:
                    return json.load(resp)
        except urllib.error.HTTPError as e:
            if e.code == 429:
                wait_time = (attempt + 1) * 3
                print(f"    [Rate Limited (429)] Waiting {wait_time}s before retry...", flush=True)
                time.sleep(wait_time)
            elif e.code in (500, 502, 503):
                time.sleep(2)
            else:
                raise e
        except Exception as e:
            if attempt < max_retries - 1:
                time.sleep(2)
            else:
                raise e
    return None

def sync_actress(conn, actress):
    name, ja_name, r18_id = actress
    print(f"\n▶ [{name}] ({ja_name}) - R18 ID: {r18_id}", flush=True)
    if not r18_id:
        print("   Skipped: No R18 ID found.", flush=True)
        return 0, 0

    c = conn.cursor()
    page = 1
    total_results = None
    all_raw_items = []

    while True:
        url = f"https://r18.dev/videos/vod/movies/list2/json?id={r18_id}&type=actress&page={page}"
        data = fetch_page_with_retry(url)
        if not data:
            break

        if total_results is None:
            total_results = data.get("total_results", 0)
            total_pages = (total_results + PAGE_SIZE - 1) // PAGE_SIZE if total_results > 0 else 1
            print(f"   Total R18 Results: {total_results} ({total_pages} pages to scan)", flush=True)

        results = data.get("results", [])
        if not results:
            break

        all_raw_items.extend(results)
        if len(all_raw_items) >= total_results or len(results) < PAGE_SIZE:
            break

        page += 1
        time.sleep(0.25) # Polite throttle between pages

    # Deduplicate raw items to canonical JAV IDs
    unique_movies = {}
    for item in all_raw_items:
        title_ja = item.get("title_ja") or ""
        cid = item.get("content_id") or ""
        dvd_id = item.get("dvd_id")

        if is_photobook_or_nonvideo(title_ja, cid):
            continue

        jav_id = normalize_id(dvd_id, cid)
        if not jav_id:
            continue

        date = item.get("release_date") or ""
        cover_url = get_high_res_cover(item.get("jacket_thumb_url"))

        if jav_id not in unique_movies:
            unique_movies[jav_id] = {
                "id": jav_id,
                "combined_id": cid,
                "title": title_ja or jav_id,
                "original_title": title_ja,
                "release_date": date,
                "cover_url": cover_url,
            }
        else:
            # If current item has a better date or cover, update
            existing = unique_movies[jav_id]
            if not existing["release_date"] and date:
                existing["release_date"] = date
            if (not existing["cover_url"] or "ps.jpg" in existing["cover_url"]) and cover_url:
                existing["cover_url"] = cover_url
            if dvd_id and not existing["id"].count("-"):
                existing["id"] = normalize_id(dvd_id, cid)

    # Upsert into database
    new_count = 0
    updated_count = 0
    now_str = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    for jav_id, m in unique_movies.items():
        c.execute("SELECT id, title, cover_url, actresses_json FROM movies WHERE id = ?", (jav_id,))
        row = c.fetchone()

        actress_obj = {"id": r18_id, "name": name, "ja_name": ja_name}

        if row is None:
            # Brand new movie release record
            actresses_json = json.dumps([actress_obj], ensure_ascii=False)
            c.execute("""
                INSERT INTO movies (
                    id, combined_id, title, original_title, maker, release_date, cover_url, actresses_json, scraped_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, (
                jav_id,
                m["combined_id"],
                m["title"],
                m["original_title"],
                "",
                m["release_date"],
                m["cover_url"],
                actresses_json,
                now_str
            ))
            new_count += 1
        else:
            # Existing movie: merge actress into actresses_json if not already present
            existing_actresses = []
            try:
                if row[3]:
                    existing_actresses = json.loads(row[3])
            except Exception:
                existing_actresses = []

            names_list = [a.get("name", "").lower() for a in existing_actresses if isinstance(a, dict)]
            if name.lower() not in names_list:
                existing_actresses.append(actress_obj)
                new_actresses_json = json.dumps(existing_actresses, ensure_ascii=False)
                c.execute("UPDATE movies SET actresses_json = ? WHERE id = ?", (new_actresses_json, jav_id))
                updated_count += 1

            # Update cover_url if local DB had no cover or empty
            if not row[2] and m["cover_url"]:
                c.execute("UPDATE movies SET cover_url = ? WHERE id = ?", (m["cover_url"], jav_id))
                updated_count += 1

    conn.commit()
    print(f"   ✔ Synced: {len(unique_movies)} unique video releases ({new_count} new added, {updated_count} updated)", flush=True)
    return len(unique_movies), new_count

def main():
    print("=" * 70)
    print("  🚀 R19DEV Studio: Fast Filmography Sync (Phase 1)")
    print(f"  Target SQLite: {DB_PATH}")
    print("=" * 70, flush=True)

    if not os.path.exists(DB_PATH):
        print(f"Error: Database file not found at {DB_PATH}")
        sys.exit(1)

    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()
    c.execute("SELECT name, ja_name, r18_id FROM actresses ORDER BY name ASC")
    actresses = c.fetchall()

    print(f"Found {len(actresses)} followed actresses to synchronize.\n")

    start_time = time.time()
    total_unique = 0
    total_new = 0

    for idx, act in enumerate(actresses, 1):
        print(f"[{idx}/{len(actresses)}]", end=" ")
        unique_cnt, new_cnt = sync_actress(conn, act)
        total_unique += unique_cnt
        total_new += new_cnt

    elapsed = time.time() - start_time
    conn.close()

    print("\n" + "=" * 70)
    print("  🎉 Phase 1 Sync Complete!")
    print(f"  • Actresses processed: {len(actresses)}")
    print(f"  • Total unique releases tracked: {total_unique}")
    print(f"  • New releases added to database: {total_new}")
    print(f"  • Time taken: {elapsed:.1f} seconds ({elapsed / 60:.1f} minutes)")
    print("=" * 70, flush=True)

if __name__ == "__main__":
    main()
