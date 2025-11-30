# Contact Form Setup Guide

The contact form sends submissions directly to Telegram and is protected by reCAPTCHA v3.

## Environment Variables

Add these variables to your `.env.development` or `.env.production` file:

```bash
# Telegram Configuration (for contact form)
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
TELEGRAM_CHAT_ID=your_telegram_chat_id_here

# Google reCAPTCHA v3 Configuration
RECAPTCHA_SECRET_KEY=your_recaptcha_secret_key_here
```

## Setup Steps

### 1. Get Telegram Bot Token

Your bot is already created: [@GiraffeCloudBot](https://t.me/GiraffeCloudBot)

You should have the bot token from @BotFather.

### 2. Get Your Telegram Chat ID

**Option A: Using @userinfobot (Easiest)**

1. Open Telegram
2. Search for `@userinfobot`
3. Start the bot
4. It will show your chat ID

**Option B: Using Your Bot**

1. Send any message to [@GiraffeCloudBot](https://t.me/GiraffeCloudBot)
2. Open this URL (replace `YOUR_BOT_TOKEN`):
   ```
   https://api.telegram.org/botYOUR_BOT_TOKEN/getUpdates
   ```
3. Look for `"chat":{"id":123456789}` in the JSON response
4. That number is your chat ID

### 3. Get reCAPTCHA v3 Keys

1. Go to [Google reCAPTCHA Admin](https://www.google.com/recaptcha/admin)
2. Click "+" to create a new site
3. Settings:
   - Label: `GiraffeCloud Contact Form`
   - reCAPTCHA type: **reCAPTCHA v3**
   - Domains: Add your domains (e.g., `giraffecloud.com`, `localhost`)
4. Submit
5. Copy the **Site Key** and **Secret Key**

### 4. Add to Frontend Environment

Add the reCAPTCHA site key to your frontend `.env.local`:

```bash
NEXT_PUBLIC_RECAPTCHA_SITE_KEY=your_recaptcha_site_key_here
```

## API Endpoint

**POST** `/api/v1/contact/submit`

### Request Body

```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "message": "Your message here (max 1000 chars)",
  "recaptcha_token": "token_from_recaptcha_v3"
}
```

### Rate Limiting

- **5 requests per hour** per IP address
- Prevents spam and abuse

### Validation

- Name: 2-100 characters
- Email: valid email format
- Message: 10-1000 characters
- reCAPTCHA: minimum score 0.5

## Testing

1. Start the backend server
2. Submit a message through the contact form
3. Check your Telegram for the notification

The message will appear formatted like:

```
🆕 New Contact Form Submission

Name: John Doe
Email: john@example.com
Message:
Your message here...
```

## Security Features

- ✅ reCAPTCHA v3 bot protection
- ✅ Rate limiting (5 requests/hour per IP)
- ✅ Input validation and sanitization
- ✅ HTML escape for Telegram messages
- ✅ Maximum message length enforcement
