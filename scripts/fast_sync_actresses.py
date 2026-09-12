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
    """Normalize DVD ID or DMM Content ID to canonical JAV-ID (e.g. SNOS-115, MIDA-517)."""
    raw = dvd_id.strip().upper() if dvd_id else (content_id or "").strip().upper()
    if not raw:
        return ""

    # Strip BOD / DOD suffix (Built On Demand / Disc On Demand DVD-R)
    raw = re.sub(r"[-_]?(?:BOD|DOD)$", "", raw, flags=re.IGNORECASE)

    # Strip DMM Outlet discount prefixes (77 = DVD outlet, 88 = Blu-ray outlet)
    raw = re.sub(r"^(?:77|88)(?=[A-Za-z]{2,6})", "", raw)

    # Strip DMM special prefixes (H_xxx, DB_)
    raw = re.sub(r"^(?:H_\d+|[DB]_)", "", raw, flags=re.IGNORECASE)

    # Strip Rental prefix R (e.g. RSDMT-286 -> SDMT-286)
    raw = re.sub(r"^R(?=[A-Za-z]{3,5}-\d+)", "", raw)

    # Strip DMM edition prefixes (9 = Blu-ray, K9 = Limited Blu-ray, KA9/KC9 = Tickets, TK = FANZA special, 4 = Aircontrol)
    raw = re.sub(r"^(?:K9|KA9|KC9|TK|9|4)(?=[A-Za-z]{2,6}[-_]?\d+)", "", raw, flags=re.IGNORECASE)
    raw = re.sub(r"TK\d*$", "", raw, flags=re.IGNORECASE)

    # Strip AI remaster patterns (e.g. 1SDMT00286AI -> SDMT-286)
    m_ai = re.match(r"^1?([A-Z]{2,6})0*(\d{1,5})AI$", raw)
    if m_ai:
        num = int(m_ai.group(2))
        num_str = f"{num:03d}" if num < 1000 else str(num)
        return f"{m_ai.group(1)}-{num_str}"

    # Strip single digit maker prefix (e.g. 2BOM077 -> BOM-077, 2DVAJ541 -> DVAJ-541, 5MIR117 -> MIR-117)
    raw = re.sub(r"^[1-9](?=[A-Za-z]{3,5}\d+)", "", raw)

    # Standard pattern: 2-6 letters + numbers + optional single letter suffix
    m = re.match(r"^([A-Z]{2,6})[-_]?0*(\d{1,5})([A-Z]?)$", raw)
    if m:
        prefix = m.group(1)
        num = int(m.group(2))
        suffix_letter = m.group(3) or ""
        num_str = f"{num:03d}" if num < 1000 else str(num)
        return f"{prefix}-{num_str}{suffix_letter}"

    return raw

def is_photobook_or_nonvideo(title_ja, content_id):
    """Check if item is a digital photobook, pose book, or non-video release."""
    t = (title_ja or "").lower()
    c = (content_id or "").lower()
    
    book_keywords = [
        "写真集", "デジタル写真集", "ポーズブック", "フォトブック",
        "電子書籍", "グラビアスナック", "photobook", "posebook"
    ]
    if any(kw in t for kw in book_keywords):
        return True

    # DMM Book / E-Book category prefix (starts with 'b' followed by numbers, but not maker codes like bomn/boie/bobb)
    if c.startswith("g_") or (c.startswith("b") and not c.startswith(("bomn", "boie", "bobb"))):
        if any(kw in t for kw in ["写真", "グラビア", "ブック", "book"]):
            return True

    return False

def is_compilation(jav_id, title_ja):
    """Check if release is an omnibus, best-of, or multi-hour compilation recut."""
    jid = (jav_id or "").upper()
    t = title_ja or ""
    tl = t.lower()

    # Common compilation maker/series prefixes
    if jid.startswith(("OFJE-", "OFRF-", "OFKU-", "OFMA-")):
        return True

    # Title keywords indicating omnibus / compilation / best-of / multi-scene recuts
    comp_keywords = [
        "総集編", "ベスト", "オムニバス", "コレクション", "傑作選", "名作選",
        "コンプリート", "メモリアル", "厳選", "セレクション", "全集", "パック",
        "box", "ハイライト", "ダイジェスト", "大乱交", "福袋", "まるごと収録",
        "選りすぐり", "大百科", "名場面", "大行進", "名鑑", "100選", "50選", "30選"
    ]
    for kw in comp_keywords:
        if kw in tl:
            return True

    for kw in ["best", "complete", "memorial", "omnibus", "selection", "digest"]:
        if kw in tl:
            return True

    # Multi-hour recuts (e.g. 4時間, 6時間, 8時間, 10時間, 12時間, 16時間, 19時間, 24時間, 42時間)
    if re.search(r"\d+時間", t):
        return True

    # Multi-scene omnibus patterns (e.g. 50発, 100発, 100本番, 10連発)
    if re.search(r"\d{2,}発|\d{2,}本番|\d+連発|\d+射精|\d+sex", t, re.IGNORECASE):
        return True

    # Multi-actress omnibus indicators in title (e.g. 10人の美女, 16人, 20人, 46人, 48人)
    if re.search(r"\d+人[の\s]", t):
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

    # Deduplicate raw items to canonical JAV IDs and unique titles
    unique_movies = {}
    title_to_id = {}

    for item in all_raw_items:
        title_ja = item.get("title_ja") or ""
        cid = item.get("content_id") or ""
        dvd_id = item.get("dvd_id")

        if is_photobook_or_nonvideo(title_ja, cid):
            continue

        jav_id = normalize_id(dvd_id, cid)
        if not jav_id:
            continue

        if is_compilation(jav_id, title_ja):
            continue

        date = item.get("release_date") or ""
        cover_url = get_high_res_cover(item.get("jacket_thumb_url"))

        # Clean title key to merge duplicate releases of same film (e.g. PPB-269 vs PPBD-269)
        clean_t = re.sub(r"【.*?】|（.*?）|\(.*?\)|[_\s\-]", "", title_ja) if title_ja else ""
        if len(clean_t) >= 15 and clean_t in title_to_id:
            canon_id = title_to_id[clean_t]
            existing = unique_movies[canon_id]
            if not existing["release_date"] and date:
                existing["release_date"] = date
            if (not existing["cover_url"] or "ps.jpg" in existing["cover_url"]) and cover_url:
                existing["cover_url"] = cover_url
            continue

        if jav_id not in unique_movies:
            unique_movies[jav_id] = {
                "id": jav_id,
                "combined_id": cid,
                "title": title_ja or jav_id,
                "original_title": title_ja,
                "release_date": date,
                "cover_url": cover_url,
            }
            if len(clean_t) >= 15:
                title_to_id[clean_t] = jav_id
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
