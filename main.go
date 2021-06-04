package main

import (
	"context"
	"encoding/json"
	"github.com/PullRequestInc/go-gpt3"
	tgbotapi "github.com/Syfaro/telegram-bot-api"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"strings"
)

const allowedUserId int = 27948387 // change to your Telegram user id

type PromptDesign struct {
	BeforePrompt     string
	AfterPrompt      string
	MaxTokens        int
	Temperature      float32
	TopP             float32
	PresencePenalty  float32
	FrequencyPenalty float32
	Echo             bool
	Stop             []string
	AddBeforeAnswer  string
	AddAfterAnswer   string
	Engine           string
}

func (v gpt3Client) GetResponse(prompt string, promptDesign PromptDesign) (string, error) {
	if promptDesign.Engine == "" {
		promptDesign.Engine = gpt3.DefaultEngine
	}
	resp, err := v.client.CompletionWithEngine(v.ctx, promptDesign.Engine, gpt3.CompletionRequest{
		Prompt:           []string{promptDesign.BeforePrompt + prompt + promptDesign.AfterPrompt},
		MaxTokens:        gpt3.IntPtr(promptDesign.MaxTokens),
		Stop:             promptDesign.Stop,
		Echo:             promptDesign.Echo,
		Temperature:      &promptDesign.Temperature,
		TopP:             &promptDesign.TopP,
		PresencePenalty:  promptDesign.PresencePenalty,
		FrequencyPenalty: promptDesign.FrequencyPenalty,
	})
	log.Println("Sent request")
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(resp)
	log.Println(string(b))
	return promptDesign.AddBeforeAnswer + resp.Choices[0].Text + promptDesign.AddAfterAnswer, nil
}

func (v gpt3Client) Handler(promptDesign PromptDesign) func(string) (string, error) {
	return func(prompt string) (string, error) {
		return v.GetResponse(prompt, promptDesign)
	}
}

type gpt3Client struct {
	client gpt3.Client
	ctx    context.Context
}

func execCommand(handler func(string) (string, error), input string) string {
	var reply string
	log.Println("Q:", input)
	answ, err := handler(input)
	if err != nil {
		reply = "Error: " + err.Error()
	} else {
		reply = answ
	}
	log.Println("A:", reply)
	return reply
}

func (v gpt3Client) botCommand(command, input string) string {
	if d, exists := Designs[command]; exists {
		return execCommand(v.Handler(d), input)
	}
	help := "/q - ask a question\n/qf - ask a question, but the answer probably won't be made up\n" +
		"/c - convert sentence to the correct English\n/marv - use a bot reluctantly giving answers\n" +
		"/analogy - explain an analogy\n/bash - convert text to bash command"
	switch command {
	case "/help":
		return help
	case "":
		return "Please specify the command\n\n" + help
	default:
		return "Unknown command. Help: /help"
	}
}

func main() {
	godotenv.Load()

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatalln("Missing API KEY")
	}
	botToken := os.Getenv("BOT_TOKEN")
	if apiKey == "" {
		log.Fatalln("Missing Bot Token")
	}

	client := gpt3Client{gpt3.NewClient(apiKey), context.Background()}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	//bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}
		if update.Message.From.ID != allowedUserId {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Sorry, you're "+strconv.Itoa(update.Message.From.ID))
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			continue
		}

		// No GPT-3 above this line!

		text := update.Message.Text

		var command, reply string

		for c := range Designs {
			if text == c || strings.HasPrefix(text, c+" ") {
				command = c
				break
			}
		}
		if text == "/help" || strings.HasPrefix(text, "/help ") {
			command = "/help"
			break
		}
		reply = strings.TrimSpace(client.botCommand(command, strings.TrimSpace(strings.TrimPrefix(text, command))))

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		msg.ReplyToMessageID = update.Message.MessageID

		bot.Send(msg)
	}
}
