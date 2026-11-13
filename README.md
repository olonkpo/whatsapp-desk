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
Campaigns are saved locally in SQLite. Use **Save campaign** to preserve draft edits. Starting and quitting also save the current message, interval and recipient selection. On reopening, review the recovered campaign and press **Start sending** to resume ready recipients. Sending never restarts automatically. Interrupted sends become **uncertain** and remain excluded. **Clear completed** removes sent recipients from the active list while retaining history. Importing a new list replaces the saved campaign, so review history before sending again.

## Local data
Mac: ~/Library/Application Support/WhatsAppDesk
Windows: %AppData%\WhatsAppDesk
This folder contains the linked-device session, chats and send history.
Keep it private. Unlink this computer from WhatsApp's Linked Devices on your
phone when you no longer use it. No session or contact data ships in the ZIP.


Based on [WhatsApp MCP](https://github.com/lharries/whatsapp-mcp). See LICENSE and go.mod for dependencies.

Photo and video attachments: choose one JPEG/PNG photo or MP4 video up to 16 MB. The personalized message is sent as its caption (up to 1024 characters). Attachments are saved with the local campaign and can be removed before sending.

## Known limitations
Delivery/read receipts are not tracked. Uncertain sends halt the queue and require manual WhatsApp review. One photo/video is supported, with a 16 MB app limit. Builds are unsigned previews; Windows and Apple Silicon need target-machine testing. See [ROADMAP.md](ROADMAP.md) for planned work and [CONTRIBUTING.md](CONTRIBUTING.md) to contribute.
