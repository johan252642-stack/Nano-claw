🚀 Instalasi Claw Assistant — Perintah
📦 Prasyarat (Pastikan sudah terinstall)

· Go (versi 1.21+): Download
· Git: Download
· Ollama (opsional, untuk model lokal): Download

---

🐧 Linux / macOS (Terminal)

Salin dan jalankan perintah berikut satu per satu:

```bash
# 1. Clone repositori
git clone https://github.com/username/claw-assistant.git
cd claw-assistant

# 2. Download dependensi Go
go mod tidy

# 3. Jalankan aplikasi
go run main.go
```

Catatan: Ganti username/claw-assistant dengan URL repositori GitHub milikmu.

Setelah berjalan, buka browser dan akses:
👉 http://localhost:8080

---

🪟 Windows (CMD atau PowerShell)

Menggunakan Command Prompt (CMD)

```cmd
git clone https://github.com/username/claw-assistant.git
cd claw-assistant
go mod tidy
go run main.go
```

Menggunakan PowerShell

```powershell
git clone https://github.com/username/claw-assistant.git
cd claw-assistant
go mod tidy
go run main.go
```

---

🐳 (Opsional) Menggunakan Docker

Jika ingin menjalankan tanpa menginstall Go:

```bash
docker build -t claw-assistant .
docker run -p 8080:8080 claw-assistant
```

File Dockerfile belum disertakan, tapi kamu bisa membuatnya sendiri.

---

✅ Setelah Aplikasi Berjalan

1. Buka http://localhost:8080 di browser.
2. Klik tab Settings.
3. Masukkan API key untuk provider yang ingin digunakan (OpenAI, Gemini, Groq, DeepSeek, OpenRouter).
4. Jika ingin model lokal, pastikan Ollama berjalan (ollama serve), lalu pilih model atau download model baru dari UI.
5. Aktifkan Telegram Bot dengan memasukkan token dan daftar user ID yang diizinkan (opsional).
6. Klik Simpan Semua Pengaturan.
7. Pindah ke tab Chat, pilih model, dan mulai ngobrol!

---

🛠️ Perintah Tambahan (Untuk Pengembang)

Membangun binary (executable)

```bash
go build -o claw-assistant
./claw-assistant   # Linux/macOS
claw-assistant.exe # Windows
```

Menjalankan dengan konfigurasi kustom

```bash
CONFIG_PATH=/path/to/config.json go run main.go
```

Menghentikan aplikasi

Tekan Ctrl + C di terminal.

---

📝 Catatan Penting

· Jangan commit file config.json atau .env ke GitHub. File .gitignore sudah disediakan.
· Untuk WhatsApp Business, diperlukan konfigurasi webhook tambahan (belum otomatis di versi ini).
· Pastikan Ollama berjalan di background jika ingin menggunakan model lokal:
  ```bash
  ollama serve
  ```

---

🔄 Update ke versi terbaru

```bash
git pull origin main
go mod tidy
go run main.go
```

---

❓ Troubleshooting

Masalah Solusi
go: command not found Install Go terlebih dahulu
address already in use Port 8080 sudah dipakai. Ganti port: PORT=9090 go run main.go
Ollama tidak merespon Pastikan ollama serve berjalan dan URL di Settings sudah benar (http://localhost:11434)
Telegram bot tidak merespon Cek token, pastikan bot sudah di-start (/start), dan user ID kamu ada di daftar allowed

---

Dengan panduan di atas, pengguna tinggal copy-paste perintah ke terminal dan langsung menjalankan Claw Assistant. Simpan file ini sebagai INSTALL.md atau gabungkan dengan README.md.
