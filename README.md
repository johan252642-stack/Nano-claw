# 🦞 Nano-claw — AI Assistant Ringan dengan Web UI

Nano-claw adalah asisten AI modular yang mendukung berbagai model (OpenAI, Gemini, Groq, DeepSeek, OpenRouter, dan Ollama lokal) serta dapat terhubung ke Telegram dan WhatsApp. Semua pengaturan dilakukan melalui antarmuka web.

## 📦 Prasyarat

- **Go** (versi 1.21 atau lebih baru) – panduan instalasi di bawah
- **Git** (opsional, untuk clone repositori)
- **Ollama** (hanya jika ingin menggunakan model AI lokal)

---

## 🚀 Instalasi Go di Setiap Sistem Operasi

### 🐧 Linux (x86_64 / AMD64)

```bash
# Unduh Go 1.22.5
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# Hapus instalasi lama (jika ada)
sudo rm -rf /usr/local/go

# Ekstrak ke /usr/local
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz

# Tambahkan ke PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verifikasi
go version
```

🍎 macOS (Intel / Apple Silicon)

Untuk Intel (amd64):

```bash
wget https://go.dev/dl/go1.22.5.darwin-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.5.darwin-amd64.tar.gz
```

Untuk Apple Silicon (arm64 / M1/M2):

```bash
wget https://go.dev/dl/go1.22.5.darwin-arm64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.5.darwin-arm64.tar.gz
```

Setup PATH (keduanya):

```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc
source ~/.zshrc
go version
```

🪟 Windows

1. Kunjungi https://go.dev/dl/
2. Unduh file .msi untuk Windows (misal go1.22.5.windows-amd64.msi)
3. Jalankan installer dan ikuti petunjuk (biasanya terinstal di C:\Go)
4. Installer akan otomatis menambahkan C:\Go\bin ke PATH sistem
5. Buka Command Prompt atau PowerShell, lalu ketik:
   ```cmd
   go version
   ```

📱 Nethunter / Kali Linux (ARM64 / aarch64)

```bash
# Unduh Go untuk ARM64
wget https://go.dev/dl/go1.22.5.linux-arm64.tar.gz

# Hapus lama (jika ada)
sudo rm -rf /usr/local/go

# Ekstrak
sudo tar -C /usr/local -xzf go1.22.5.linux-arm64.tar.gz

# PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Cek
go version
```

Catatan untuk Nethunter: Jika menggunakan Termux, instal dengan pkg install golang saja.

---

📥 Clone & Jalankan Nano-claw

```bash
# Clone repositori (ganti dengan URL GitHub-mu)
git clone https://github.com/username/Nano-claw.git
cd Nano-claw

# Download dependensi Go
go mod tidy

# Jalankan server
go run main.go
```

Setelah berjalan, buka browser dan akses: http://localhost:8080

---

🧩 Fitur Singkat

· Web UI untuk input API key & pilih model
· Multi-provider: OpenAI, Gemini, Groq, DeepSeek, OpenRouter, Ollama
· Model lokal: Auto-download model Ollama dari UI
· Telegram Bot dan WhatsApp (webhook)
· Chat history di browser

---

📝 Catatan

· File konfigurasi config.json akan dibuat otomatis setelah pertama kali menjalankan.
· Jangan commit config.json atau .env ke GitHub – sudah ada di .gitignore.
· Untuk model lokal, pastikan Ollam

🔧 Troubles
Masalah Solusi
go: command not found Ikuti panduan instalasi Go di atas sesuai OS
address already in use Ganti port dengan PORT=9090 go run main.go
Ollama tidak merespon Cek ollama serve dan URL di Settings (http://localhost:11434)
Telegram bot tidak aktif Pastikan token benar, bot sudah di-star. dan user ID di izinkan
