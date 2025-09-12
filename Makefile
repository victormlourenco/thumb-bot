# Thumb Bot Makefile

.PHONY: run

# Run the bot
run:
	@echo "Starting thumb-bot..."
	@if [ -z "$$TELEGRAM_TOKEN" ]; then \
		echo "Error: TELEGRAM_TOKEN environment variable is not set"; \
		echo "Please set your bot token: export TELEGRAM_TOKEN='your_token_here'"; \
		exit 1; \
	fi
	go run main.go
