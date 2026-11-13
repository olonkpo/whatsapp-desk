# WhatsApp Desk

A local app for Excel and CSV recipient lists, personalized messages, and WhatsApp group member exports.

## Build and run

Use Go 1.26 or later. Run `go test ./...`, then `go build -o WhatsAppDesk .` and open the resulting executable. The app opens in your browser.

## Link your account
On your phone, open WhatsApp > Settings > Linked Devices > Link a Device.
Scan the QR code shown in the app. Each computer creates its own linked device.
Do not copy session files from another computer. If a pairing code expires,
close and reopen the app to generate a fresh code. Internet access is required.

## Use the app
Import an Excel .xlsx or CSV file with First Name, Last Name and Phone Number
columns. Put international phone numbers in the first worksheet. Or click
Choose a WhatsApp group, search your groups and extract a participant list.
Names from WhatsApp are shown for reference: Edit name sets first/last names.
Some group members' numbers may be unavailable; the app reports the count.
Download list saves an Excel-compatible CSV file.

Write your message with {name}, {first_name}, {last_name} or {phone}, then preview it.
Select recipients, set the interval and start sending. Pause/Resume and Stop
control the queue; an in-flight send may finish. An uncertain send stops the
queue and must be checked in WhatsApp before retrying, to avoid duplicates.
Sent means accepted by WhatsApp, not confirmation of delivery or reading.

Send history is saved locally and can be viewed with Send history.
The active list/queue is in memory and is not resumed after quitting. Reimport
starts a new list, so review history before sending the same message again.

## Local data
Mac: ~/Library/Application Support/WhatsAppDesk
Windows: %AppData%\WhatsAppDesk
This folder contains the linked-device session, chats and send history.
Keep it private. Unlink this computer from WhatsApp's Linked Devices on your
phone when you no longer use it. No session or contact data ships in the ZIP.


Based on [WhatsApp MCP](https://github.com/lharries/whatsapp-mcp). See LICENSE and go.mod for dependencies.

Photo and video attachments: choose one JPEG/PNG photo or MP4 video up to 16 MB. The personalized message is sent as its caption (up to 1024 characters). Attachments remain in memory and can be removed before sending.
