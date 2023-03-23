# ChatGPT Telegram bot

[Try on Telegram](https://t.me/GeneralPurposeTransformerBot)

## Setup
1. Get your [OpenAI API](https://openai.com/api/) key

2. Get your Telegram bot token from [@BotFather](https://t.me/BotFather)

3. Clone the repo:
    ```bash
    git clone https://github.com/Mihonarium/chatgpt-gpt4-telegram-bot.git
    cd chatgpt-gpt4-telegram-bot
    ```

3. Edit `config.yml` to set your tokens.

4. 🔥 And now **run**:
    ```bash
    go get ./...
    go run main.go
    ```
    
    
## Commands
- /retry - Regenerate last bot answer
- /new - Start new dialog
- /gpt4 - Switch to GPT-4
- /gpt35 - Switch to GPT-3.5-turbo
- /system_prompt - Set the system prompt
