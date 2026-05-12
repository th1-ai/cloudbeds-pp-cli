---
name: pp-cloudbeds
description: "The Cloudbeds CLI nobody else built — full PMS API surface plus a local SQLite mirror, FTS5 search, dated... Trigger phrases: `what's happening at the hotel today`, `which rooms are stale dirty`, `guests arriving with unpaid balance`, `reservation timeline for RES-`, `occupancy trend last week`, `use cloudbeds`, `run cloudbeds-pp-cli`."
author: "DigiGrowthAgency"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cloudbeds-pp-cli
---

# Cloudbeds — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cloudbeds-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cloudbeds --cli-only
   ```
2. Verify: `cloudbeds-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Cloudbeds reservation, guest, room, rate, and housekeeping record can be synced into a local SQLite store and queried offline with FTS5. On top of the 115-endpoint PMS API surface, this CLI adds front-desk and revenue-management commands — `today`, `housekeeping stale`, `rates drift`, `occupancy trend`, `payments unpaid`, `reservations timeline`, `audit night` — that the official SDKs cannot answer because they require dated snapshots or cross-table joins. Output is agent-native by default: `--json`, `--select`, typed exit codes, `--dry-run` for mutations, and an MCP server tree-mirrored from the Cobra command tree.

## When to Use This CLI

Use this CLI any time an agent needs Cloudbeds data shaped for shell pipelines or downstream reasoning — front-desk operations (`today`, `audit night`), housekeeping urgency (`housekeeping stale`), revenue management (`occupancy trend`, `rates drift`, `sources mix`), payment triage (`payments unpaid`), guest investigation (`guests search --history`, `reservations timeline`), and integration debugging (`reconcile`). Pick this CLI over a generic HTTP call when the question requires dated snapshots or cross-table joins, and pick the spec-derived endpoint commands when you need a typed wrapper for a specific RPC.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local time-series that compounds
- **`housekeeping stale`** — Find rooms that have been dirty longer than a threshold so you can chase them before arrivals.

  _Use this when an agent is asked which rooms are blocking arrivals; native Cloudbeds endpoints can't answer 'how long has this been dirty'._

  ```bash
  cloudbeds-pp-cli housekeeping stale --hours 4 --json
  ```
- **`rates drift`** — Show how rates moved over a window, by date, plan, and room type.

  _Use this when an agent is asked 'did our Tuesday rate hike actually move ADR' — needs persisted rate history the API does not expose._

  ```bash
  cloudbeds-pp-cli rates drift --days 14 --room-type STD --json
  ```
- **`occupancy trend`** — Daily occupancy, ADR, and RevPAR series with week-over-week deltas.

  _Use this for revenue reviews — series and WoW deltas no Cloudbeds endpoint surfaces._

  ```bash
  cloudbeds-pp-cli occupancy trend --since 14d --json
  ```

### Composed front-desk operations
- **`today`** — One-shot front-desk dashboard: arrivals, departures, in-house, unassigned, dirty rooms, today's ADR and occupancy.

  _Use this at shift handoff or when an agent is asked 'what's happening at the property right now' — one call instead of five._

  ```bash
  cloudbeds-pp-cli today --json
  ```
- **`reservations timeline`** — Single-reservation forensic view: status changes, payments, notes, room assignments — chronological.

  _Use this when an agent is asked to investigate a reservation dispute or guest complaint — one call instead of four._

  ```bash
  cloudbeds-pp-cli reservations timeline RES-12345 --json
  ```
- **`audit night`** — End-of-shift bundle: arrivals not checked-in, departures not checked-out, unbalanced folios, payments since last audit.

  _Use this for end-of-shift handoff — one agent-shaped doc replaces tabbing across four web-UI screens._

  ```bash
  cloudbeds-pp-cli audit night --json
  ```

### Cross-table aggregation
- **`sources mix`** — Bookings, nights, and revenue by source (Booking.com, Expedia, Direct, Airbnb) over a window.

  _Use this when an agent is asked 'where are bookings coming from' or for partner channel-mix reports._

  ```bash
  cloudbeds-pp-cli sources mix --since 30d --json
  ```
- **`payments unpaid`** — Reservations arriving in the next N days with non-zero balance.

  _Use this when an agent is asked 'who hasn't paid yet for an upcoming stay' — Cloudbeds web UI requires per-reservation drilldown._

  ```bash
  cloudbeds-pp-cli payments unpaid --within 7d --json
  ```
- **`reservations no-shows`** — No-show rate by source, room type, and lead-time bucket; flags repeat no-show guests.

  _Use this when an agent is asked which sources or segments are bleeding revenue to no-shows._

  ```bash
  cloudbeds-pp-cli reservations no-shows --since 30d --json
  ```

### Local search that beats the API
- **`guests search`** — Full-text search across guest name, email, phone, and notes; returns past stays with totals.

  _Use this when an agent is asked 'has this guest stayed before' or 'find all guests who booked the honeymoon suite' — FTS hides the verbose getGuestList payload._

  ```bash
  cloudbeds-pp-cli guests search "smith" --history --json --select guest.firstName,guest.lastName,stays.totalNights,stays.totalRevenue
  ```

### Agent-native plumbing
- **`reconcile`** — Compare local mirror to live API for a sampled window; emit a typed diff.

  _Use this when an agent is asked to verify a sync mirror is current, or to triage a 'guest says they booked but I don't see it' ticket._

  ```bash
  cloudbeds-pp-cli reconcile --table reservations --since 24h --json
  ```

## Command Reference

**access-token** — Manage access token

- `cloudbeds-pp-cli access-token` — Query the authorization server for an access token used to access property resources.</br> If the automatic delivery...

**append-custom-item** — Manage append custom item

- `cloudbeds-pp-cli append-custom-item` — Append single, or multiple, custom items and their associated payments to an existing one in a Reservation, House...

**create-allotment-block** — Manage create allotment block

- `cloudbeds-pp-cli create-allotment-block` — Retreive allotment blocks @apiQuery {Integer} propertyID Property ID

**create-allotment-block-notes** — Manage create allotment block notes

- `cloudbeds-pp-cli create-allotment-block-notes` — Add a note to an allotment block

**delete-adjustment** — Manage delete adjustment

- `cloudbeds-pp-cli delete-adjustment` — Voids the AdjustmentID transaction on the specified reservationID

**delete-allotment-block** — Manage delete allotment block

- `cloudbeds-pp-cli delete-allotment-block` — Delete allotment blocks

**delete-app-property-settings** — Manage delete app property settings

- `cloudbeds-pp-cli delete-app-property-settings` — Delete app property settings

**delete-guest-note** — Manage delete guest note

- `cloudbeds-pp-cli delete-guest-note` — Archives an existing guest note.

**delete-reservation-note** — Manage delete reservation note

- `cloudbeds-pp-cli delete-reservation-note` — Archives an existing reservation note.

**delete-room-block** — Manage delete room block

- `cloudbeds-pp-cli delete-room-block` — Deletes a room block

**delete-webhook** — Manage delete webhook

- `cloudbeds-pp-cli delete-webhook` — Remove subscription for webhook. Read the [Webhooks guide](https://integrations.cloudbeds.com/hc/en-us/articles/36000...

**get-allotment-blocks** — Manage get allotment blocks

- `cloudbeds-pp-cli get-allotment-blocks` — Retrieve allotment blocks

**get-app-property-settings** — Manage get app property settings

- `cloudbeds-pp-cli get-app-property-settings` — Returns the app property settings

**get-app-settings** — Manage get app settings

- `cloudbeds-pp-cli get-app-settings` — Get the current app settings for a property.<br />

**get-app-state** — Manage get app state

- `cloudbeds-pp-cli get-app-state` — Get the current app integration state for a property.<br /> This call is only available for third-party integration...

**get-available-room-types** — Manage get available room types

- `cloudbeds-pp-cli get-available-room-types` — Returns a list of room types with availability considering the informed parameters ### Group account support

**get-currency-settings** — Manage get currency settings

- `cloudbeds-pp-cli get-currency-settings` — Get currency settings

**get-custom-fields** — Manage get custom fields

- `cloudbeds-pp-cli get-custom-fields` — Gets custom fields list<br /> ¹ data.displayed = 'booking' - Display this field to guests on the booking engine.<br...

**get-dashboard** — Manage get dashboard

- `cloudbeds-pp-cli get-dashboard` — Returns basic information about the current state of the hotel

**get-email-schedule** — Manage get email schedule

- `cloudbeds-pp-cli get-email-schedule` — Returns a list of all existing email scheduling. This call is only available for third-party integration partners,...

**get-email-templates** — Manage get email templates

- `cloudbeds-pp-cli get-email-templates` — Returns a list of all existing email templates. This call is only available for third-party integration partners,...

**get-files** — Manage get files

- `cloudbeds-pp-cli get-files` — Returns a list of files attached to a hotel or group profile, ordered by creation date

**get-group-notes** — Manage get group notes

- `cloudbeds-pp-cli get-group-notes` — Returns group notes

**get-groups** — Manage get groups

- `cloudbeds-pp-cli get-groups` — Returns the groups for a property

**get-guest** — Manage get guest

- `cloudbeds-pp-cli get-guest` — Returns information on a guest specified by the Reservation ID parameter

**get-guest-list** — Manage get guest list

- `cloudbeds-pp-cli get-guest-list` — Returns a list of guests, ordered by modification date ### Group account support

**get-guest-notes** — Manage get guest notes

- `cloudbeds-pp-cli get-guest-notes` — Retrieves a guest notes

**get-guests-by-filter** — Manage get guests by filter

- `cloudbeds-pp-cli get-guests-by-filter` — Returns a list of guests matching the selected parameters ### Group account support

**get-guests-by-status** — Manage get guests by status

- `cloudbeds-pp-cli get-guests-by-status` — Returns a list of guests in the current status (Not Checked In, In House, Checked Out or Cancelled), sorted by...

**get-guests-modified** — Manage get guests modified

- `cloudbeds-pp-cli get-guests-modified` — Returns a list of guests based on their modification date. Note that when a guest checks in or checks out of a room,...

**get-hotel-details** — Manage get hotel details

- `cloudbeds-pp-cli get-hotel-details` — Returns the details of a specific hotel, identified by 'propertyID'

**get-hotels** — Manage get hotels

- `cloudbeds-pp-cli get-hotels` — Returns a list of hotels, filtered by the parameters passed ### Group account support

**get-house-account-list** — Manage get house account list

- `cloudbeds-pp-cli get-house-account-list` — Pulls list of active house accounts

**get-housekeepers** — Manage get housekeepers

- `cloudbeds-pp-cli get-housekeepers` — Returns a list of housekeepers ### Group account support

**get-housekeeping-status** — Manage get housekeeping status

- `cloudbeds-pp-cli get-housekeeping-status` — Returns the current date's housekeeping information The housekeeping status is calculated basing on the set of...

**get-item** — Manage get item

- `cloudbeds-pp-cli get-item` — Gets the details for the one itemID<br /> <sup>1</sup> only if data.stockInventory = true<br> <sup>2</sup> Taxes,...

**get-item-categories** — Manage get item categories

- `cloudbeds-pp-cli get-item-categories` — Gets the item category list

**get-items** — Manage get items

- `cloudbeds-pp-cli get-items` — Gets all the items and their prices the hotel has created in myfrontdesk<br> <sup>1</sup> only if...

**get-package-names** — Manage get package names

- `cloudbeds-pp-cli get-package-names` — Return a list of billing package names for a property

**get-packages** — Manage get packages

- `cloudbeds-pp-cli get-packages` — This efficient method allows you to retrieve the collection of packages associated with a property. Packages here...

**get-payment-methods** — Manage get payment methods

- `cloudbeds-pp-cli get-payment-methods` — Get a list of active methods for a property, or list of properties

**get-payments-capabilities** — Manage get payments capabilities

- `cloudbeds-pp-cli get-payments-capabilities` — Lists the payment capabilities of a given property

**get-rate** — Manage get rate

- `cloudbeds-pp-cli get-rate` — Returns the rate of the room type selected, based on the provided parameters

**get-rate-jobs** — Manage get rate jobs

- `cloudbeds-pp-cli get-rate-jobs` — Returns a list of Rate Jobs. Rate jobs are only returned within 7 days of creation, after 7 days they will not be...

**get-rate-plans** — Manage get rate plans

- `cloudbeds-pp-cli get-rate-plans` — Returns the rates of the room type or promo code selected, based on the provided parameters. If no parameters are...

**get-reservation** — Manage get reservation

- `cloudbeds-pp-cli get-reservation` — Returns information on a booking specified by the reservationID parameter

**get-reservation-assignments** — Manage get reservation assignments

- `cloudbeds-pp-cli get-reservation-assignments` — Returns a list of rooms/reservations assigned for a selected date.

**get-reservation-notes** — Manage get reservation notes

- `cloudbeds-pp-cli get-reservation-notes` — Retrieves reservation notes based on parameters

**get-reservation-room-details** — Manage get reservation room details

- `cloudbeds-pp-cli get-reservation-room-details` — Returns information about particular room in reservation by its subReservationID

**get-reservations** — Manage get reservations

- `cloudbeds-pp-cli get-reservations` — Returns a list of reservations that matched the filters criteria.<br /> Please note that some reservations...

**get-reservations-with-rate-details** — Manage get reservations with rate details

- `cloudbeds-pp-cli get-reservations-with-rate-details` — Returns a list of reservations with added information regarding booked rates and sources.<br /> Please note that...

**get-room-blocks** — Manage get room blocks

- `cloudbeds-pp-cli get-room-blocks` — Returns a list of all room blocks considering the informed parameters.

**get-room-types** — Manage get room types

- `cloudbeds-pp-cli get-room-types` — Returns a list of room types filtered by the selected parameters ### Group account support

**get-rooms** — Manage get rooms

- `cloudbeds-pp-cli get-rooms` — Returns a list of all rooms considering the informed parameters. If Check-in/out dates are sent, only unassigned...

**get-rooms-fees-and-taxes** — Manage get rooms fees and taxes

- `cloudbeds-pp-cli get-rooms-fees-and-taxes` — Get applicable fees and tax to a booking. This is meant to be used on checkout to display to the guest.

**get-rooms-unassigned** — Manage get rooms unassigned

- `cloudbeds-pp-cli get-rooms-unassigned` — Returns a list of unassigned rooms in the property. Call is alias of [getRooms](#api-Room-getRooms). Please check...

**get-sources** — Manage get sources

- `cloudbeds-pp-cli get-sources` — Gets available property sources

**get-taxes-and-fees** — Manage get taxes and fees

- `cloudbeds-pp-cli get-taxes-and-fees` — Returns the taxes and fees set for the property. Read the [Rate-Based tax (Dynamic Tax)...

**get-users** — Manage get users

- `cloudbeds-pp-cli get-users` — Returns information on the properties' users ### Group account support

**get-webhooks** — Manage get webhooks

- `cloudbeds-pp-cli get-webhooks` — List webhooks for which the API client is subscribed to.

**list-allotment-block-notes** — Manage list allotment block notes

- `cloudbeds-pp-cli list-allotment-block-notes` — List notes added to an allotment block

**oauth** — Manage oauth

- `cloudbeds-pp-cli oauth` — In the context of properties being distributed across multiple localizations, this endpoint serves to retrieve the...

**patch-group** — Manage patch group

- `cloudbeds-pp-cli patch-group` — Updates an existing group with information provided. At least one information field is required for this call.

**patch-rate** — Manage patch rate

- `cloudbeds-pp-cli patch-rate` — Update the rate of the room based on rateID selected, based on the provided parameters. You can make multiple rate...

**post-adjustment** — Manage post adjustment

- `cloudbeds-pp-cli post-adjustment` — Adds an adjustment to a reservation

**post-app-error** — Manage post app error

- `cloudbeds-pp-cli post-app-error` — Submit the error received by the hybrid integration from the partner to the MFD

**post-app-property-settings** — Manage post app property settings

- `cloudbeds-pp-cli post-app-property-settings` — Post app property settings

**post-app-state** — Manage post app state

- `cloudbeds-pp-cli post-app-state` — Update app integration state for a property ID. <br /> This call is only available for third-party integration...

**post-charge** — Manage post charge

- `cloudbeds-pp-cli post-charge` — Use a payment method to process a payment on a reservation, group profile, accounts receivable ledger, or house account.

**post-credit-card** — Manage post credit card

- `cloudbeds-pp-cli post-credit-card` — Returns the rate of the room type selected, based on the provided parameters

**post-custom-field** — Manage post custom field

- `cloudbeds-pp-cli post-custom-field` — Sets custom fields. The call should only be made once to add the field to the system.

**post-custom-item** — Manage post custom item

- `cloudbeds-pp-cli post-custom-item` — Adds single, or multiple, custom items and their associated payments to a Reservation or House Account as a single...

**post-custom-payment-method** — Manage post custom payment method

- `cloudbeds-pp-cli post-custom-payment-method` — Add a Custom Payment Method to a property. This call does not allow to add Payment Methods: credit cards, bank...

**post-email-schedule** — Manage post email schedule

- `cloudbeds-pp-cli post-email-schedule` — Creates a new email schedule for existing email template. Email template can be scheduled based on two parameters:...

**post-email-template** — Manage post email template

- `cloudbeds-pp-cli post-email-template` — Creates a new email template. See the full list of available language parameters <a...

**post-file** — Manage post file

- `cloudbeds-pp-cli post-file` — Attaches a file to a hotel

**post-government-receipt** — Manage post government receipt

- `cloudbeds-pp-cli post-government-receipt` — Add a Government Receipt to a Reservation or House Account

**post-group-note** — Manage post group note

- `cloudbeds-pp-cli post-group-note` — Adds a group note

**post-guest** — Manage post guest

- `cloudbeds-pp-cli post-guest` — Adds a guest to reservation as an additional guest.

**post-guest-document** — Manage post guest document

- `cloudbeds-pp-cli post-guest-document` — Attaches a document to a guest

**post-guest-note** — Manage post guest note

- `cloudbeds-pp-cli post-guest-note` — Adds a guest note

**post-guest-photo** — Manage post guest photo

- `cloudbeds-pp-cli post-guest-photo` — Attaches a photo to a guest

**post-guests-to-room** — Manage post guests to room

- `cloudbeds-pp-cli post-guests-to-room` — Assigns guest(s) to a room in a reservation and adds these guests as additional guests.

**post-housekeeper** — Manage post housekeeper

- `cloudbeds-pp-cli post-housekeeper` — Add New Housekeeper

**post-housekeeping-assignment** — Manage post housekeeping assignment

- `cloudbeds-pp-cli post-housekeeping-assignment` — Assign rooms (single or multiple) to an existing housekeeper

**post-housekeeping-status** — Manage post housekeeping status

- `cloudbeds-pp-cli post-housekeeping-status` — Switches the current date's housekeeping status for a specific room ID to either clean or dirty The housekeeping...

**post-item** — Manage post item

- `cloudbeds-pp-cli post-item` — Adds an item either to a reservation or to a house account.

**post-item-category** — Manage post item category

- `cloudbeds-pp-cli post-item-category` — Adds new items category

**post-items-to-inventory** — Manage post items to inventory

- `cloudbeds-pp-cli post-items-to-inventory` — Adds new items batch<br /> ¹ only if item.stockInventory = true<br />

**post-new-house-account** — Manage post new house account

- `cloudbeds-pp-cli post-new-house-account` — Add a new House Account

**post-payment** — Manage post payment

- `cloudbeds-pp-cli post-payment` — Add a payment to a specified reservation, house account, or group. If multiple IDs are provided, precedence is...

**post-reservation** — Manage post reservation

- `cloudbeds-pp-cli post-reservation` — Adds a reservation to the selected property

**post-reservation-document** — Manage post reservation document

- `cloudbeds-pp-cli post-reservation-document` — Attaches a document to a reservation

**post-reservation-note** — Manage post reservation note

- `cloudbeds-pp-cli post-reservation-note` — Adds a reservation note

**post-room-assign** — Manage post room assign

- `cloudbeds-pp-cli post-room-assign` — Assign/Reassign a room on a guest reservation

**post-room-block** — Manage post room block

- `cloudbeds-pp-cli post-room-block` — Adds a room block to the selected property.

**post-room-check-in** — Manage post room check in

- `cloudbeds-pp-cli post-room-check-in` — Check-in a room already assigned for a guest

**post-room-check-out** — Manage post room check out

- `cloudbeds-pp-cli post-room-check-out` — Check-out a room already assigned for a guest. If all rooms are checked out, the reservation status will update...

**post-void-item** — Manage post void item

- `cloudbeds-pp-cli post-void-item` — Voids the itemID transaction on the specified Reservation ID, House Account ID, or Group. If payments were sent in...

**post-void-payment** — Manage post void payment

- `cloudbeds-pp-cli post-void-payment` — Voids a payment (using paymentID) to a specified reservation or house account.

**post-webhook** — Manage post webhook

- `cloudbeds-pp-cli post-webhook` — Subscribe a webhook for a specified event. Read the [Webhooks...

**put-app-property-settings** — Manage put app property settings

- `cloudbeds-pp-cli put-app-property-settings` — Put app property settings

**put-group** — Manage put group

- `cloudbeds-pp-cli put-group` — Adds a group to the property. Please note that the default setting for 'Route to Group Folio' will be 'No,' and the...

**put-guest** — Manage put guest

- `cloudbeds-pp-cli put-guest` — Updates an existing guest with information provided. At least one information field is required for this call.

**put-guest-note** — Manage put guest note

- `cloudbeds-pp-cli put-guest-note` — Updates an existing guest note.

**put-house-account-status** — Manage put house account status

- `cloudbeds-pp-cli put-house-account-status` — Change specific house account to either open or closed.

**put-housekeeper** — Manage put housekeeper

- `cloudbeds-pp-cli put-housekeeper` — Edit Housekeeper Details

**put-item-to-inventory** — Manage put item to inventory

- `cloudbeds-pp-cli put-item-to-inventory` — Updates an item with information provided<br /> ¹ only if item.stockInventory = true<br />

**put-rate** — Manage put rate

- `cloudbeds-pp-cli put-rate` — Update the rate of the room based on rateID selected, based on the provided parameters. You can make multiple rate...

**put-reservation** — Manage put reservation

- `cloudbeds-pp-cli put-reservation` — Updates a reservation, such as custom fields, estimated arrival time, room configuration and reservation status.

**put-reservation-note** — Manage put reservation note

- `cloudbeds-pp-cli put-reservation-note` — Updates an existing reservation note.

**put-room-block** — Manage put room block

- `cloudbeds-pp-cli put-room-block` — Updates a room block.

**update-allotment-block** — Manage update allotment block

- `cloudbeds-pp-cli update-allotment-block` — Update an allotment block @apiQuery {Integer} propertyID Property ID

**update-allotment-block-notes** — Manage update allotment block notes

- `cloudbeds-pp-cli update-allotment-block-notes` — Update a note on an allotment block

**userinfo** — Manage userinfo

- `cloudbeds-pp-cli userinfo` — Returns information on user who authorized connection


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cloudbeds-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Front-desk shift handoff in one JSON blob

```bash
cloudbeds-pp-cli today --json --select arrivals.count,departures.count,in_house.count,unassigned.count,housekeeping.dirty_count,kpi.adr,kpi.occupancy
```

Narrows the today board to the seven numbers a shift supervisor reads to a Slack channel. Without --select agents waste tokens parsing in-house guest objects.

### Find the long-staying VIP

```bash
cloudbeds-pp-cli guests search "vip" --history --json --select guest.firstName,guest.lastName,stays.totalNights,stays.lastStay
```

LIKE-based search across guest name, email, phone fields joined to stay history; returns four agent-readable fields per match instead of the verbose getGuestList payload. Use the top-level `search` command for FTS5.

### Did Tuesday's rate hike stick?

```bash
cloudbeds-pp-cli rates drift --days 14 --room-type STD --json
```

Diffs dated rate snapshots — answer to a question Cloudbeds returns only current rates and cannot answer without local history.

### Mirror reconcile in CI

```bash
cloudbeds-pp-cli reconcile --table reservations --since 24h --json --select api_count,local_count,added,removed
```

Hooks into a CI step that runs after every webhook deploy; non-zero exit code if any rows differ.

## Auth Setup

Cloudbeds API keys start with `cbat_` and never expire unless unused for 30 days. Set `CLOUDBEDS_OAUTH2` in your environment, or run `cloudbeds-pp-cli auth set-token <cbat_…>` to persist the token to `~/.config/cloudbeds-pp-cli/config.toml`. Confirm with `auth status`. The CLI sends the token as `Authorization: Bearer cbat_…`.

Run `cloudbeds-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
cloudbeds-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cloudbeds-pp-cli feedback --stdin < notes.txt
cloudbeds-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cloudbeds-pp-cli/feedback.jsonl`. They are never POSTed unless `CLOUDBEDS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLOUDBEDS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
cloudbeds-pp-cli profile save briefing --json
cloudbeds-pp-cli --profile briefing get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000
cloudbeds-pp-cli profile list --json
cloudbeds-pp-cli profile show briefing
cloudbeds-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `cloudbeds-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cloudbeds-pp-mcp -- cloudbeds-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cloudbeds-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cloudbeds-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cloudbeds-pp-cli <command> --help`.
