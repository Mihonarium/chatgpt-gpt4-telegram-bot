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
	tokenizer "github.com/samber/go-gpt-3-encoder"
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
var userSettingsMap = make(map[int64]User)
var mu = &sync.Mutex{}

type User struct {
	Model                string
	SystemPrompt         string
	State                string
	CurrentContext       *context.CancelFunc
	CurrentMessageBuffer string
}

type Config struct {
	TelegramToken string   `yaml:"telegram_token"`
	OpenAIKey     string   `yaml:"openai_api_key"`
	AllowedUsers  []string `yaml:"allowed_telegram_usernames"`
}

func ReadConfig() (Config, error) {
	var config Config
	configFile, err := os.Open("gpt4_bot_config.yml")
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
		userSettingsMap[update.Message.Chat.ID] = User{
			Model:        model,
			SystemPrompt: update.Message.Text,
			State:        StateDefault,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "System prompt set.")
		bot.Send(msg)
		return
	}
	/*generatedText, err := generateTextWithGPT(client, update.Message.Text, update.Message.Chat.ID, model)
	if err != nil {
		log.Printf("Failed to generate text with GPT: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, generatedText)
	msg.ReplyToMessageID = update.Message.MessageID

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
	}*/
	generatedTextStream, err := generateTextStreamWithGPT(client, update.Message.Text, update.Message.Chat.ID, model)
	if err != nil {
		log.Printf("Failed to generate text stream with GPT: %v", err)
		return
	}
	text := ""
	messageID := 0
	for generatedText := range generatedTextStream {
		if strings.TrimSpace(generatedText) == "" {
			continue
		}
		if text == "" {
			// Send the first message
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, generatedText+"...")
			msg.ReplyToMessageID = update.Message.MessageID
			msg_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Failed to send message: %v", err)
			}
			messageID = msg_.MessageID
			fmt.Println("Message ID: ", msg_.MessageID)
			text += generatedText
			continue
		}
		text += generatedText
		// if the length of the text is too long, send a new message
		if len(text) > 4096 {
			text = generatedText
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
			msg.ReplyToMessageID = messageID
			msg_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Failed to send message: %v", err)
			}
			messageID = msg_.MessageID
			continue
		}
		// Edit the message
		msg := tgbotapi.NewEditMessageText(update.Message.Chat.ID, messageID, text+"...")
		_, err := bot.Send(msg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
		}
	}
	msg := tgbotapi.NewEditMessageText(update.Message.Chat.ID, messageID, text)
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Failed to edit message: %v", err)
	}
	CompleteResponse(update.Message.Chat.ID)
}

var helpMessage = "Available commands are:\n" +
	"  /new: Start a new conversation\n" +
	"  /retry: Regenerate last bot answer in case of any error\n" +
	"  /gpt4: Switch to model GPT4\n" +
	"  /gpt35: Switch to GPT-3.5-turbo\n" +
	"  /system_prompt: Set the system prompt\n"

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, client gpt3.Client) {
	command := update.Message.Command()
	commandArg := update.Message.CommandArguments()
	switch command {
	case "help":
		// Send help message
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpMessage)
		bot.Send(msg)
	case "start":
		// Reset the conversation history for the user
		mu.Lock()
		conversationHistory[update.Message.Chat.ID] = []gpt3.ChatCompletionRequestMessage{
			{
				Role:    "system",
				Content: DefaultSystemPrompt,
			},
		}
		userSettingsMap[update.Message.Chat.ID] = User{
			Model:        DefaultModel,
			SystemPrompt: DefaultSystemPrompt,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome to the GPT-4 Telegram bot!\n"+helpMessage)
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
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Conversation history cleared.")
		bot.Send(msg)
	case "gpt4":
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = User{
			Model: GPT4Model,
		}
		mu.Unlock()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Switched to GPT-4 model.")
		bot.Send(msg)
	case "gpt35":
		mu.Lock()
		userSettingsMap[update.Message.Chat.ID] = User{
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
	case "stop":
		mu.Lock()
		user := userSettingsMap[update.Message.Chat.ID]
		if user.CurrentContext != nil {
			CompleteResponse(update.Message.Chat.ID)
		}
		mu.Unlock()
	case "system_prompt":
		if commandArg == "" {
			mu.Lock()
			userSettingsMap[update.Message.Chat.ID] = User{
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
		userSettingsMap[update.Message.Chat.ID] = User{
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
	maxTokens := 4096
	if model == GPT4Model {
		maxTokens = 8192
	}
	e, err := tokenizer.NewEncoder()
	if err != nil {
		return "", fmt.Errorf("failed to create encoder: %w", err)
	}
	totalTokens := 0
	for _, message := range conversationHistory[chatID] {
		q, err := e.Encode(message.Content)
		if err != nil {
			return "", fmt.Errorf("failed to encode message: %w", err)
		}
		totalTokens += len(q)
		q, err = e.Encode(message.Role)
		if err != nil {
			return "", fmt.Errorf("failed to encode message: %w", err)
		}
		totalTokens += len(q)
	}
	maxTokens -= totalTokens
	request := gpt3.ChatCompletionRequest{
		Model:       model,
		Messages:    conversationHistory[chatID],
		Temperature: &temp,
		MaxTokens:   maxTokens,
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

func generateTextStreamWithGPT(client gpt3.Client, inputText string, chatID int64, model string) (chan string, error) {
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
	ctx, cancel := context.WithCancel(context.Background())
	mu.Lock()
	user := userSettingsMap[chatID]
	user.CurrentContext = &cancel
	mu.Unlock()
	response := make(chan string)
	// Call the OpenAI API
	go func() {
		err := client.ChatCompletionStream(ctx, request, func(completion *gpt3.ChatCompletionStreamResponse) {
			log.Printf("Received completion: %v\n", completion)
			response <- completion.Choices[0].Delta.Content
			mu.Lock()
			user := userSettingsMap[chatID]
			user.CurrentMessageBuffer += completion.Choices[0].Delta.Content
			userSettingsMap[chatID] = user
			mu.Unlock()
			if completion.Choices[0].FinishReason != "" {
				close(response)
				CompleteResponse(chatID)
			}
		})
		if err != nil {
			// if response open, close it
			if _, ok := <-response; ok {
				response <- "failed to call OpenAI API"
				close(response)
			}
			// return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
		}
	}()

	return response, nil
}

func CompleteResponse(chatID int64) {
	mu.Lock()
	user := userSettingsMap[chatID]
	if user.CurrentContext == nil {
		mu.Unlock()
		return
	}
	(*user.CurrentContext)()
	user.CurrentContext = nil
	generatedText := user.CurrentMessageBuffer
	user.CurrentMessageBuffer = ""
	userSettingsMap[chatID] = user
	mu.Unlock()

	// Get the generated text
	generatedText = strings.TrimSpace(generatedText)

	// Add the AI's response to the conversation history
	conversationHistory[chatID] = append(conversationHistory[chatID], gpt3.ChatCompletionRequestMessage{
		Role:    "assistant",
		Content: generatedText,
	})
}
