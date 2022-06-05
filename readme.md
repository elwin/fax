# Fax

This project is quite simple: It receives messages on a Telegram Bot and forwards them to my thermal printer at home.

## Installation
```bash
go install github.com/elwin/fax
```

## Usage
```bash
// Possibly you need to give rw permission to your device:
// sudo chmod +777 /dev/usb/lp0

fax --device_path /dev/usb/lp0 --telegram_token abc123
```
