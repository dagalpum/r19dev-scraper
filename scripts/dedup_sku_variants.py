#!/usr/bin/env python3
"""
Comprehensive Cleanup & Deduplication for R19DEV SQLite Database:
1. Filters out Photobooks, Posebooks, and Non-video items.
2. Filters out remaining Compilations, Box sets (42hr, 19hr), and Omnibus recuts.
3. Merges format variants: 77/88 (Outlet), BOD/DOD, 1...AI (Remasters), R... (Rental), etc.
4. Merges duplicate titles for the same actress into canonical JAV IDs.
"""

import sqlite3
import json
import os
import re

DB_PATH = os.path.expanduser("~/Library/Caches/r19dev/r19dev.db")

def is_photobook_or_nonvideo(title_ja, content_id):
    t = (title_ja or "").lower()
    c = (content_id or "").lower()
    book_keywords = [
        "写真集", "デジタル写真集", "ポーズブック", "フォトブック",
        "電子書籍", "グラビアスナック", "photobook", "posebook"
    ]
    if any(kw in t for kw in book_keywords):
        return True
    if c.startswith("g_") or (c.startswith("b") and not c.startswith(("bomn", "boie", "bobb"))):
        if any(kw in t for kw in ["写真", "グラビア", "ブック", "book"]):
            return True
    return False

def is_compilation(jav_id, title_ja):
    jid = (jav_id or "").upper()
    t = title_ja or ""
    tl = t.lower()

    if jid.startswith(("OFJE-", "OFRF-", "OFKU-", "OFMA-")):
        return True

    comp_keywords = [
        "総集編", "ベスト", "オムニバス", "コレクション", "傑作選", "名作選",
        "コンプリート", "メモリアル", "厳選", "セレクション", "全集", "パック",
        "box", "ハイライト", "ダイジェスト", "大乱交", "福袋", "まるごと収録",
        "選りすぐり", "大百科", "名場面", "大行進", "名鑑", "100選", "50選", "30選",
        "上半期傑作", "下半期傑作"
    ]
    for kw in comp_keywords:
        if kw in tl:
            return True

    for kw in ["best", "complete", "memorial", "omnibus", "selection", "digest"]:
        if kw in tl:
            return True

    if re.search(r"\d+時間", t):
        return True

    if re.search(r"\d{2,}発|\d{2,}本番|\d+連発|\d+射精|\d+sex", t, re.IGNORECASE):
        return True

    if re.search(r"\d+人[の\s]", t):
        return True

    return False

def normalize_id(dvd_id, content_id):
    raw = dvd_id.strip().upper() if dvd_id else (content_id or "").strip().upper()
    if not raw:
        return ""

    raw = re.sub(r"[-_]?(?:BOD|DOD)$", "", raw, flags=re.IGNORECASE)
    raw = re.sub(r"^(?:77|88)(?=[A-Za-z]{2,6})", "", raw)
    raw = re.sub(r"^(?:H_\d+|[DB]_)", "", raw, flags=re.IGNORECASE)
    raw = re.sub(r"^R(?=[A-Za-z]{3,5}-\d+)", "", raw)
    raw = re.sub(r"^(?:K9|KA9|KC9|TK|9|4)(?=[A-Za-z]{2,6}[-_]?\d+)", "", raw, flags=re.IGNORECASE)
    raw = re.sub(r"TK\d*$", "", raw, flags=re.IGNORECASE)

    m_ai = re.match(r"^1?([A-Z]{2,6})0*(\d{1,5})AI$", raw)
    if m_ai:
        num = int(m_ai.group(2))
        num_str = f"{num:03d}" if num < 1000 else str(num)
        return f"{m_ai.group(1)}-{num_str}"

    raw = re.sub(r"^[1-9](?=[A-Za-z]{3,5}\d+)", "", raw)

    m = re.match(r"^([A-Z]{2,6})[-_]?0*(\d{1,5})([A-Z]?)$", raw)
    if m:
        prefix = m.group(1)
        num = int(m.group(2))
        suffix = m.group(3) or ""
        num_str = f"{num:03d}" if num < 1000 else str(num)
        return f"{prefix}-{num_str}{suffix}"

    return raw

def clean_title_key(t):
    t = re.sub(r"【.*?】|（.*?）|\(.*?\)|[_\s\-]", "", t or "")
    return t

def main():
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()

    c.execute("SELECT movie_id FROM organized_movies")
    org_movie_ids = set(r[0] for r in c.fetchall())

    c.execute("SELECT id, combined_id, title, original_title, maker, release_date, cover_url, actresses_json, scraped_at FROM movies")
    rows = c.fetchall()

    print(f"Initial movie rows: {len(rows)}")

    # 1. Filter out photobooks and compilations that are not downloaded
    valid_rows = []
    deleted_nonvideo = 0
    deleted_comp = 0

    for r in rows:
        jid, cid, title, orig, maker, rdate, cov, act, scraped = r
        if jid in org_movie_ids:
            valid_rows.append(r)
            continue

        if is_photobook_or_nonvideo(title or orig, cid):
            c.execute("DELETE FROM movies WHERE id = ?", (jid,))
            deleted_nonvideo += 1
            continue

        if is_compilation(jid, title or orig):
            c.execute("DELETE FROM movies WHERE id = ?", (jid,))
            deleted_comp += 1
            continue

        valid_rows.append(r)

    conn.commit()
    print(f"Deleted non-video / photobooks: {deleted_nonvideo}")
    print(f"Deleted remaining compilations: {deleted_comp}")
    print(f"Remaining candidates to process: {len(valid_rows)}")

    # 2. Group by canonical ID and by cleaned title
    groups = {}
    title_to_group = {}

    for r in valid_rows:
        jid, cid, title, orig, maker, rdate, cov, act, scraped = r
        canon = normalize_id(jid, cid)
        if not canon:
            canon = jid

        ct = clean_title_key(title or orig)
        group_key = canon

        # If a group with this exact clean title already exists, merge into that group
        if len(ct) >= 15 and ct in title_to_group:
            group_key = title_to_group[ct]
        else:
            if len(ct) >= 15:
                title_to_group[ct] = group_key

        groups.setdefault(group_key, []).append({
            "id": jid,
            "combined_id": cid,
            "title": title,
            "original_title": orig,
            "maker": maker,
            "release_date": rdate,
            "cover_url": cov,
            "actresses_json": act,
            "scraped_at": scraped,
        })

    merged_groups_count = 0
    updated_single = 0

    for canon, items in groups.items():
        if len(items) == 1:
            item = items[0]
            if item["id"] != canon and item["id"] not in org_movie_ids:
                c.execute("UPDATE movies SET id = ? WHERE id = ?", (canon, item["id"]))
                updated_single += 1
            continue

        # Find primary item (prefer organized movie, then exact canon match)
        primary = None
        for it in items:
            if it["id"] in org_movie_ids:
                primary = it
                break
        if not primary:
            for it in items:
                if it["id"] == canon:
                    primary = it
                    break
        if not primary:
            primary = items[0]

        best_cover = ""
        for it in items:
            cov = it["cover_url"] or ""
            if "pl.jpg" in cov:
                best_cover = cov
                break
            elif cov and not best_cover:
                best_cover = cov

        best_date = ""
        for it in items:
            d = it["release_date"] or ""
            if d and (not best_date or d > best_date):
                best_date = d

        best_title = primary["title"] or ""
        best_orig_title = primary["original_title"] or ""
        for it in items:
            if not best_title and it["title"]:
                best_title = it["title"]
            if not best_orig_title and it["original_title"]:
                best_orig_title = it["original_title"]

        all_actresses = []
        seen_act_names = set()
        for it in items:
            try:
                acts = json.loads(it["actresses_json"]) if it["actresses_json"] else []
                for a in acts:
                    aname = a.get("name", "").lower()
                    if aname and aname not in seen_act_names:
                        seen_act_names.add(aname)
                        all_actresses.append(a)
            except Exception:
                pass

        merged_act_json = json.dumps(all_actresses, ensure_ascii=False) if all_actresses else primary["actresses_json"]

        # Delete all items in this group
        ids_to_del = [it["id"] for it in items]
        placeholders = ",".join(["?"] * len(ids_to_del))
        c.execute(f"DELETE FROM movies WHERE id IN ({placeholders})", ids_to_del)

        # Re-insert single merged canonical item
        final_id = primary["id"] if primary["id"] in org_movie_ids else canon
        c.execute("""
            INSERT INTO movies (id, combined_id, title, original_title, maker, release_date, cover_url, actresses_json, scraped_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            final_id,
            primary["combined_id"],
            best_title,
            best_orig_title,
            primary["maker"],
            best_date,
            best_cover,
            merged_act_json,
            primary["scraped_at"]
        ))
        merged_groups_count += 1

    conn.commit()
    c.execute("SELECT count(*) FROM movies")
    final_count = c.fetchone()[0]
    print(f"Updated single non-canonical IDs: {updated_single}")
    print(f"Merged variant groups: {merged_groups_count}")
    print(f"Final movie count in DB: {final_count}")
    conn.close()

if __name__ == "__main__":
    main()
