package main

import (
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
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/generativeai"
	"google.golang.org/api/option"
)

// ---------- Konfigurasi ----------
type Config struct {
	TelegramBotToken string   `json:"telegram_bot_token"`
	TelegramEnabled  bool     `json:"telegram_enabled"`
	WhatsAppToken    string   `json:"whatsapp_token"`
	WhatsAppPhoneID  string   `json:"whatsapp_phone_id"`
	WhatsAppEnabled  bool     `json:"whatsapp_enabled"`
	OpenRouterKey    string   `json:"openrouter_key"`
	GeminiKey        string   `json:"gemini_key"`
	GroqKey          string   `json:"groq_key"`
	DeepSeekKey      string   `json:"deepseek_key"`
	OpenAIKey        string   `json:"openai_key"`
	OllamaURL        string   `json:"ollama_url"`
	OllamaModel      string   `json:"ollama_model"`
	SelectedModel    string   `json:"selected_model"` // local atau salah satu cloud
	AllowedTelegramUsers []int64 `json:"allowed_telegram_users"`
}

var config Config
var configMutex sync.RWMutex
var configPath = "config.json"

// ---------- Provider abstrak ----------
type Message struct {
	Role    string
	Content string
}

type AIProvider interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	Name() string
}

// OpenAI provider (juga untuk Groq, DeepSeek, OpenRouter via baseURL)
type OpenAIProvider struct {
	client *openai.Client
	model  string
	name   string
}

func NewOpenAIProvider(apiKey, baseURL, model, name string) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIProvider{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
		name:   name,
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	reqMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: reqMessages,
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
func (p *OpenAIProvider) Name() string { return p.name }

// Gemini provider
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

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Konversi ke format Gemini
	genModel := p.client.GenerativeModel(p.model)
	// Prompt system diambil dari pesan pertama jika role system
	var system string
	var chatMessages []Message
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			chatMessages = append(chatMessages, m)
		}
	}
	if system != "" {
		genModel.SystemInstruction = &generativeai.Content{
			Parts: []generativeai.Part{generativeai.Text(system)},
		}
	}
	// Ambil pesan user terakhir
	userMsg := ""
	for i := len(chatMessages) - 1; i >= 0; i-- {
		if chatMessages[i].Role == "user" {
			userMsg = chatMessages[i].Content
			break
		}
	}
	resp, err := genModel.GenerateContent(ctx, generativeai.Text(userMsg))
	if err != nil {
		return "", err
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%s", resp.Candidates[0].Content.Parts[0]), nil
	}
	return "", fmt.Errorf("no response")
}
func (p *GeminiProvider) Name() string { return "gemini" }

// Ollama provider (local)
type OllamaProvider struct {
	url   string
	model string
}

func NewOllamaProvider(url, model string) *OllamaProvider {
	return &OllamaProvider{url: url, model: model}
}

func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	// Prompt sederhana: gabungkan system + user
	var prompt strings.Builder
	for _, m := range messages {
		if m.Role == "system" {
			prompt.WriteString("System: " + m.Content + "\n")
		} else if m.Role == "user" {
			prompt.WriteString("User: " + m.Content + "\n")
		}
	}
	prompt.WriteString("Assistant: ")

	reqBody := map[string]interface{}{
		"model":  p.model,
		"prompt": prompt.String(),
		"stream": false,
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(p.url+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Response, nil
}
func (p *OllamaProvider) Name() string { return "ollama" }

// ---------- Load/Save config ----------
func loadConfig() {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// file tidak ada, buat default
		config = Config{
			OllamaURL:   "http://localhost:11434",
			OllamaModel: "llama3",
			SelectedModel: "local",
		}
		saveConfig()
		return
	}
	json.Unmarshal(data, &config)
}

func saveConfig() {
	configMutex.Lock()
	defer configMutex.Unlock()
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

// ---------- AI Router ----------
func getAIProvider(modelName string) (AIProvider, error) {
	switch modelName {
	case "local":
		if config.OllamaURL == "" {
			return nil, fmt.Errorf("Ollama URL not set")
		}
		return NewOllamaProvider(config.OllamaURL, config.OllamaModel), nil
	case "openrouter":
		return NewOpenAIProvider(config.OpenRouterKey, "https://openrouter.ai/api/v1", "openrouter/auto", "OpenRouter"), nil
	case "groq":
		return NewOpenAIProvider(config.GroqKey, "https://api.groq.com/openai/v1", "llama3-70b-8192", "Groq"), nil
	case "deepseek":
		return NewOpenAIProvider(config.DeepSeekKey, "https://api.deepseek.com/v1", "deepseek-chat", "DeepSeek"), nil
	case "openai":
		return NewOpenAIProvider(config.OpenAIKey, "", "gpt-3.5-turbo", "OpenAI"), nil
	case "gemini":
		return NewGeminiProvider(config.GeminiKey, "gemini-pro")
	default:
		return nil, fmt.Errorf("unknown model")
	}
}

// ---------- Ollama model management ----------
func pullOllamaModel(modelName string) error {
	// Panggil Ollama pull
	cmd := exec.Command("ollama", "pull", modelName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------- Telegram Bot ----------
var telegramBot *telego.Bot

func startTelegramBot() {
	if !config.TelegramEnabled || config.TelegramBotToken == "" {
		return
	}
	bot, err := telego.NewBot(config.TelegramBotToken)
	if err != nil {
		log.Printf("Telegram bot error: %v", err)
		return
	}
	telegramBot = bot
	updates, _ := bot.UpdatesViaLongPolling(nil)
	go func() {
		for update := range updates {
			if update.Message != nil && update.Message.Text != nil {
				userID := update.Message.From.ID
				// Cek allowed users
				allowed := len(config.AllowedTelegramUsers) == 0
				for _, uid := range config.AllowedTelegramUsers {
					if uid == userID {
						allowed = true
						break
					}
				}
				if !allowed {
					bot.SendMessage(telegoutil.Message(update.Message.Chat.ID, "Maaf, kamu tidak diizinkan."))
					continue
				}
				// Proses AI
				provider, err := getAIProvider(config.SelectedModel)
				if err != nil {
					bot.SendMessage(telegoutil.Message(update.Message.Chat.ID, "Error: "+err.Error()))
					continue
				}
				messages := []Message{
					{Role: "system", Content: "Kamu adalah asisten AI yang membantu."},
					{Role: "user", Content: update.Message.Text},
				}
				reply, err := provider.Chat(context.Background(), messages)
				if err != nil {
					reply = "Error: " + err.Error()
				}
				bot.SendMessage(telegoutil.Message(update.Message.Chat.ID, reply))
			}
		}
	}()
	log.Println("Telegram bot started")
}

// ---------- WhatsApp (simulasi sederhana) ----------
// Untuk production perlu webhook, di sini hanya placeholder
func startWhatsApp() {
	if !config.WhatsAppEnabled || config.WhatsAppToken == "" {
		return
	}
	// Implementasi WhatsApp Business API butuh webhook dan verifikasi
	// Kita bisa tambahkan endpoint /webhook/whatsapp
	log.Println("WhatsApp bot ready (webhook diperlukan)")
}

// ---------- Web UI ----------
func main() {
	// Load env
	godotenv.Load()
	// Load config
	loadConfig()
	// Start bot
	go startTelegramBot()
	go startWhatsApp()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	// Halaman utama
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// API untuk mendapatkan konfigurasi saat ini
	r.GET("/api/config", func(c *gin.Context) {
		configMutex.RLock()
		defer configMutex.RUnlock()
		c.JSON(http.StatusOK, config)
	})

	// API untuk menyimpan konfigurasi
	r.POST("/api/config", func(c *gin.Context) {
		var newConfig Config
		if err := c.BindJSON(&newConfig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configMutex.Lock()
		config = newConfig
		configMutex.Unlock()
		saveConfig()
		// Restart bot jika perlu (sederhana: restart goroutine)
		go startTelegramBot()
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API untuk daftar model lokal yang tersedia di Ollama
	r.GET("/api/ollama/models", func(c *gin.Context) {
		if config.OllamaURL == "" {
			c.JSON(http.StatusOK, []string{})
			return
		}
		resp, err := http.Get(config.OllamaURL + "/api/tags")
		if err != nil {
			c.JSON(http.StatusOK, []string{})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		json.Unmarshal(body, &result)
		names := []string{}
		for _, m := range result.Models {
			names = append(names, m.Name)
		}
		c.JSON(http.StatusOK, names)
	})

	// API untuk pull model Ollama
	r.POST("/api/ollama/pull", func(c *gin.Context) {
		var req struct {
			Model string `json:"model"`
		}
		c.BindJSON(&req)
		if req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model name required"})
			return
		}
		// Jalankan pull asynchronously
		go func() {
			pullOllamaModel(req.Model)
		}()
		c.JSON(http.StatusOK, gin.H{"status": "pulling started"})
	})

	// API chat (untuk web UI)
	r.POST("/api/chat", func(c *gin.Context) {
		var req struct {
			Message string `json:"message"`
			Model   string `json:"model"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		model := req.Model
		if model == "" {
			model = config.SelectedModel
		}
		provider, err := getAIProvider(model)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		messages := []Message{
			{Role: "system", Content: "Anda adalah asisten AI yang ramah dan membantu."},
			{Role: "user", Content: req.Message},
		}
		reply, err := provider.Chat(context.Background(), messages)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"reply": reply})
	})

	r.Run(":8080")
}
