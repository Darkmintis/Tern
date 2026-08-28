# Telegram Setup

Get release notifications in Telegram when builds succeed or fail.

## Quick Setup (2 minutes)

### 1. Create Bot

1. Open Telegram, search for **@BotFather**
2. Send `/newbot`
3. Name: `YourTeam Release Bot`
4. Username: `yourteam_tern_bot`
5. Copy the **bot token** (looks like `123456789:ABCdefGHI...`)

### 2. Get Chat ID

**For a group:**
1. Add your bot to the group
2. Send any message in the group
3. Open: `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates`
4. Find `"chat":{"id":-1001234567890}` — that's your chat ID

**For a channel:**
1. Add your bot as admin to the channel
2. Same API call to get chat ID

### 3. Configure Tern

Add to your `.env`:

```bash
TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz
TELEGRAM_CHAT_ID=-1001234567890
```

### 4. Test

```bash
tern doctor  # Should show: env:TELEGRAM_BOT_TOKEN: present
```

## What You'll Receive

### On Successful Release
```
✅ Release Shipped

Version: 1.2.3
Platform: android
Track: internal

[🚀 Promote to Production]
[⏪ Emergency Rollback]
[📊 Check Status]
```

### On Failed Release
```
🚨 RELEASE FAILED

Version: 1.2.3
Error: upload failed: 403 permission denied

[🔄 Retry Release] [⏪ Rollback Now]
[📜 View Full Logs]
```

## GitHub Actions Setup

Add secrets to your repository:

```
Settings → Secrets and variables → Actions

TELEGRAM_BOT_TOKEN=123456789:ABCdef...
TELEGRAM_CHAT_ID=-1001234567890
```

The workflow will automatically notify on success/failure.

## Ternfile Configuration

Add `notify` step to your lane:

```ternfile
lane release:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal
  tag git prefix:v
  notify telegram env:TELEGRAM_BOT_TOKEN
```

## Troubleshooting

| Problem | Fix |
|---------|-----|
| No notifications | Check `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` are set |
| "bot was kicked" | Re-add bot to the group |
| "chat not found" | Verify chat ID is correct (negative for groups) |
| Button links not working | Create a bot via @BotFather, not a userbot |

## Security Notes

- Bot token gives full control over the bot — keep it secret
- Don't commit `.env` to git (it's in `.gitignore`)
- Rotate token if exposed: send `/token` to @BotFather
