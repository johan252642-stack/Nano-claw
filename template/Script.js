// Load config saat halaman dimuat
async function loadConfig() {
    const res = await fetch('/api/config');
    const cfg = await res.json();
    document.getElementById('telegramEnabled').checked = cfg.telegram_enabled;
    document.getElementById('telegramToken').value = cfg.telegram_bot_token;
    document.getElementById('allowedUsers').value = (cfg.allowed_telegram_users || []).join(',');
    document.getElementById('whatsappEnabled').checked = cfg.whatsapp_enabled;
    document.getElementById('whatsappToken').value = cfg.whatsapp_token;
    document.getElementById('whatsappPhoneID').value = cfg.whatsapp_phone_id;
    document.getElementById('openrouterKey').value = cfg.openrouter_key;
    document.getElementById('geminiKey').value = cfg.gemini_key;
    document.getElementById('groqKey').value = cfg.groq_key;
    document.getElementById('deepseekKey').value = cfg.deepseek_key;
    document.getElementById('openaiKey').value = cfg.openai_key;
    document.getElementById('ollamaUrl').value = cfg.ollama_url;
    document.getElementById('defaultModel').value = cfg.selected_model;
    // Load Ollama models
    loadOllamaModels();
}

async function loadOllamaModels() {
    const res = await fetch('/api/ollama/models');
    const models = await res.json();
    const select = document.getElementById('ollamaModelSelect');
    select.innerHTML = models.map(m => `<option value="${m}">${m}</option>`).join('');
    if (models.length > 0) {
        // Set current selected ollama model from config
        const cfgRes = await fetch('/api/config');
        const cfg = await cfgRes.json();
        if (cfg.ollama_model) select.value = cfg.ollama_model;
    }
}

// Simpan config
document.getElementById('settingsForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const allowedUsers = document.getElementById('allowedUsers').value.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n));
    const newConfig = {
        telegram_bot_token: document.getElementById('telegramToken').value,
        telegram_enabled: document.getElementById('telegramEnabled').checked,
        whatsapp_token: document.getElementById('whatsappToken').value,
        whatsapp_phone_id: document.getElementById('whatsappPhoneID').value,
        whatsapp_enabled: document.getElementById('whatsappEnabled').checked,
        openrouter_key: document.getElementById('openrouterKey').value,
        gemini_key: document.getElementById('geminiKey').value,
        groq_key: document.getElementById('groqKey').value,
        deepseek_key: document.getElementById('deepseekKey').value,
        openai_key: document.getElementById('openaiKey').value,
        ollama_url: document.getElementById('ollamaUrl').value,
        ollama_model: document.getElementById('ollamaModelSelect').value,
        selected_model: document.getElementById('defaultModel').value,
        allowed_telegram_users: allowedUsers
    };
    await fetch('/api/config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(newConfig)
    });
    alert('Pengaturan disimpan! Bot Telegram akan restart otomatis.');
    // Reload model select di chat
    loadModelSelect();
});

// Pull model Ollama
document.getElementById('pullModelBtn').addEventListener('click', async () => {
    const newModel = document.getElementById('newOllamaModel').value;
    if (!newModel) return;
    await fetch('/api/ollama/pull', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({model: newModel})
    });
    alert(`Download model ${newModel} dimulai. Proses bisa memakan waktu.`);
    setTimeout(loadOllamaModels, 2000);
});

// Model select di tab chat
async function loadModelSelect() {
    const res = await fetch('/api/config');
    const cfg = await res.json();
    const select = document.getElementById('modelSelect');
    const options = ['local', 'openrouter', 'groq', 'deepseek', 'openai', 'gemini'];
    select.innerHTML = options.map(opt => `<option value="${opt}" ${cfg.selected_model === opt ? 'selected' : ''}>${opt}</option>`).join('');
}
loadModelSelect();

// Chat functionality
const chatBox = document.getElementById('chatBox');
const userInput = document.getElementById('userInput');
const sendBtn = document.getElementById('sendBtn');
let currentModel = '';

async function sendMessage() {
    const msg = userInput.value.trim();
    if (!msg) return;
    addMessage(msg, 'user');
    userInput.value = '';
    addMessage('🔄', 'bot');
    const model = document.getElementById('modelSelect').value;
    const res = await fetch('/api/chat', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({message: msg, model: model})
    });
    const data = await res.json();
    // hapus loading
    chatBox.removeChild(chatBox.lastChild);
    if (data.error) {
        addMessage('Error: '+data.error, 'bot');
    } else {
        addMessage(data.reply, 'bot');
    }
}

function addMessage(text, sender) {
    const div = document.createElement('div');
    div.className = `message ${sender}`;
    div.textContent = text;
    chatBox.appendChild(div);
    chatBox.scrollTop = chatBox.scrollHeight;
}

sendBtn.addEventListener('click', sendMessage);
userInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
    }
});

// Tab switching
document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        const tab = btn.dataset.tab;
        document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        document.querySelectorAll('.tab-content').forEach(cont => cont.classList.remove('active'));
        document.getElementById(`${tab}-tab`).classList.add('active');
        if (tab === 'settings') loadConfig();
    });
});

// Initial load
loadConfig();
