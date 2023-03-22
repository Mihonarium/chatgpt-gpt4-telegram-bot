package main

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/PullRequestInc/go-gpt3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	GPT4Model       = "gpt-4"
	GPT35TurboModel = "gpt-3.5-turbo"
)

const DefaultModel = GPT35TurboModel
const DefaultSystemPrompt = "You are a helpful AI assistant."

var config Config

// Store conversation history per user
var conversationHistory = make(map[int64][]gpt3.ChatCompletionRequestMessage)
var userSettingsMap = make(map[int64]UserSettings)
var mu = &sync.Mutex{}

type UserSettings struct {
	Model        string
	SystemPrompt string
	State        string
}

type Config struct {
	TelegramToken string   `yaml:"telegram_token"`
	OpenAIKey     string   `yaml:"openai_api_key"`
	AllowedUsers  []string `yaml:"allowed_telegram_usernames"`
}

func ReadConfig() (Config, error) {
	var config Config
	configFile, err := os.Open("config.yml")
	if err != nil {
		return config, err
	}
	defer configFile.Close()
	decoder := yaml.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		return config, err
	}
	return config, nil
}

const (
	StateDefault                = ""
	StateWaitingForSystemPrompt = "waiting_for_system_prompt"
)

func main() {
	var err error
	config, err = ReadConfig()
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}
	// Initialize the OpenAI API client
	client := gpt3.NewClient(config.OpenAIKey)

	// Initialize the Telegram bot
	bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Listen for updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("Failed to get updates channel: %v", err)
	}

	// Handle updates
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check if the message is a command
		if update.Message.IsCommand() {
			handleCommand(bot, update, client)
		} else {
			handleMessage(bot, update, client)
		}
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, client gpt3.Client) {
	if !contains(config.AllowedUsers, update.Message.From.UserName) {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are not allowed to use this bot.")
		bot.Send(msg)
		return
	}
	mu.Lock()
	state := userSettingsMap[update.Message.Chat.ID].State
	model := userSettingsMap[update.Message.Chat.ID].Model
	if model == "" {
		model = DefaultModel
	}
	mu.Unlock()
	if state == StateWaitingForSystemPrompt {
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = UserSettings{
			Model:        model,
			SystemPrompt: update.Message.Text,
			State:        StateDefault,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "System prompt set.")
		bot.Send(msg)
		return
	}
	generatedText, err := generateTextWithGPT(client, update.Message.Text, update.Message.Chat.ID, model)
	if err != nil {
		log.Printf("Failed to generate text with GPT: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, generatedText)
	msg.ReplyToMessageID = update.Message.MessageID

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, client gpt3.Client) {
	command := update.Message.Command()
	commandArg := update.Message.CommandArguments()
	switch command {
	case "start":
		// Reset the conversation history for the user
		mu.Lock()
		conversationHistory[update.Message.Chat.ID] = []gpt3.ChatCompletionRequestMessage{
			{
				Role:    "system",
				Content: DefaultSystemPrompt,
			},
		}
		userSettingsMap[update.Message.Chat.ID] = UserSettings{
			Model:        DefaultModel,
			SystemPrompt: DefaultSystemPrompt,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome to the GPT-4 Telegram bot!")
		bot.Send(msg)
	case "new":
		// Reset the conversation history for the user
		mu.Lock()
		conversationHistory[update.Message.Chat.ID] = []gpt3.ChatCompletionRequestMessage{
			{
				Role:    "system",
				Content: userSettingsMap[update.Message.Chat.ID].SystemPrompt,
			},
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, ".")
		bot.Send(msg)
	case "gpt4":
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = UserSettings{
			Model: GPT4Model,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Switched to GPT-4 model.")
		bot.Send(msg)
	case "gpt35":
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = UserSettings{
			Model: GPT35TurboModel,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Switched to GPT-3.5-turbo model.")
		bot.Send(msg)
	case "retry":
		// Retry the last message
		mu.Lock()
		lastMessage := conversationHistory[update.Message.Chat.ID][len(conversationHistory[update.Message.Chat.ID])-2]
		conversationHistory[update.Message.Chat.ID] = conversationHistory[update.Message.Chat.ID][:len(conversationHistory[update.Message.Chat.ID])-2]
		model := userSettingsMap[update.Message.Chat.ID].Model
		if model == "" {
			model = DefaultModel
		}
		mu.Unlock()
		generatedText, err := generateTextWithGPT(client, lastMessage.Content, update.Message.Chat.ID, model)
		if err != nil {
			log.Printf("Failed to generate text with GPT: %v", err)
			return
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, generatedText)
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
	case "system_prompt":
		if commandArg == "" {
			mu.Lock()
			userSettingsMap[update.Message.Chat.ID] = UserSettings{
				Model:        userSettingsMap[update.Message.Chat.ID].Model,
				SystemPrompt: userSettingsMap[update.Message.Chat.ID].SystemPrompt,
				State:        StateWaitingForSystemPrompt,
			}
			mu.Unlock()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide a system prompt.")
			bot.Send(msg)
			return
		}
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = UserSettings{
			Model:        userSettingsMap[update.Message.Chat.ID].Model,
			SystemPrompt: commandArg,
		}
		conversationHistory[update.Message.Chat.ID] = []gpt3.ChatCompletionRequestMessage{
			{
				Role:    "system",
				Content: commandArg,
			},
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("System prompt set: %s", commandArg))
		bot.Send(msg)
	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Unknown command: %s", command))
		bot.Send(msg)
	}
}

func generateTextWithGPT(client gpt3.Client, inputText string, chatID int64, model string) (string, error) {
	// Add the user's message to the conversation history
	conversationHistory[chatID] = append(conversationHistory[chatID], gpt3.ChatCompletionRequestMessage{
		Role:    "user",
		Content: inputText,
	})

	temp := float32(0.7)
	request := gpt3.ChatCompletionRequest{
		Model:       model,
		Messages:    conversationHistory[chatID],
		Temperature: &temp,
		MaxTokens:   3000,
		TopP:        1,
	}
	ctx := context.Background()

	// Call the OpenAI API
	response, err := client.ChatCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}

	// Get the generated text
	generatedText := response.Choices[0].Message.Content
	generatedText = strings.TrimSpace(generatedText)

	// Add the AI's response to the conversation history
	conversationHistory[chatID] = append(conversationHistory[chatID], gpt3.ChatCompletionRequestMessage{
		Role:    "assistant",
		Content: generatedText,
	})

	return generatedText, nil
}
