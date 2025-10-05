# Thumb Bot - Telegram Media Processor

A Telegram bot that processes media files from various platforms (Twitter, Instagram, Vocaroo) and converts them to thumbnails or previews.

## Features

- **Webhook-based**: Uses Telegram webhooks instead of long polling for better serverless compatibility
- **Multi-platform support**: Processes media from Twitter, Instagram, and Vocaroo
- **Vercel ready**: Optimized for serverless deployment on Vercel
- **Modern Go**: Built with Go 1.21+ and modern libraries

## Architecture

The bot has been converted from a long-polling architecture to a webhook-based system:

- **`cmd/main.go`**: Local development server
- **`api/index.go`**: Vercel serverless function entry point
- **`internal/webhook/`**: Webhook handling logic
- **`internal/service/`**: Media processing services
- **`internal/integration/`**: External platform integrations

## Prerequisites

- Go 1.21 or higher
- Telegram Bot Token
- Vercel account (for deployment)

## Local Development

1. **Clone the repository**:
   ```bash
   git clone <your-repo-url>
   cd thumb-bot
   ```

2. **Install dependencies**:
   ```bash
   go mod tidy
   ```

3. **Set up configuration**:
   Create a `config/config.json` file with your bot token:
   ```json
   {
     "telegram_token": "YOUR_BOT_TOKEN_HERE"
   }
   ```

4. **Run locally**:
   ```bash
   go run cmd/main.go
   ```

   The server will start on port 3000.

## Vercel Deployment

### Quick Deploy

1. **Deploy with script**:
   ```bash
   ./deploy.sh
   ```

2. **Manual deployment**:
   ```bash
   vercel --prod
   ```

### Environment Variables

Set these in your Vercel project dashboard:

- `TELEGRAM_TOKEN`: Your Telegram bot token
- `WEBHOOK_URL`: Your Vercel app URL (e.g., `https://your-app.vercel.app`)

### Webhook Setup

After deployment, your webhook will be available at:
```
https://your-app.vercel.app/webhook
```

The bot automatically sets the webhook URL in production environments.

## API Endpoints

- `GET /health` - Health check endpoint
- `POST /webhook` - Telegram webhook endpoint

## Testing

### Local Testing

```bash
# Health check
curl http://localhost:3000/health

# Test webhook (with sample update)
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{"update_id": 123, "message": {"text": "test"}}'
```

### Vercel Testing

```bash
# Health check
curl https://your-app.vercel.app/health

# Webhook endpoint
curl -X POST https://your-app.vercel.app/webhook \
  -H "Content-Type: application/json" \
  -d '{"update_id": 123, "message": {"text": "test"}}'
```

## Configuration

The bot uses a JSON configuration file located at `config/config.json`:

```json
{
  "telegram_token": "YOUR_BOT_TOKEN",
  "log_level": "info"
}
```

## Development

### Project Structure

```
thumb-bot/
├── api/                 # Vercel serverless functions
│   └── index.go        # Main webhook handler
├── cmd/                 # Local development
│   └── main.go         # Local server
├── internal/            # Internal packages
│   ├── webhook/        # Webhook handling
│   ├── service/        # Business logic
│   ├── integration/    # External APIs
│   └── infra/          # Infrastructure
├── config/              # Configuration files
├── vercel.json          # Vercel configuration
└── deploy.sh            # Deployment script
```

### Adding New Features

1. **New media type**: Add to `internal/integration/`
2. **New service**: Add to `internal/service/`
3. **New webhook handler**: Add to `internal/webhook/`

## Troubleshooting

### Common Issues

1. **Webhook not working**: Check if the webhook URL is set correctly in Telegram
2. **Build failures**: Ensure Go version is 1.21+
3. **Environment variables**: Verify all required env vars are set in Vercel

### Logs

Check Vercel function logs in the dashboard for debugging information.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test locally and on Vercel
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review Vercel deployment logs
3. Open an issue on GitHub