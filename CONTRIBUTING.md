# Contributing

Use Go 1.26 or later. Run `go test ./...` and `go vet ./...` before submitting a pull request. Format Go changes with `gofmt`. Build using `CGO_ENABLED=0 go build -o WhatsAppDesk .`.

For isolated manual testing, set WHATSAPP_DESK_DATA_DIR to an empty temporary directory. Set WHATSAPP_DESK_NO_BROWSER=1 to suppress opening a browser. Never send real messages without the account owner's permission.

Explain the problem, resulting behavior and relevant validation in each pull request. Include meaningful regression tests for persistence and sending changes. Preserve host/token guards and the rule that uncertain sends are not retried automatically.

Never commit session files, QR codes, recipient lists, chats, histories, logs, attachments or credentials. Use invented data in reports and tests. Screenshot reports must hide account details and phone numbers.
