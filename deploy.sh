#!/bin/bash

echo "🚀 Deploying Telegram Bot to Vercel..."

# Check if vercel CLI is installed
if ! command -v vercel &> /dev/null; then
    echo "❌ Vercel CLI not found. Installing..."
    npm install -g vercel
fi

# Deploy to Vercel
echo "📦 Building and deploying..."
vercel --prod

echo "✅ Deployment complete!"
echo "🔗 Your webhook URL will be: https://your-app.vercel.app/webhook"
echo "📝 Don't forget to set your environment variables in Vercel dashboard:"
echo "   - TELEGRAM_TOKEN"
echo "   - WEBHOOK_URL"
