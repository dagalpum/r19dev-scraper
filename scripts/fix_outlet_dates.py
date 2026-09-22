#!/usr/bin/env python3
"""
Fix Outlet Dates and Content IDs
Restores authentic release dates and content_ids for movies corrupted by DMM outlet/campaign SKUs (77/88 prefix).
"""

import sqlite3
import re
import os

R19_DB = os.path.expanduser("~/Library/Application Support/r19dev/r19dev.db")
R18_DB = os.path.expanduser("~/Library/Application Support/r19dev/r18_dump.db")

def main():
    r19_conn = sqlite3.connect(R19_DB)
    r18_conn = sqlite3.connect(R18_DB)
    
    r19_c = r19_conn.cursor()
    r18_c = r18_conn.cursor()
    
    # 1. Clean up phantom duplicate 77/88 movie records that have a clean counterpart
    r19_c.execute("SELECT id FROM movies WHERE id LIKE '77%' OR id LIKE '88%'")
    phantom_ids = [r[0] for r in r19_c.fetchall()]
    deleted_phantoms = 0
    for pid in phantom_ids:
        clean_id = re.sub(r'^(?:77|88)', '', pid)
        # Check if clean_id exists
        r19_c.execute("SELECT id FROM movies WHERE id = ?", (clean_id,))
        if r19_c.fetchone():
            # Merge movie_actresses
            r19_c.execute("UPDATE OR IGNORE movie_actresses SET movie_id = ? WHERE movie_id = ?", (clean_id, pid))
            r19_c.execute("DELETE FROM movie_actresses WHERE movie_id = ?", (pid,))
            r19_c.execute("DELETE FROM movies WHERE id = ?", (pid,))
            deleted_phantoms += 1
            print(f"Merged & deleted phantom duplicate: {pid} -> {clean_id}")
    
    # 2. Fix corrupted dates and combined_ids for movies with 77/88 combined_id or future/re-release dates
    r19_c.execute("SELECT id, combined_id, release_date FROM movies")
    all_movies = r19_c.fetchall()
    
    fixed_count = 0
    for mid, cid, rdate in all_movies:
        m = re.match(r'^([A-Za-z]+)-?0*(\d+)([A-Za-z]?)$', mid)
        candidates = [mid.lower(), mid.lower().replace('-', '')]
        if m:
            prefix = m.group(1).lower()
            num = int(m.group(2))
            suffix = m.group(3).lower()
            candidates.extend([
                f'{prefix}{num}{suffix}',
                f'{prefix}{num:03d}{suffix}',
                f'{prefix}{num:05d}{suffix}',
                f'{prefix}-{num:03d}{suffix}',
                f'{prefix}-{num}{suffix}'
            ])
        candidates = list(set(candidates))
        placeholders = ','.join('?' for _ in candidates)
        
        r18_c.execute(f'''
            SELECT content_id, dvd_id, release_date 
            FROM r18_movies 
            WHERE (content_id IN ({placeholders}) OR clean_id IN ({placeholders}) OR dvd_id IN ({placeholders}))
              AND NOT content_id LIKE '77%' AND NOT content_id LIKE '88%'
              AND release_date IS NOT NULL AND release_date != ''
            ORDER BY release_date ASC
        ''', candidates * 3)
        rows = r18_c.fetchall()
        if rows:
            auth_cid, auth_dvd, auth_date = rows[0]
            # If current date is corrupted (future date > auth_date, or cid starts with 77/88)
            should_fix = False
            new_cid = cid
            new_date = rdate
            
            if cid and (cid.startswith('77') or cid.startswith('88')):
                should_fix = True
                new_cid = auth_cid
            
            if rdate != auth_date and (rdate > auth_date and (rdate >= '2025-01-01' or (int(rdate[:4]) - int(auth_date[:4])) >= 1)):
                should_fix = True
                new_date = auth_date
                if not new_cid or new_cid.startswith(('77', '88')):
                    new_cid = auth_cid
            
            if should_fix:
                r19_c.execute('''
                    UPDATE movies 
                    SET release_date = ?, combined_id = ? 
                    WHERE id = ?
                ''', (new_date, new_cid, mid))
                fixed_count += 1
                print(f"Fixed {mid:<15}: CID {cid} -> {new_cid} | Date {rdate} -> {new_date}")
    
    r19_conn.commit()
    print(f"\nDone! Deleted {deleted_phantoms} phantom records. Fixed {fixed_count} corrupted release dates / combined_ids.")

if __name__ == '__main__':
    main()
