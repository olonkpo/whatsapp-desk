# Roadmap

## Implemented locally — campaign recovery
- SQLite campaign storage, including attachment bytes.
- Save campaign and Clear completed actions.
- Restore recipients, message and interval on startup without automatic sending.
- Treat interrupted sends as uncertain; never automatically retry them.
- Stop before sending if campaign storage fails.

## Next — delivery visibility
Store WhatsApp message IDs and process receipt events. Show delivered/read counts when receipts are available. Verify receipt handling against the installed whatsmeow API; this app connects directly to whatsmeow and does not require an MCP receipt endpoint.

## Later — review and attachments
Add explicit review actions for uncertain sends and an optional continue-to-review mode. Keep halt-on-uncertain as the default. Do not assume automatic retries are duplicate-safe.
Add document uploads and then multiple attachments, tracking each outgoing message separately. Verify current service limits before changing app limits.

## Later — portability and polish
Evaluate encrypted history backups separately from account session migration. Re-link each computer by QR until a supported migration design is tested. Evaluate a native wrapper after reliability work. Mobile access requires its own authentication and deployment design; never expose the current local server directly to the internet.

## Validation still needed
Live attachment delivery, target-machine Windows/Apple Silicon execution, and live receipt tracking.
