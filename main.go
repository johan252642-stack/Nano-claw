package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/generativeai"
	"google.golang.org/api/option"
)

// ==================================================================
// 1. Konfigurasi
// ==================================================================

type Config struct {
	// Provider aktif
	ActiveProvider string `json:"active_provider"` // openai, gemini, groq, deepseek, openrouter, ollama

	// API Keys
	OpenAIKey     string `json:"openai_key"`
	GeminiKey     string `json:"gemini_key"`
	GroqKey       string `json:"groq_key"`
	DeepSeekKey   string `json:"deepseek_key"`
	OpenRouterKey string `json:"openrouter_key"`

	// OpenRouter model (bisa diubah kapan saja)
	OpenRouterModel string `json:"openrouter_model"` // default "openai/gpt-3.5-turbo"

	// Ollama
	OllamaURL       string `json:"ollama_url"`
	OllamaModel     string `json:"ollama_model"`

	// Telegram
	TelegramToken      string  `json:"telegram_token"`
	TelegramEnabled    bool    `json:"telegram_enabled"`
	AllowedTelegramIDs []int64 `json:"allowed_telegram_ids"` // hanya user ID ini yang bisa pakai bot

	// Workspace & Prompt
	WorkspaceDir string `json:"workspace_dir"`
	SystemPrompt string `json:"system_prompt"`
}

var config Config
var configPath = "config.json"

func loadConfig() {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Default config
		config = Config{
			ActiveProvider:   "openrouter",
			OpenRouterModel:  "openai/gpt-3.5-turbo",
			OllamaURL:        "http://localhost:11434",
			OllamaModel:      "llama3",
			WorkspaceDir:     "./workspace",
			SystemPrompt:     "Kamu adalah asisten AI yang membantu. Kamu bisa menjalankan perintah terminal, membaca/menulis file, dan mencari web. Berikan jawaban singkat dan tepat.",
			TelegramEnabled:  false,
			AllowedTelegramIDs: []int64{},
		}
		saveConfig()
		return
	}
	json.Unmarshal(data, &config)
}

func saveConfig() {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

// ==================================================================
// 2. AI Provider Interface & Implementasi
// ==================================================================

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMProvider interface {
	Chat(messages []Message) (string, error)
	Name() string
}

// Provider OpenAI (digunakan juga untuk Groq, DeepSeek, OpenRouter karena kompatibel)
type OpenAICompatibleProvider struct {
	apiKey  string
	baseURL string
	model   string
	name    string
}

func (p *OpenAICompatibleProvider) Chat(messages []Message) (string, error) {
	url := p.baseURL + "/chat/completions"
	reqBody := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
	}
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response error: %v", err)
	}
	if result.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response from %s", p.name)
}
func (p *OpenAICompatibleProvider) Name() string { return p.name }

// Gemini Provider (menggunakan library google.golang.org/api)
type GeminiProvider struct {
	client *generativeai.Client
	model  string
}

func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	ctx := context.Background()
	client, err := generativeai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiProvider{client: client, model: model}, nil
}

func (p *GeminiProvider) Chat(messages []Message) (string, error) {
	// Ambil system prompt dan user message terakhir
	var system string
	var userMsg string
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else if m.Role == "user" {
			userMsg = m.Content
		}
	}
	ctx := context.Background()
	genModel := p.client.GenerativeModel(p.model)
	if system != "" {
		genModel.SystemInstruction = &generativeai.Content{
			Parts: []generativeai.Part{generativeai.Text(system)},
		}
	}
	resp, err := genModel.GenerateContent(ctx, generativeai.Text(userMsg))
	if err != nil {
		return "", err
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%s", resp.Candidates[0].Content.Parts[0]), nil
	}
	return "", fmt.Errorf("no response from Gemini")
}
func (p *GeminiProvider) Name() string { return "gemini" }

// Ollama Provider (local)
type OllamaProvider struct {
	baseURL string
	model   string
}

func (p *OllamaProvider) Chat(messages []Message) (string, error) {
	var prompt strings.Builder
	for _, m := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}
	prompt.WriteString("assistant: ")
	reqBody := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt.String(),
		"stream": false,
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(p.baseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Response string `json:"response"`
	}
	json.Unmarshal(body, &result)
	return result.Response, nil
}
func (p *OllamaProvider) Name() string { return "ollama" }

func getProvider() (LLMProvider, error) {
	switch config.ActiveProvider {
	case "openai":
		if config.OpenAIKey == "" {
			return nil, fmt.Errorf("OpenAI API key missing")
		}
		return &OpenAICompatibleProvider{
			apiKey:  config.OpenAIKey,
			baseURL: "https://api.openai.com/v1",
			model:   "gpt-3.5-turbo",
			name:    "OpenAI",
		}, nil
	case "groq":
		if config.GroqKey == "" {
			return nil, fmt.Errorf("Groq API key missing")
		}
		return &OpenAICompatibleProvider{
			apiKey:  config.GroqKey,
			baseURL: "https://api.groq.com/openai/v1",
			model:   "llama3-70b-8192",
			name:    "Groq",
		}, nil
	case "deepseek":
		if config.DeepSeekKey == "" {
			return nil, fmt.Errorf("DeepSeek API key missing")
		}
		return &OpenAICompatibleProvider{
			apiKey:  config.DeepSeekKey,
			baseURL: "https://api.deepseek.com/v1",
			model:   "deepseek-chat",
			name:    "DeepSeek",
		}, nil
	case "openrouter":
		if config.OpenRouterKey == "" {
			return nil, fmt.Errorf("OpenRouter API key missing")
		}
		return &OpenAICompatibleProvider{
			apiKey:  config.OpenRouterKey,
			baseURL: "https://openrouter.ai/api/v1",
			model:   config.OpenRouterModel, // bisa diubah kapan saja
			name:    "OpenRouter",
		}, nil
	case "gemini":
		if config.GeminiKey == "" {
			return nil, fmt.Errorf("Gemini API key missing")
		}
		return NewGeminiProvider(config.GeminiKey, "gemini-pro")
	case "ollama":
		return &OllamaProvider{baseURL: config.OllamaURL, model: config.OllamaModel}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", config.ActiveProvider)
	}
}

// ==================================================================
// 3. Tools (Command Execution, File I/O, Web Search)
// ==================================================================

func executeCommand(cmdStr string) (string, error) {
	fmt.Printf("\n⚠️  AI ingin menjalankan perintah: %s\n", cmdStr)
	fmt.Print("Izinkan? (y/n): ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		return "", fmt.Errorf("perintah ditolak user")
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func readFile(filename string) (string, error) {
	fullPath := filepath.Join(config.WorkspaceDir, filename)
	data, err := os.ReadFile(fullPath)
	return string(data), err
}

func writeFile(filename, content string) error {
	fullPath := filepath.Join(config.WorkspaceDir, filename)
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func listDir(path string) (string, error) {
	fullPath := filepath.Join(config.WorkspaceDir, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, "\n"), nil
}

func webSearch(query string) (string, error) {
	// Placeholder: integrasi dengan Brave Search atau DuckDuckGo
	// Untuk demo, kita kembalikan pesan
	return fmt.Sprintf("Hasil pencarian untuk '%s' (integrasi web search butuh API key)", query), nil
}

func webFetch(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// ==================================================================
// 4. Sub-Agent Sederhana
// ==================================================================

type SubAgent struct {
	Name        string
	Description string
	Process     func(input string) (string, error)
}

var subAgents = []SubAgent{
	{
		Name:        "calculator",
		Description: "Menghitung ekspresi matematika sederhana",
		Process: func(input string) (string, error) {
			// Evaluasi ekspresi matematika (sangat sederhana)
			// Dalam production gunakan library seperti govaluate
			return fmt.Sprintf("Hasil dari %s = (contoh: implementasi eval)", input), nil
		},
	},
	{
		Name:        "reverse",
		Description: "Membalik string",
		Process: func(input string) (string, error) {
			runes := []rune(input)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return string(runes), nil
		},
	},
}

func callSubAgent(agentName, input string) (string, error) {
	for _, a := range subAgents {
		if a.Name == agentName {
			return a.Process(input)
		}
	}
	return "", fmt.Errorf("sub-agent '%s' tidak ditemukan", agentName)
}

// ==================================================================
// 5. Cron Scheduler (Placeholder)
// ==================================================================

type CronScheduler struct {
	// Placeholder
}

func (c *CronScheduler) AddJob(schedule string, cmd func()) {
	fmt.Printf("Menambahkan job dengan schedule %s (integrasi cron belum selesai)\n", schedule)
	// TODO: implement with robfig/cron
}

// ==================================================================
// 6. MCP (Model Context Protocol) Placeholder
// ==================================================================

type MCPClient struct {
	ServerURL string
}

func (m *MCPClient) CallTool(toolName string, args map[string]interface{}) (string, error) {
	return fmt.Sprintf("Memanggil MCP tool '%s' dengan args %v", toolName, args), nil
}

// ==================================================================
// 7. Telegram Bot (dengan whitelist user ID)
// ==================================================================

var telegramBot *telego.Bot

func startTelegramBot() {
	if !config.TelegramEnabled || config.TelegramToken == "" {
		fmt.Println("Telegram bot tidak diaktifkan atau token kosong.")
		return
	}
	bot, err := telego.NewBot(config.TelegramToken)
	if err != nil {
		log.Printf("Gagal membuat bot Telegram: %v", err)
		return
	}
	telegramBot = bot
	updates, _ := bot.UpdatesViaLongPolling(nil)
	go func() {
		for update := range updates {
			if update.Message != nil && update.Message.Text != nil {
				userID := update.Message.From.ID
				// Cek whitelist
				allowed := len(config.AllowedTelegramIDs) == 0 // jika kosong, semua diizinkan
				for _, uid := range config.AllowedTelegramIDs {
					if uid == userID {
						allowed = true
						break
					}
				}
				if !allowed {
					bot.SendMessage(telegoutil.Message(update.Message.Chat.ID, "Maaf, kamu tidak diizinkan menggunakan bot ini."))
					continue
				}
				// Proses chat dengan AI
				go handleTelegramMessage(bot, update.Message)
			}
		}
	}()
	fmt.Println("✅ Telegram bot aktif")
}

func handleTelegramMessage(bot *telego.Bot, message *telego.Message) {
	provider, err := getProvider()
	if err != nil {
		bot.SendMessage(telegoutil.Message(message.Chat.ID, "Error: "+err.Error()))
		return
	}
	messages := []Message{
		{Role: "system", Content: config.SystemPrompt},
		{Role: "user", Content: message.Text},
	}
	reply, err := provider.Chat(messages)
	if err != nil {
		reply = "Error: " + err.Error()
	}
	// Batasi panjang pesan (Telegram max 4096)
	if len(reply) > 4000 {
		reply = reply[:4000] + "..."
	}
	bot.SendMessage(telegoutil.Message(message.Chat.ID, reply))
}

// ==================================================================
// 8. CLI Chat Interaktif
// ==================================================================

func chatCLI() {
	provider, err := getProvider()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("\n🤖 Menggunakan provider: %s\n", provider.Name())
	fmt.Println("💬 Mulai chat. Ketik 'exit' untuk keluar, '!cmd <perintah>' untuk eksekusi langsung, '!file read/write/list', '!subagent <nama> <input>'")
	scanner := bufio.NewScanner(os.Stdin)
	messages := []Message{
		{Role: "system", Content: config.SystemPrompt},
	}
	for {
		fmt.Print("\n🧑 You: ")
		scanner.Scan()
		input := scanner.Text()
		if input == "exit" {
			break
		}
		// Handle perintah langsung
		if strings.HasPrefix(input, "!cmd ") {
			cmd := strings.TrimPrefix(input, "!cmd ")
			out, _ := executeCommand(cmd)
			fmt.Printf("Output:\n%s\n", out)
			continue
		}
		if strings.HasPrefix(input, "!file read ") {
			fname := strings.TrimPrefix(input, "!file read ")
			content, err := readFile(fname)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Isi file:\n%s\n", content)
			}
			continue
		}
		if strings.HasPrefix(input, "!file write ") {
			parts := strings.SplitN(input, " ", 3)
			if len(parts) < 3 {
				fmt.Println("Format: !file write <nama> <konten>")
				continue
			}
			err := writeFile(parts[2], parts[3])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("File berhasil ditulis.")
			}
			continue
		}
		if strings.HasPrefix(input, "!file list") {
			path := "."
			if strings.HasPrefix(input, "!file list ") {
				path = strings.TrimPrefix(input, "!file list ")
			}
			list, err := listDir(path)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Daftar file:\n%s\n", list)
			}
			continue
		}
		if strings.HasPrefix(input, "!subagent ") {
			parts := strings.SplitN(input, " ", 3)
			if len(parts) < 3 {
				fmt.Println("Format: !subagent <nama> <input>")
				continue
			}
			out, err := callSubAgent(parts[1], parts[2])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("Hasil sub-agent: %s\n", out)
			}
			continue
		}
		if strings.HasPrefix(input, "!websearch ") {
			query := strings.TrimPrefix(input, "!websearch ")
			out, err := webSearch(query)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("%s\n", out)
			}
			continue
		}
		// Chat normal
		messages = append(messages, Message{Role: "user", Content: input})
		fmt.Print("🤖 AI: ")
		reply, err := provider.Chat(messages)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		// Deteksi perintah dari AI (format !cmd)
		if strings.Contains(reply, "!cmd ") {
			lines := strings.Split(reply, "\n")
			var newReply strings.Builder
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "!cmd") {
					cmd := strings.TrimPrefix(strings.TrimSpace(line), "!cmd")
					cmd = strings.TrimSpace(cmd)
					out, err := executeCommand(cmd)
					if err != nil {
						newReply.WriteString(fmt.Sprintf("\n[Error eksekusi: %v]\n", err))
					} else {
						newReply.WriteString(fmt.Sprintf("\n[Hasil eksekusi]:\n%s\n", out))
					}
				} else {
					newReply.WriteString(line + "\n")
				}
			}
			reply = newReply.String()
		}
		fmt.Println(reply)
		messages = append(messages, Message{Role: "assistant", Content: reply})
		// Batasi history
		if len(messages) > 20 {
			messages = messages[2:]
		}
	}
}

// ==================================================================
// 9. Menu Konfigurasi
// ==================================================================

func configMenu() {
	for {
		fmt.Println("\n=== Konfigurasi Nano-claw ===")
		fmt.Printf("1. Pilih AI Provider (sekarang: %s)\n", config.ActiveProvider)
		fmt.Printf("2. Ganti OpenRouter model (sekarang: %s)\n", config.OpenRouterModel)
		fmt.Printf("3. Input / ubah API Keys\n")
		fmt.Printf("4. Pengaturan Ollama (URL & model)\n")
		fmt.Printf("5. Pengaturan Telegram (token, enable, whitelist user ID)\n")
		fmt.Printf("6. Edit System Prompt\n")
		fmt.Printf("7. Lihat konfigurasi saat ini\n")
		fmt.Printf("0. Kembali ke menu utama\n")
		fmt.Print("Pilihan: ")
		var opt int
		fmt.Scanln(&opt)
		switch opt {
		case 1:
			fmt.Println("Provider yang tersedia: openai, gemini, groq, deepseek, openrouter, ollama")
			fmt.Print("Masukkan nama provider: ")
			fmt.Scanln(&config.ActiveProvider)
			saveConfig()
			fmt.Println("✅ Provider diubah.")
		case 2:
			fmt.Print("Masukkan model OpenRouter (contoh: openai/gpt-4, google/gemini-pro, deepseek/deepseek-chat): ")
			fmt.Scanln(&config.OpenRouterModel)
			saveConfig()
			fmt.Println("✅ Model OpenRouter diubah.")
		case 3:
			fmt.Print("OpenAI Key: ")
			fmt.Scanln(&config.OpenAIKey)
			fmt.Print("Gemini Key: ")
			fmt.Scanln(&config.GeminiKey)
			fmt.Print("Groq Key: ")
			fmt.Scanln(&config.GroqKey)
			fmt.Print("DeepSeek Key: ")
			fmt.Scanln(&config.DeepSeekKey)
			fmt.Print("OpenRouter Key: ")
			fmt.Scanln(&config.OpenRouterKey)
			saveConfig()
			fmt.Println("✅ API keys disimpan.")
		case 4:
			fmt.Printf("Ollama URL (default http://localhost:11434): ")
			fmt.Scanln(&config.OllamaURL)
			fmt.Printf("Ollama model (default llama3): ")
			fmt.Scanln(&config.OllamaModel)
			saveConfig()
			fmt.Println("✅ Pengaturan Ollama disimpan.")
		case 5:
			fmt.Printf("Telegram Bot Token: ")
			fmt.Scanln(&config.TelegramToken)
			fmt.Printf("Aktifkan Telegram bot? (true/false): ")
			var enable bool
			fmt.Scanln(&enable)
			config.TelegramEnabled = enable
			fmt.Println("Masukkan user ID yang diizinkan (pisahkan dengan koma, contoh: 123456789,987654321)")
			fmt.Print("User IDs: ")
			var idsStr string
			fmt.Scanln(&idsStr)
			if idsStr != "" {
				parts := strings.Split(idsStr, ",")
				var ids []int64
				for _, p := range parts {
					id, _ := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
					if id != 0 {
						ids = append(ids, id)
					}
				}
				config.AllowedTelegramIDs = ids
			} else {
				config.AllowedTelegramIDs = []int64{}
			}
			saveConfig()
			fmt.Println("✅ Pengaturan Telegram disimpan. Restart bot jika perlu.")
			// restart bot sederhana (panggil ulang startTelegramBot)
			go startTelegramBot()
		case 6:
			fmt.Println("System prompt saat ini:")
			fmt.Println(config.SystemPrompt)
			fmt.Println("Masukkan system prompt baru (ketik 'done' di baris baru):")
			scanner := bufio.NewScanner(os.Stdin)
			var newPrompt strings.Builder
			for scanner.Scan() {
				line := scanner.Text()
				if line == "done" {
					break
				}
				newPrompt.WriteString(line + "\n")
			}
			config.SystemPrompt = newPrompt.String()
			saveConfig()
			fmt.Println("✅ System prompt disimpan.")
		case 7:
			fmt.Printf("Active Provider: %s\n", config.ActiveProvider)
			fmt.Printf("OpenRouter Model: %s\n", config.OpenRouterModel)
			fmt.Printf("Telegram Enabled: %v, Allowed IDs: %v\n", config.TelegramEnabled, config.AllowedTelegramIDs)
			fmt.Printf("Workspace: %s\n", config.WorkspaceDir)
		case 0:
			return
		}
	}
}

// ==================================================================
// 10. Main
// ==================================================================

func main() {
	// Load .env (opsional)
	godotenv.Load()
	loadConfig()
	// Buat workspace
	os.MkdirAll(config.WorkspaceDir, 0755)

	// Start Telegram bot jika diaktifkan
	if config.TelegramEnabled && config.TelegramToken != "" {
		go startTelegramBot()
	}

	fmt.Println("🦞 Nano-claw AI Assistant (Setara PicoClaw)")

	for {
		fmt.Println("\n╔════════════════════════════════╗")
		fmt.Println("║  1. Chat dengan AI              ║")
		fmt.Println("║  2. Konfigurasi                 ║")
		fmt.Println("║  0. Keluar                      ║")
		fmt.Println("╚════════════════════════════════╝")
		fmt.Print("Pilihan: ")
		var choice int
		fmt.Scanln(&choice)
		switch choice {
		case 1:
			chatCLI()
		case 2:
			configMenu()
		case 0:
			fmt.Println("Sampai jumpa!")
			return
		default:
			fmt.Println("Pilihan tidak valid")
		}
	}
}
