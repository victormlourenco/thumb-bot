#!/bin/bash

echo "🧪 Testing Webhook Functionality..."

# Test health endpoint
echo "📡 Testing health endpoint..."
curl -s http://localhost:3000/health | jq '.' 2>/dev/null || curl -s http://localhost:3000/health

echo -e "\n📡 Testing webhook endpoint..."
# Test webhook with sample update
curl -s -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "update_id": 123456789,
    "message": {
      "message_id": 1,
      "from": {
        "id": 123456,
        "is_bot": false,
        "first_name": "Test",
        "username": "testuser"
      },
      "chat": {
        "id": 123456,
        "first_name": "Test",
        "username": "testuser",
        "type": "private"
      },
      "date": 1640995200,
      "text": "Hello, bot!"
    }
  }' | jq '.' 2>/dev/null || echo "Response received"

echo -e "\n✅ Test completed!"
echo "💡 Make sure the bot is running with: go run cmd/main.go"
