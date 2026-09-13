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

---

## 🔄 แผนการดำเนินงานและขั้นตอนถัดไป (Roadmap & Next Steps)

### ระยะที่ 1: ดำเนินการย้ายและจัดระเบียบคลัง Archive (Option 2 Execution)
- [ ] **รันการจัดระเบียบไฟล์ 525 เรื่องจาก `/Volumes/home/BT/Archive/` ไปยัง `/Volumes/home/BT/organized/`:**
  - ย้ายไฟล์วิดีโอและจัดรูปแบบชื่อโฟลเดอร์ปลายทางตามมาตรฐาน `{Actress}/{JAV-ID} {English Title}/`
  - นำไฟล์ภาพเดิม (`folder.jpg` -> `poster.jpg`, `fanart.jpg`, `extrafanart/`) ย้ายมาที่ปลายทางทันที (Instant Move ภายใน Synology Volume เดียวกัน)
  - สร้างไฟล์ `{JAV-ID}.nfo` และ `movie.html` เวอร์ชันใหม่ล่าสุดให้ครบทุกเรื่อง
  - เคลียร์โฟลเดอร์ว่างใน Archive ให้สะอาดเรียบร้อย
  - ข้ามไฟล์ตกค้างที่ไม่ระบุชื่อ (`/Volumes/home/BT/Archive/@Unknown/.mp4`) และบันทึกรายงานให้ผู้ใช้ทราบ
- [ ] **อัปเดตไฟล์ `movie.html` เดิมใน `/Volumes/home/BT/organized/`:**
  - วนลูปอัปเดตไฟล์ `movie.html` ของภาพยนตร์เดิมในคลังให้เป็นเวอร์ชัน Cinematic Viewer ใหม่ทั้งหมด เพื่อให้มีมาตรฐานเดียวกัน 100%

### ระยะที่ 2: การซิงก์และอัปเดตฐานข้อมูล (Database Sync)
- [ ] **บันทึกข้อมูลเข้า `r19dev.db`:**
  - อัปเดตตาราง `movies`, `library_files`, และ `actresses` ให้มีข้อมูลของภาพยนตร์ใหม่ทั้ง 525 เรื่องครบถ้วน
  - อัปเดตเปอร์เซ็นต์ความคืบหน้า (Completion Metrics) ของนักแสดงทั้ง 13 คนที่ติดตามอยู่ (Followed Actresses) เช่น JULIA, Sakura Miura, Miru, Mayuki Ito เป็นต้น
  - นำรายชื่อนักแสดงใหม่อีก 36 คนเข้าสู่หมวดหมู่ Unfollowed Actresses เพื่อให้เลือกติดตามได้สะดวก

### ระยะที่ 3: การตรวจสอบและสรุปผล (Verification & Reporting)
- [ ] **ตรวจสอบความครบถ้วนของคลัง:**
  - ยืนยันว่าภาพยนตร์ทุกเรื่องเปิดดูข้อมูลและเล่นไฟล์วิดีโอได้ปกติ
  - ตรวจสอบความถูกต้องของสถิติและตัวเลขบน Web UI
  - สรุปรายงานผลการจัดระเบียบให้ผู้ใช้ทราบ
