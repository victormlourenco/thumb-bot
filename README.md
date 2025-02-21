# thumb-bot

Telegram bot that extracts and sends media from Twitter, Instagram, and Vocaroo.

## Prerequisites

- Go 1.16 or later
- Docker
- Docker Compose

## Configuration

1. Copy the `config/config.json` file and fill in the required fields:
    ```json
    {
        "telegramToken": "YOUR_TELEGRAM_BOT_TOKEN"
    }
    ```

## Build and Run

### Using Makefile

1. **Build the project:**
    ```sh
    make build
    ```

2. **Run the project:**
    ```sh
    make run
    ```

### Using Docker Compose

1. **Build and run the project:**
    ```sh
    docker-compose up --build
    ```

## License

This project is licensed under the Apache License 2.0.