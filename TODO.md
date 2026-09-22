# 📋 รายการสิ่งที่ต้องทำและแผนการดำเนินงาน (TODO & Roadmap)

**โครงการ**: `r19dev-scraper` — ระบบสแกน ค้นหาข้อมูล จัดระเบียบ และคลังสื่อ JAV แบบอัตโนมัติ  
**อัปเดตล่าสุด**: 13 กันยายน 2026 (v1.8.0)

---

## 🚀 งานที่ทำเสร็จสิ้นแล้ว (Completed)

### 1. แก้ไขข้อผิดพลาดและปรับปรุงความเสถียรของ Web UI
- [x] **แก้ไขข้อผิดพลาด Tab Library โหลดไม่ขึ้น (TypeError Bug):**
  - แก้ไขปัญหาใน `pkg/web/static/js/actress.js` ที่ฟังก์ชัน `renderAllMoviesCatalogHtml()` อ้างอิงตัวแปร `state.movieStatuses` ผิด (ในระบบจริงคือ `state.userStates`) จนเกิดข้อผิดพลาด `Cannot read properties of undefined (reading 'SNOS-343')`
  - ปรับใช้ Safe Optional Chaining (`state.userStates?.[m.movie_id]`) พร้อม Fallback ค่าจากอ็อบเจกต์ภาพยนตร์โดยตรง ทำให้หน้า Library โหลดแสดงผลได้อย่างสมบูรณ์และปลอดภัย
- [x] **เพิ่ม Native SVG Favicon Handler:**
  - เพิ่ม Route `/favicon.ico` ใน `pkg/web/server.go` ส่งคืนไอคอน Movie Reel SVG พร้อม HTTP Cache 86400s ป้องกันการเกิดข้อผิดพลาด 404 ในเบราว์เซอร์คอนโซล

### 2. ปรับปรุงประสิทธิภาพ Scanner สำหรับเน็ตเวิร์ก NAS / SMB
- [x] **Optimized WalkDir Traversal:**
  - ปรับแต่ง `pkg/scanner/scanner.go` ให้ตรวจสอบและข้ามโฟลเดอร์ที่ไม่ใช่วิดีโอทันที (`.actors`, `extrafanart`, `@eaDir`, และโฟลเดอร์ซ่อนที่ขึ้นต้นด้วย `.`) โดยไม่ต้องรัน `os.Lstat` ซ้ำซ้อน
  - ช่วยลดเวลาในการสแกนคลัง `/Volumes/home/BT/Archive` ที่มีภาพย่อยกว่า 15,000 รูป จากเดิมที่ติดค้าง/Timeout ให้เสร็จสิ้นภายในเวลาเพียงไม่กี่วินาที

### 3. ยกเครื่องดีไซน์และฟังก์ชัน `movie.html` (Cinematic Offline Viewer)
- [x] **Cinematic Backdrop Hero:** ดึงภาพ `fanart.jpg` มาทำเป็นพื้นหลังแบนเนอร์เบลอระดับพรีเมียม (Cinematic Blur + Dark Gradient Fade) เหมือนระบบสตรีมมิงชั้นนำ พร้อมระบบ Fallback อัตโนมัติ
- [x] **Interactive Action Bar:**
  - ปุ่ม `▶ Play Movie` ลิงก์ตรงไปยังไฟล์วิดีโอบนเครื่อง (เช่น `SNOS-049.mp4`) เพื่อเปิดเล่นด้วยเครื่องเล่นประจำเครื่อง (IINA, VLC, QuickTime)
  - ปุ่ม `🖥️ Watch in Browser` เปิด Pop-up HTML5 Video Player ภายในเบราว์เซอร์ทันที
  - ปุ่ม `🎬 Watch Trailer` ดูตัวอย่างคลิปสั้นจาก Official Stream
  - ปุ่ม `🏠 R19dev Hub` ลิงก์ตรงกลับไปยังแดชบอร์ดหลัก `http://localhost:8080`
- [x] **Smart Multi-Part Support:** ตรวจจับไฟล์วิดีโอที่มีหลายพาร์ทอัตโนมัติ (เช่น `-pt1.mp4`, `-pt2.mp4`) พร้อมสร้างปุ่ม `▶ Play Part 1`, `▶ Play Part 2` แยกให้ทันที
- [x] **In-Page Lightbox Gallery:** แกลเลอรีภาพฉากตัวอย่างในเรื่องขยายเปิดเป็น Lightbox Modal เต็มจอในหน้าเดิม ไม่เด้งเปิด Tab ใหม่ รองรับการกดปุ่มลูกศรคีย์บอร์ด `←` / `→` เพื่อเลื่อนดูภาพ และปุ่ม `ESC` เพื่อปิด
- [x] **One-Click Copy JAV ID:** คลิกที่ Badge รหัสหนังเพื่อคัดลอกรหัสเข้า Clipboard ทันที พร้อม Floating Toast Alert แจ้งเตือน
- [x] **Local Asset Auto-Discovery:** ตรวจสแกนไฟล์ภาพใน `extrafanart/` และไฟล์วิดีโอในเครื่องโดยอัตโนมัติ ทำงานแบบ Pure Vanilla CSS/JS แบบออฟไลน์ 100%

### 4. สำรวจและวิเคราะห์คลัง `/Volumes/home/BT/Archive/`
- [x] **สแกนโครงสร้างทั้งหมด:** พบ 525 วิดีโอไฟล์ (อยู่ใน 521 โฟลเดอร์หนัง) แยก 50 โฟลเดอร์นักแสดง
- [x] **จับคู่กับ `r18_dump.db`:** แมตช์ข้อมูลภาษาอังกฤษและรูปภาพครบถ้วน 100% (524 เรื่องเป็นภาพยนตร์ใหม่ และพบเรื่องที่ซ้ำกับคลังปัจจุบัน 1 เรื่องคือ `IPZZ-751`)
- [x] **ทดสอบการจัดระเบียบ (Simulation):** สร้างแผนและตรวจสอบการชนกันของไฟล์ (0 Conflicts)

### 5. ระบบจัดระเบียบและโยกย้ายไฟล์อัจฉริยะ (Migration & TUI Package)
- [x] **สร้าง Package `pkg/migrator` อย่างเป็นทางการ:**
  - เพิ่มคำสั่ง `r19dev migrate <source> [dest] [--dry-run] [--no-tui] [-y/--yes]`
  - ระบบ Discovery สตรีมมิ่งความคืบหน้าแบบเรียลไทม์ (ทุกๆ 2 ไฟล์) พร้อมแอนิเมชัน Spinner หมุนสด
  - หน้าต่าง TUI คำนวณความสูงหน้าจอแบบ Dynamic ขยาย `Recent Activity` เต็มความสูง Terminal
  - ระบบ Pre-flight Summary ตรวจสอบไฟล์ล่วงหน้า แสดงตารางสรุปผล และถามยืนยันก่อนเริ่มย้ายจริง
  - ย้ายไฟล์แบบ Instant Atomic Rename ภายใน Synology Volume เดียวกัน รวดเร็วและปลอดภัย 100%
  - สร้างไฟล์ `{JAV-ID}.nfo` และ Cinematic `movie.html` รุ่นล่าสุดให้ทุกเรื่องอัตโนมัติ

### 6. ดำเนินการย้ายคลัง Archive สำเร็จ 100% (Archive Migration Execution)
- [x] **โยกย้าย 525 เรื่องจาก `/Volumes/home/BT/Archive/` สู่ `/Volumes/home/BT/organized/`:**
  - ย้ายไฟล์วิดีโอ 525 ไฟล์โดยไม่เกิด Error ใดๆ ใช้เวลาเพียง 6 นาที 59 วินาที (1.3 เรื่อง/วินาที)
  - นำเข้าภาพ `poster.jpg`, `fanart.jpg`, และแกลเลอรี `extrafanart/` ครบทุกเรื่อง
  - อัปเดตไฟล์ `movie.html` ของภาพยนตร์เดิมในคลังทั้งหมดให้เป็น Cinematic Backdrop Viewer
  - ทำความสะอาดลบโฟลเดอร์ว่างใน Archive ทั้งหมด 916 โฟลเดอร์อย่างปลอดภัย คงเหลือเฉพาะไฟล์ตกค้างที่ไม่ระบุชื่อ (`@Unknown/.mp4`) และไฟล์ดัชนี

### 7. ปรับมาตรฐานชื่อโฟลเดอร์นักแสดงเป็นภาษาอังกฤษ (Option 1: Firstname Lastname)
- [x] **Standardize English Actress Folders 100%:**
  - ตรวจพบนักแสดง 19 คนที่มีชื่อโฟลเดอร์เป็นภาษาญี่ปุ่น เนื่องจากใน Dump ไม่มี Romaji
  - ทำการแมปและเปลี่ยนชื่อโฟลเดอร์ทั้ง 19 คนเป็นภาษาอังกฤษสากลตามแบบที่ 1 (Firstname Lastname) ทั้งหมด เช่น:
    - `入田真綾` -> `Maaya Irita`
    - `日向かえで` / `日向かえで` -> `Kaede Hinata`
    - `五日市芽依` -> `Mei Itsukaichi`
    - `日向陽葵` -> `Himari Hinata`
    - `八蜜凛` -> `Rin Hachimitsu`
  - อัปเดต Path ในฐานข้อมูล SQLite (`organized_movies`, `library_files`, และ `actresses`) ให้ตรงกันสมบูรณ์
  - เพิ่มพจนานุกรมชื่อและตรรกะ ASCII Detection ลงใน `pkg/migrator/engine.go` ป้องกันการสร้างโฟลเดอร์ภาษาญี่ปุ่นในอนาคต
  - ปัจจุบันคลังปลายทางมีโฟลเดอร์นักแสดงทั้งหมด 60 คน เป็นภาษาอังกฤษตามมาตรฐาน 100%

### 8. เพิ่มระบบ Video Player Modal บน Web UI
- [x] **Inline Streaming Player:**
  - เพิ่ม Route `/api/stream/{id}` รองรับ HTTP Range Requests (Status 206 Partial Content)
  - เพิ่มปุ่ม `▶ Play` ใน Modal ข้อมูลภาพยนตร์บน Web UI สามารถสตรีมดูวิดีโอผ่านเบราว์เซอร์ได้ทันที
  - ดีไซน์สวยงามระดับพรีเมียม เข้ากับธีม Dark Glassmorphism ของ R19DEV Studio

### 9. ระบบค้นหา Torrent และจัดการดาวน์โหลด Transmission บน Web UI
- [x] **Torrent Discovery & 1-Click Download ใน Movie Detail Modal:**
  - เพิ่มระบบค้นหา Torrent จาก Sukebei Nyaa แบบอัจฉริยะ (Smart Scoring) แสดง Badge คุณภาพชัดเจน (4K UHD, 1080p, 🔓 Uncensored, 💬 Subtitles, ⭐ Best Match, Seeders/Leechers)
  - รองรับการค้นหาอัตโนมัติเมื่อเปิดดูข้อมูลหนังที่ขาด (Missing Status) พร้อมปุ่มค้นหาด้วย JAV ID หรือ Title
  - ปุ่ม 1-Click ส่ง Magnet/Torrent Link เข้าคิว Transmission Daemon ได้ทันที
- [x] **Downloads & Staging Queue Drawer:**
  - เพิ่มปุ่ม Downloads พร้อมตัวเลขสถานะงาน (Badge Count) ที่แถบเมนูหลักด้านบน (Navbar)
  - แผง Drawer แสดงรายการดาวน์โหลดแบบเรียลไทม์: รหัสหนัง, ปก, ชื่อ Torrent, Progress Bar (%), ความเร็วการดาวน์โหลด (Speed), และเวลาที่เหลือ (ETA)
  - ปุ่มคำสั่งต่อรายการ: จัดระเบียบเข้าคลังทันที (`⚡ Organize Now`), เปิดโฟลเดอร์ใน Finder, และลบออกจากคิว
  - ระบบ Auto-polling อัปเดตสถานะอัตโนมัติเมื่อมีงานกำลังดาวน์โหลด
- [x] **Settings Modal:**
  - ปุ่มตั้งค่า (ไอคอนฟันเฟือง) บน Navbar สำหรับกำหนดค่า Transmission RPC URL, Username, Password, Download Directory และ Sukebei URL
  - ปุ่ม `🔌 Test Connection` ทดสอบการเชื่อมต่อกับ Transmission พร้อมแสดงผลและเวอร์ชันทันที
  - สวิตช์เปิด/ปิด `Auto-Organize Completed Downloads`

### 10. ระบบ Auto-Organize Pipeline (Staging ➔ Organized NAS)
- [x] **On-Demand & Background Staging Organizer (`/api/torrents/queue/organize`):**
  - ตรวจหาไฟล์วิดีโอที่ดาวน์โหลดเสร็จแล้วใน Staging/Transmission Directory
  - ดึงข้อมูลอภิพันธุ์ (Metadata) จาก Tier-1 Offline DumpStore และจัดระเบียบไฟล์เข้าโครงสร้าง Jellyfin บน NAS อัตโนมัติ พร้อมสร้าง NFO, ภาพปก, ภาพฉาก และ `movie.html`
  - บันทึกสถานะเข้า SQLite `download_queue` และ `organized_movies`
- [x] **Transmission Webhook Receiver (`/api/webhook/download-complete` & `/api/torrents/webhook`):**
  - รองรับการเรียกจากสคริปต์เมื่อทอร์เรนต์ดาวน์โหลดเสร็จ (`script-torrent-done-filename`) ของ Transmission
  - ปรับสถานะเป็น `staging` และสั่งจัดระเบียบเข้าคลังโดยอัตโนมัติหากเปิดการตั้งค่า Auto-Organize

---

## 📊 สถานะคลังภาพยนตร์ปัจจุบัน (Library Status)
- **ตำแหน่งคลังหลัก**: `/Volumes/home/BT/organized`
- **จำนวนภาพยนตร์ในคลัง**: 593 เรื่อง (ครอบคลุม 60 นักแสดง)
- **จำนวนไฟล์วิดีโอ**: 609 ไฟล์
- **สถานะ Web UI Server**: รันอยู่ที่ `http://localhost:8080` (Targeting `/Volumes/home/BT/organized`)
