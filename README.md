# Cloudbeds CLI

**The Cloudbeds CLI nobody else built — full PMS API surface plus a local SQLite mirror, FTS5 search, dated snapshots, and front-desk transcendence commands no SDK wrapper offers.**

Every Cloudbeds reservation, guest, room, rate, and housekeeping record can be synced into a local SQLite store and queried offline with FTS5. On top of the 115-endpoint PMS API surface, this CLI adds front-desk and revenue-management commands — `today`, `housekeeping stale`, `rates drift`, `occupancy trend`, `payments unpaid`, `reservations timeline`, `audit night` — that the official SDKs cannot answer because they require dated snapshots or cross-table joins. Output is agent-native by default: `--json`, `--select`, typed exit codes, `--dry-run` for mutations, and an MCP server tree-mirrored from the Cobra command tree.

Learn more at [Cloudbeds](https://www.cloudbeds.com/).

## Install

The recommended path installs both the `cloudbeds-pp-cli` binary and the `pp-cloudbeds` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cloudbeds
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cloudbeds --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloudbeds-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cloudbeds --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cloudbeds --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cloudbeds skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cloudbeds. The skill defines how its required CLI can be installed.
```

## Authentication

Cloudbeds API keys start with `cbat_` and never expire unless unused for 30 days. Set `CLOUDBEDS_OAUTH2` in your environment, or run `cloudbeds-pp-cli auth set-token <cbat_…>` to persist the token to `~/.config/cloudbeds-pp-cli/config.toml`. Confirm with `auth status`. The CLI sends the token as `Authorization: Bearer cbat_…`.

## Quick Start

```bash
# Verify your API key is set and the API is reachable.
cloudbeds-pp-cli doctor --json


# Mirror the last 7 days of reservations, guests, rooms, rates, and housekeeping into the local store.
cloudbeds-pp-cli sync --resources get-reservations --since 7d


# One-shot front-desk dashboard: arrivals, departures, in-house, unassigned, dirty rooms, today's ADR/occupancy.
cloudbeds-pp-cli today --json


# Find rooms dirty longer than 4 hours — Cloudbeds API can't answer this without local snapshots.
cloudbeds-pp-cli housekeeping stale --hours 4 --json


# Reservations arriving in the next 7 days with non-zero balance.
cloudbeds-pp-cli payments unpaid --within 7d --json

```

## Unique Features

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

## Usage

Run `cloudbeds-pp-cli --help` for the full command reference and flag list.

## Commands

### access-token

Manage access token

- **`cloudbeds-pp-cli access-token create`** - Query the authorization server for an access token used to access property resources.</br> If the automatic delivery method for API keys is used, the grant type `urn:ietf:params:oauth:grant-type:api-key` needs to be used to request an API key. This grant type requires `grant_type=urn:ietf:params:oauth:grant-type:api-key`, `client_id`, `client_secret`, `redirect_uri` and `code`.</br> For OAuth 2.0., two different grant types (`authorization_code`, `refresh_token`) are supported. Authorization code grant type requires `grant_type=authorization_code`, `client_id`, `client_secret`, `redirect_uri`, `code`. Refresh token grant type requires `grant_type=refresh_token`, `client_id`, `client_secret`, `refresh_token`.</br> Read the [Authentication guide](https://integrations.cloudbeds.com/hc/en-us/sections/14731510501915-Authentication) for implementation tips, user flows and testing advice.

### append-custom-item

Manage append custom item

- **`cloudbeds-pp-cli append-custom-item create`** - Append single, or multiple, custom items and their associated payments to an existing one in a Reservation, House Account, or Group.

### create-allotment-block

Manage create allotment block

- **`cloudbeds-pp-cli create-allotment-block create`** - Retreive allotment blocks @apiQuery {Integer} propertyID Property ID

### create-allotment-block-notes

Manage create allotment block notes

- **`cloudbeds-pp-cli create-allotment-block-notes create`** - Add a note to an allotment block

### delete-adjustment

Manage delete adjustment

- **`cloudbeds-pp-cli delete-adjustment delete`** - Voids the AdjustmentID transaction on the specified reservationID

### delete-allotment-block

Manage delete allotment block

- **`cloudbeds-pp-cli delete-allotment-block create`** - Delete allotment blocks

### delete-app-property-settings

Manage delete app property settings

- **`cloudbeds-pp-cli delete-app-property-settings create`** - Delete app property settings

### delete-guest-note

Manage delete guest note

- **`cloudbeds-pp-cli delete-guest-note delete`** - Archives an existing guest note.

### delete-reservation-note

Manage delete reservation note

- **`cloudbeds-pp-cli delete-reservation-note delete`** - Archives an existing reservation note.

### delete-room-block

Manage delete room block

- **`cloudbeds-pp-cli delete-room-block delete`** - Deletes a room block

### delete-webhook

Manage delete webhook

- **`cloudbeds-pp-cli delete-webhook delete`** - Remove subscription for webhook. Read the [Webhooks guide](https://integrations.cloudbeds.com/hc/en-us/articles/360007612553-Webhooks) to see available objects, actions, payload info and more. ### Group account support

### get-allotment-blocks

Manage get allotment blocks

- **`cloudbeds-pp-cli get-allotment-blocks list`** - Retrieve allotment blocks

### get-app-property-settings

Manage get app property settings

- **`cloudbeds-pp-cli get-app-property-settings list`** - Returns the app property settings

### get-app-settings

Manage get app settings

- **`cloudbeds-pp-cli get-app-settings list`** - Get the current app settings for a property.<br />

### get-app-state

Manage get app state

- **`cloudbeds-pp-cli get-app-state list`** - Get the current app integration state for a property.<br /> This call is only available for third-party integration partners, and not for property client IDs. Read the [Connecting/Disconnecting Apps guide](https://integrations.cloudbeds.com/hc/en-us/articles/360007613213-Connecting-Disconnecting-Apps) to further understand the use cases.

### get-available-room-types

Manage get available room types

- **`cloudbeds-pp-cli get-available-room-types list`** - Returns a list of room types with availability considering the informed parameters ### Group account support

### get-currency-settings

Manage get currency settings

- **`cloudbeds-pp-cli get-currency-settings list`** - Get currency settings

### get-custom-fields

Manage get custom fields

- **`cloudbeds-pp-cli get-custom-fields list`** - Gets custom fields list<br /> ¹ data.displayed = "booking" - Display this field to guests on the booking engine.<br /> ¹ data.displayed = "reservation" - Add this field to the reservation folio for use by staff.<br /> ¹ data.displayed = "card" - Make this field available for registration cards.<br />

### get-dashboard

Manage get dashboard

- **`cloudbeds-pp-cli get-dashboard list`** - Returns basic information about the current state of the hotel

### get-email-schedule

Manage get email schedule

- **`cloudbeds-pp-cli get-email-schedule list`** - Returns a list of all existing email scheduling. This call is only available for third-party integration partners, and not for property client IDs.

### get-email-templates

Manage get email templates

- **`cloudbeds-pp-cli get-email-templates list`** - Returns a list of all existing email templates. This call is only available for third-party integration partners, and not for property client IDs.

### get-files

Manage get files

- **`cloudbeds-pp-cli get-files list`** - Returns a list of files attached to a hotel or group profile, ordered by creation date

### get-group-notes

Manage get group notes

- **`cloudbeds-pp-cli get-group-notes list`** - Returns group notes

### get-groups

Manage get groups

- **`cloudbeds-pp-cli get-groups list`** - Returns the groups for a property

### get-guest

Manage get guest

- **`cloudbeds-pp-cli get-guest list`** - Returns information on a guest specified by the Reservation ID parameter

### get-guest-list

Manage get guest list

- **`cloudbeds-pp-cli get-guest-list list`** - Returns a list of guests, ordered by modification date ### Group account support

### get-guest-notes

Manage get guest notes

- **`cloudbeds-pp-cli get-guest-notes list`** - Retrieves a guest notes

### get-guests-by-filter

Manage get guests by filter

- **`cloudbeds-pp-cli get-guests-by-filter list`** - Returns a list of guests matching the selected parameters ### Group account support

### get-guests-by-status

Manage get guests by status

- **`cloudbeds-pp-cli get-guests-by-status list`** - Returns a list of guests in the current status (Not Checked In, In House, Checked Out or Cancelled), sorted by modification date. If no date range is passed, it returns all guests with the selected status. ### Group account support

### get-guests-modified

Manage get guests modified

- **`cloudbeds-pp-cli get-guests-modified list`** - Returns a list of guests based on their modification date. Note that when a guest checks in or checks out of a room, their record is modified at that time. If no date range is passed, only the records for the current day are returned. Also note that if the guest is assigned to multiple rooms, it will result in multiple records. ### Group account support

### get-hotel-details

Manage get hotel details

- **`cloudbeds-pp-cli get-hotel-details list`** - Returns the details of a specific hotel, identified by "propertyID"

### get-hotels

Manage get hotels

- **`cloudbeds-pp-cli get-hotels list`** - Returns a list of hotels, filtered by the parameters passed ### Group account support

### get-house-account-list

Manage get house account list

- **`cloudbeds-pp-cli get-house-account-list list`** - Pulls list of active house accounts

### get-housekeepers

Manage get housekeepers

- **`cloudbeds-pp-cli get-housekeepers list`** - Returns a list of housekeepers ### Group account support

### get-housekeeping-status

Manage get housekeeping status

- **`cloudbeds-pp-cli get-housekeeping-status list`** - Returns the current date's housekeeping information The housekeeping status is calculated basing on the set of fields roomOccupied | roomCondition | roomBlocked | vacantPickup | roomBlocked | refusedService The available statuses are: - Vacant and Dirty (VD): false | “dirty” | false | false | false | false - Occupied and Dirty (OD): true | “dirty” | false | false | false | false - Vacant and Clean (VC): false | “clean” | false | false | false | false - Occupied and Clean (OC): true | “clean” | false | false | false | false - Occupied and Clean Inspected (OCI): true | “inspected” | false | false | false | false - Vacant and Clean Inspected (VCI): false | “inspected” | false | false | false | false - Do Not Disturb (DND): if doNotDisturb is true - Refused Service (RS): if refusedService is true - Out of Order (OOO): if roomBlocked is true - Vacant and Pickup (VP): if vacantPickup is true

### get-item

Manage get item

- **`cloudbeds-pp-cli get-item list`** - Gets the details for the one itemID<br /> <sup>1</sup> only if data.stockInventory = true<br> <sup>2</sup> Taxes, fees and totals will show up only if an item has assigned tax or fee.<br>

### get-item-categories

Manage get item categories

- **`cloudbeds-pp-cli get-item-categories list`** - Gets the item category list

### get-items

Manage get items

- **`cloudbeds-pp-cli get-items list`** - Gets all the items and their prices the hotel has created in myfrontdesk<br> <sup>1</sup> only if data.stockInventory = true<br> <sup>2</sup> Taxes, fees and totals will show up only if an item has assigned tax or fee.<br>

### get-package-names

Manage get package names

- **`cloudbeds-pp-cli get-package-names list`** - Return a list of billing package names for a property

### get-packages

Manage get packages

- **`cloudbeds-pp-cli get-packages list`** - This efficient method allows you to retrieve the collection of packages associated with a property. Packages here define a group of features that a property has the ability to utilize or access. By invoking this API method, developers will get a comprehensive view of the feature sets that are available and active for a specific property. The getPackages method boasts a seamless execution that offers essential information, vital in enhancing property management, understanding available functionalities and ultimately, optimizing user experience.

### get-payment-methods

Manage get payment methods

- **`cloudbeds-pp-cli get-payment-methods list`** - Get a list of active methods for a property, or list of properties

### get-payments-capabilities

Manage get payments capabilities

- **`cloudbeds-pp-cli get-payments-capabilities list`** - Lists the payment capabilities of a given property

### get-rate

Manage get rate

- **`cloudbeds-pp-cli get-rate list`** - Returns the rate of the room type selected, based on the provided parameters

### get-rate-jobs

Manage get rate jobs

- **`cloudbeds-pp-cli get-rate-jobs list`** - Returns a list of Rate Jobs. Rate jobs are only returned within 7 days of creation, after 7 days they will not be returned in the response. Requests which do not provide a jobReferenceID will be filtered by the client ID of the request's token.

### get-rate-plans

Manage get rate plans

- **`cloudbeds-pp-cli get-rate-plans list`** - Returns the rates of the room type or promo code selected, based on the provided parameters. If no parameters are provided, then the method will return all publicly available rate plans. ### Group account support

### get-reservation

Manage get reservation

- **`cloudbeds-pp-cli get-reservation list`** - Returns information on a booking specified by the reservationID parameter

### get-reservation-assignments

Manage get reservation assignments

- **`cloudbeds-pp-cli get-reservation-assignments list`** - Returns a list of rooms/reservations assigned for a selected date.

### get-reservation-notes

Manage get reservation notes

- **`cloudbeds-pp-cli get-reservation-notes list`** - Retrieves reservation notes based on parameters

### get-reservation-room-details

Manage get reservation room details

- **`cloudbeds-pp-cli get-reservation-room-details list`** - Returns information about particular room in reservation by its subReservationID

### get-reservations

Manage get reservations

- **`cloudbeds-pp-cli get-reservations list`** - Returns a list of reservations that matched the filters criteria.<br /> Please note that some reservations modification may not be reflected in this timestamp. ### Group account support

### get-reservations-with-rate-details

Manage get reservations with rate details

- **`cloudbeds-pp-cli get-reservations-with-rate-details list`** - Returns a list of reservations with added information regarding booked rates and sources.<br /> Please note that some reservations modification may not be reflected in this timestamp.

### get-room-blocks

Manage get room blocks

- **`cloudbeds-pp-cli get-room-blocks list`** - Returns a list of all room blocks considering the informed parameters.

### get-room-types

Manage get room types

- **`cloudbeds-pp-cli get-room-types list`** - Returns a list of room types filtered by the selected parameters ### Group account support

### get-rooms

Manage get rooms

- **`cloudbeds-pp-cli get-rooms list`** - Returns a list of all rooms considering the informed parameters. If Check-in/out dates are sent, only unassigned rooms are returned. ### Group account support

### get-rooms-fees-and-taxes

Manage get rooms fees and taxes

- **`cloudbeds-pp-cli get-rooms-fees-and-taxes list`** - Get applicable fees and tax to a booking. This is meant to be used on checkout to display to the guest.

### get-rooms-unassigned

Manage get rooms unassigned

- **`cloudbeds-pp-cli get-rooms-unassigned list`** - Returns a list of unassigned rooms in the property. Call is alias of [getRooms](#api-Room-getRooms). Please check its documentation for parameters, response and example. ### Group account support

### get-sources

Manage get sources

- **`cloudbeds-pp-cli get-sources list`** - Gets available property sources

### get-taxes-and-fees

Manage get taxes and fees

- **`cloudbeds-pp-cli get-taxes-and-fees list`** - Returns the taxes and fees set for the property. Read the [Rate-Based tax (Dynamic Tax) guide](https://myfrontdesk.cloudbeds.com/hc/en-us/articles/360014103514-rate-based-tax-dynamic-tax) to understand its usage.

### get-users

Manage get users

- **`cloudbeds-pp-cli get-users list`** - Returns information on the properties' users ### Group account support

### get-webhooks

Manage get webhooks

- **`cloudbeds-pp-cli get-webhooks list`** - List webhooks for which the API client is subscribed to.

### list-allotment-block-notes

Manage list allotment block notes

- **`cloudbeds-pp-cli list-allotment-block-notes list`** - List notes added to an allotment block

### oauth

Manage oauth

- **`cloudbeds-pp-cli oauth list`** - In the context of properties being distributed across multiple localizations, this endpoint serves to retrieve the precise location of the property associated with the provided access token. Further information can be found in the [Authentication guide](https://integrations.cloudbeds.com/hc/en-us/sections/14731510501915-Authentication).

### patch-group

Manage patch group

- **`cloudbeds-pp-cli patch-group create`** - Updates an existing group with information provided. At least one information field is required for this call.

### patch-rate

Manage patch rate

- **`cloudbeds-pp-cli patch-rate create`** - Update the rate of the room based on rateID selected, based on the provided parameters. You can make multiple rate updates in a single API call. Providing a startDate and/or endDate will update rates only within the interval provided. Only non derived rates can be updated, requests to update a derived rate will return an error. This endpoint performs updates asynchronously,  rate updates are added to a queue and the endpoint returns a job reference ID. This job reference ID can be used to track job status notifications or to look up details of the update once it is completed. The API is limited to 30 interval per update, sending more than 30 will return an error.

### post-adjustment

Manage post adjustment

- **`cloudbeds-pp-cli post-adjustment create`** - Adds an adjustment to a reservation

### post-app-error

Manage post app error

- **`cloudbeds-pp-cli post-app-error create`** - Submit the error received by the hybrid integration from the partner to the MFD

### post-app-property-settings

Manage post app property settings

- **`cloudbeds-pp-cli post-app-property-settings create`** - Post app property settings

### post-app-state

Manage post app state

- **`cloudbeds-pp-cli post-app-state create`** - Update app integration state for a property ID. <br /> This call is only available for third-party integration partners, and not for property client IDs. <br /> If an app is set to 'disabled', it will remove all active sessions Read the [Connecting/Disconnecting Apps guide](https://integrations.cloudbeds.com/hc/en-us/articles/360007613213-Connecting-Disconnecting-Apps) to further understand the use cases.

### post-charge

Manage post charge

- **`cloudbeds-pp-cli post-charge create`** - Use a payment method to process a payment on a reservation, group profile, accounts receivable ledger, or house account.

### post-credit-card

Manage post credit card

- **`cloudbeds-pp-cli post-credit-card create`** - Returns the rate of the room type selected, based on the provided parameters

### post-custom-field

Manage post custom field

- **`cloudbeds-pp-cli post-custom-field create`** - Sets custom fields. The call should only be made once to add the field to the system.

### post-custom-item

Manage post custom item

- **`cloudbeds-pp-cli post-custom-item create`** - Adds single, or multiple, custom items and their associated payments to a Reservation or House Account as a single transaction.

### post-custom-payment-method

Manage post custom payment method

- **`cloudbeds-pp-cli post-custom-payment-method create`** - Add a Custom Payment Method to a property. This call does not allow to add Payment Methods: credit cards, bank transfer or Pay Pal.

### post-email-schedule

Manage post email schedule

- **`cloudbeds-pp-cli post-email-schedule create`** - Creates a new email schedule for existing email template. Email template can be scheduled based on two parameters: reservationStatusChange and reservationEvent. Only one of the parameters can be used. *reservationStatusChange* schedules email to be sent when reservation status transitions to a specific one, for instance: `confirmed`. *reservationEvent* schedules email to be sent number of days prior or after a specific event, for instance: `after_check_out` at a given time This call is only available for third-party integration partners, and not for property client IDs.

### post-email-template

Manage post email template

- **`cloudbeds-pp-cli post-email-template create`** - Creates a new email template. See the full list of available language parameters <a href="https://integrations.cloudbeds.com/hc/en-us/articles/360007144993-FAQ#methods-and-parameters">here</a>. This call is only available for third-party integration partners, and not for property client IDs.

### post-file

Manage post file

- **`cloudbeds-pp-cli post-file create`** - Attaches a file to a hotel

### post-government-receipt

Manage post government receipt

- **`cloudbeds-pp-cli post-government-receipt create`** - Add a Government Receipt to a Reservation or House Account

### post-group-note

Manage post group note

- **`cloudbeds-pp-cli post-group-note create`** - Adds a group note

### post-guest

Manage post guest

- **`cloudbeds-pp-cli post-guest create`** - Adds a guest to reservation as an additional guest.

### post-guest-document

Manage post guest document

- **`cloudbeds-pp-cli post-guest-document create`** - Attaches a document to a guest

### post-guest-note

Manage post guest note

- **`cloudbeds-pp-cli post-guest-note create`** - Adds a guest note

### post-guest-photo

Manage post guest photo

- **`cloudbeds-pp-cli post-guest-photo create`** - Attaches a photo to a guest

### post-guests-to-room

Manage post guests to room

- **`cloudbeds-pp-cli post-guests-to-room create`** - Assigns guest(s) to a room in a reservation and adds these guests as additional guests.

### post-housekeeper

Manage post housekeeper

- **`cloudbeds-pp-cli post-housekeeper create`** - Add New Housekeeper

### post-housekeeping-assignment

Manage post housekeeping assignment

- **`cloudbeds-pp-cli post-housekeeping-assignment create`** - Assign rooms (single or multiple) to an existing housekeeper

### post-housekeeping-status

Manage post housekeeping status

- **`cloudbeds-pp-cli post-housekeeping-status create`** - Switches the current date's housekeeping status for a specific room ID to either clean or dirty The housekeeping status is calculated basing on the set of fields roomOccupied | roomCondition | roomBlocked | vacantPickup | roomBlocked | refusedService The available statuses are: - Vacant and Dirty (VD): false | “dirty” | false | false | false | false - Occupied and Dirty (OD): true | “dirty” | false | false | false | false - Vacant and Clean (VC): false | “clean” | false | false | false | false - Occupied and Clean (OC): true | “clean” | false | false | false | false - Occupied and Clean Inspected (OCI): true | “inspected” | false | false | false | false - Vacant and Clean Inspected (VCI): false | “inspected” | false | false | false | false - Do Not Disturb (DND): if doNotDisturb is true - Refused Service (RS): if refusedService is true - Out of Order (OOO): if roomBlocked is true - Vacant and Pickup (VP): if vacantPickup is true

### post-item

Manage post item

- **`cloudbeds-pp-cli post-item create`** - Adds an item either to a reservation or to a house account.

### post-item-category

Manage post item category

- **`cloudbeds-pp-cli post-item-category create`** - Adds new items category

### post-items-to-inventory

Manage post items to inventory

- **`cloudbeds-pp-cli post-items-to-inventory create`** - Adds new items batch<br /> ¹ only if item.stockInventory = true<br />

### post-new-house-account

Manage post new house account

- **`cloudbeds-pp-cli post-new-house-account create`** - Add a new House Account

### post-payment

Manage post payment

- **`cloudbeds-pp-cli post-payment create`** - Add a payment to a specified reservation, house account, or group. If multiple IDs are provided, precedence is reservationID, then houseAccountID, then groupCode.

### post-reservation

Manage post reservation

- **`cloudbeds-pp-cli post-reservation create`** - Adds a reservation to the selected property

### post-reservation-document

Manage post reservation document

- **`cloudbeds-pp-cli post-reservation-document create`** - Attaches a document to a reservation

### post-reservation-note

Manage post reservation note

- **`cloudbeds-pp-cli post-reservation-note create`** - Adds a reservation note

### post-room-assign

Manage post room assign

- **`cloudbeds-pp-cli post-room-assign create`** - Assign/Reassign a room on a guest reservation

### post-room-block

Manage post room block

- **`cloudbeds-pp-cli post-room-block create`** - Adds a room block to the selected property.

### post-room-check-in

Manage post room check in

- **`cloudbeds-pp-cli post-room-check-in create`** - Check-in a room already assigned for a guest

### post-room-check-out

Manage post room check out

- **`cloudbeds-pp-cli post-room-check-out create`** - Check-out a room already assigned for a guest. If all rooms are checked out, the reservation status will update accordingly to "Checked Out" as well.

### post-void-item

Manage post void item

- **`cloudbeds-pp-cli post-void-item create`** - Voids the itemID transaction on the specified Reservation ID, House Account ID, or Group. If payments were sent in calls [postItem](https://developers.cloudbeds.com/reference/post_postitem) or [postCustomItem](https://developers.cloudbeds.com/reference/post_postcustomitem), they will be deleted too.

### post-void-payment

Manage post void payment

- **`cloudbeds-pp-cli post-void-payment create`** - Voids a payment (using paymentID) to a specified reservation or house account.

### post-webhook

Manage post webhook

- **`cloudbeds-pp-cli post-webhook create`** - Subscribe a webhook for a specified event. Read the [Webhooks guide](https://integrations.cloudbeds.com/hc/en-us/articles/360007612553-Webhooks) to see available objects, actions, payload info and more.

### put-app-property-settings

Manage put app property settings

- **`cloudbeds-pp-cli put-app-property-settings create`** - Put app property settings

### put-group

Manage put group

- **`cloudbeds-pp-cli put-group create`** - Adds a group to the property. Please note that the default setting for 'Route to Group Folio' will be 'No,' and the 'Reservation Folio Configuration' will be set as the default folio configuration. You can edit these settings through the user interface (UI).

### put-guest

Manage put guest

- **`cloudbeds-pp-cli put-guest update`** - Updates an existing guest with information provided. At least one information field is required for this call.

### put-guest-note

Manage put guest note

- **`cloudbeds-pp-cli put-guest-note update`** - Updates an existing guest note.

### put-house-account-status

Manage put house account status

- **`cloudbeds-pp-cli put-house-account-status update`** - Change specific house account to either open or closed.

### put-housekeeper

Manage put housekeeper

- **`cloudbeds-pp-cli put-housekeeper update`** - Edit Housekeeper Details

### put-item-to-inventory

Manage put item to inventory

- **`cloudbeds-pp-cli put-item-to-inventory update`** - Updates an item with information provided<br /> ¹ only if item.stockInventory = true<br />

### put-rate

Manage put rate

- **`cloudbeds-pp-cli put-rate create`** - Update the rate of the room based on rateID selected, based on the provided parameters. You can make multiple rate updates in a single API call. Providing a startDate and/or endDate will update rates only within the interval provided. Only non derived rates can be updated, requests to update a derived rate will return an error. This endpoint performs updates asynchronously,  rate updates are added to a queue and the endpoint returns a job reference ID. This job reference ID can be used to track job status notifications or to look up details of the update once it is completed. The API is limited to 30 interval per update, sending more than 30 will return an error.

### put-reservation

Manage put reservation

- **`cloudbeds-pp-cli put-reservation update`** - Updates a reservation, such as custom fields, estimated arrival time, room configuration and reservation status.

### put-reservation-note

Manage put reservation note

- **`cloudbeds-pp-cli put-reservation-note update`** - Updates an existing reservation note.

### put-room-block

Manage put room block

- **`cloudbeds-pp-cli put-room-block update`** - Updates a room block.

### update-allotment-block

Manage update allotment block

- **`cloudbeds-pp-cli update-allotment-block create`** - Update an allotment block @apiQuery {Integer} propertyID Property ID

### update-allotment-block-notes

Manage update allotment block notes

- **`cloudbeds-pp-cli update-allotment-block-notes create`** - Update a note on an allotment block

### userinfo

Manage userinfo

- **`cloudbeds-pp-cli userinfo list`** - Returns information on user who authorized connection


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cloudbeds-pp-cli get-allotment-blocks --property-id 550e8400-e29b-41d4-a716-446655440000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cloudbeds -g
```

Then invoke `/pp-cloudbeds <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cloudbeds cloudbeds-pp-mcp -e CLOUDBEDS_OAUTH2=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloudbeds-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLOUDBEDS_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cloudbeds": {
      "command": "cloudbeds-pp-mcp",
      "env": {
        "CLOUDBEDS_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cloudbeds-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cloudbeds-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLOUDBEDS_OAUTH2` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cloudbeds-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLOUDBEDS_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 access_denied — Missing Authorization header** — Set `CLOUDBEDS_OAUTH2=cbat_…` in your environment, or run `cloudbeds-pp-cli auth set-token cbat_…` to persist the token. Run `cloudbeds-pp-cli doctor` to verify.
- **429 with Retry-After — rate limit hit** — The CLI's adaptive limiter respects Retry-After automatically; wait or rerun. Cloudbeds caps properties at 5 rps and tech-partners at 10 rps.
- **Empty results from `today` or `housekeeping stale`** — Run `cloudbeds-pp-cli sync` first. Transcendence commands read the local mirror — if it's empty they have nothing to compose.
- **API key got blocked after a script bug** — Cloudbeds blocks the key (and your IP) after repeated rate-limit violations. Open a ticket with Cloudbeds developer support — the CLI cannot unblock you.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**cloudbeds-api-python**](https://github.com/cloudbeds/cloudbeds-api-python) — Python
- [**tipi-cloudbeds**](https://www.npmjs.com/package/tipi-cloudbeds) — JavaScript
- [**r4kib/cloudbeds-api**](https://github.com/r4kib/cloudbeds-api) — PHP
- [**MGPelloni/cloudbeds**](https://github.com/MGPelloni/cloudbeds) — PHP
- [**CB-API-Explorer**](https://github.com/cloudbeds/CB-API-Explorer) — C#
- [**CBAPI-OperationalReportsClientApp**](https://github.com/cloudbeds/CBAPI-OperationalReportsClientApp) — C#

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
