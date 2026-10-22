# Machine Logbook Conversations

Direction approved by the user: industrial light, warm white surfaces, charcoal text,
navy accent from the supplied logo, machine-first conversations for factory operators.
Apply antislop during implementation.

Reading: a continuous machine logbook for factory operators, using the clarity of a
paper register with a restrained industrial palette. ENERGY 1 / RHYTHM 2 / MOTION 1.

- Warm paper background keeps scanned logbook pages visually familiar.
- Charcoal text and high contrast controls support reading on the factory floor.
- Navy identifies the primary action and selected machine, not decoration.
- System sans-serif keeps labels legible and avoids external font requests.
- Author names and server timestamps form the register motif; all values come from data.
- Choose a machine first, then read its continuous timeline. No date or shift entry,
  cross-machine feed, invented metrics, or charts.
- Native dialogs provide focus containment and Escape handling. Controls have visible
  focus, at least 44px touch targets, and meaningful text labels.
- Motion is limited to brief hover/focus transitions; reduced motion is respected.
- The user-supplied IMCPro logo is displayed unchanged on login and the app header.

## IMCPro refresh

The supplied logo anchors the light industrial direction. Navy marks primary
actions; red and yellow remain in the logo only. White message surfaces sit on
a pale navy reading area to distinguish the timeline from the composer. The
system sans-serif and existing ENERGY 1 / RHYTHM 2 / MOTION 1 dials remain.

## Machine photography

User-supplied machine cutouts identify the initial machine chooser: MAILENDER, MS3, COATING
(shared by COATING 1–3 and COATING INLINE), and RuiYuan. Contain-fit images preserve the full
machine. Up to six columns on wide screens become three on tablets and two on phones. A small
machine photo beside the conversation heading keeps the destination visible.

## Conversation layout

One machine has one timeline, independent of shifts and dates. The user's WhatsApp-like
interaction direction uses a viewport-fit shell: back control, small existing machine
photo, machine name, scrolling history, and compact bottom composer. Global app chrome
stays on the chooser and archives, not inside a conversation. No fake online status,
read receipts, or realtime claims are shown.

Date separators group server timestamps chronologically. Authenticated user ID, never
list position or author name, aligns the user's messages right with a pale navy fill;
other messages stay white on the left. Quiet clock times keep message content primary.
IMCPro navy remains the send action accent, not WhatsApp green. Existing system type,
machine photos, and ENERGY 1 / RHYTHM 2 / MOTION 1 preserve the approved identity.

The composer accepts text, gallery photos, camera captures, or both. One-line text grows
up to a bounded height, while removable thumbnails stay in a horizontal strip. Existing
camera, plus, back, and up icons identify capture, attachment, navigation, and send;
accessible names and visible focus explain every icon-only control. Drafts survive
machine navigation and failed requests within the tab. Server data remains authoritative.

Historical archives retain their actual dates, shifts, author, timestamps, and
photos. A labelled archive link provides context without fabricated message text.

## Sender-owned message deletion

The quiet, labelled delete control sits on the sender's native message so removal
stays tied to its content without competing with the navy send action. Other
authors' messages and legacy archives have no delete control; admin status does
not override native-message ownership. Existing industrial surfaces, system type,
44px targets, visible focus, and ENERGY 1 / RHYTHM 2 / MOTION 1 remain unchanged.

A native confirmation dialog names the machine and timestamp and warns that the
message and all its photos will be permanently deleted. Cancel restores focus;
pending deletion blocks duplicate actions and dismissal. Failure keeps the row
and dialog available for retry. Success removes the row and returns focus to the
timeline while preserving the composer draft, keeping deletion separate from sending.

## Sender-owned message editing

The labelled Edit action stays beside its native message, using existing industrial
colors, system type, 44px controls, and ENERGY 1 / RHYTHM 2 / MOTION 1. A separate
native dialog keeps correction apart from the unsent composer draft and photos.
The saved text is prefilled and focused; cancel and Escape return focus to Edit.
Pending saves prevent duplicate actions and dismissal, while failure retains the
edit for retry. Only the server's edited_at displays Diedit; unchanged saves do not
mark a message. Photos, original timestamp, and timeline position stay unchanged.
Sender ownership is enforced by the API, including for admins, with no time limit.

## Search within a machine

Reading: text retrieval inside the existing factory logbook, retaining industrial
light, system type, and ENERGY 1 / RHYTHM 2 / MOTION 1.

The header's labelled search button keeps discovery beside the machine identity.
An inline form sits above the scrolling timeline so search never replaces the
composer, camera, or machine-scoped draft. White controls and the existing navy
submit action distinguish input from the pale reading surface without new assets.
The form stacks its input above 44px actions on phones and joins one row when
space permits. Existing focus styles, an explicit label, Enter submission, and
Escape with focus return support keyboard use. No extra animation is introduced.

Typing is a draft search; submitting commits the trimmed query to the URL. Results
keep the existing chronological message layout, author names, dates, and protected
photos. A visible query summary distinguishes filtered history from the full
timeline without implying a total count. Loading, failed search with retry, and
no-match guidance remain distinct. Reset or close restores the complete history.
Query changes discard pending pages; sending in search refreshes server results
rather than inserting a possibly unrelated message. Back/Forward and reload use
the committed URL query, while composer drafts retain their existing lifecycle.


## Responsive and accessible behavior

Mobile uses compact 44px controls, dynamic viewport sizing, visual-viewport keyboard
updates, and safe-area padding. Only history scrolls vertically during normal reading;
the composer stays in the shell's layout rather than covering messages. Longer text
scrolls within its bounded input. Older pages preserve the reading position when
prepended, and photo slots reserve space while protected images load.

Tablet adds reading gutters and two-column photo groups. Desktop caps the shell width
so short chat lines do not stretch across the monitor. Long pasted tokens wrap and
multiline whitespace remains visible. White controls and pale navy own-message bubbles
separate actions from content without shadows, decoration, or new assets.

## Camera-first capture

Reading: repeated machine evidence capture for factory operators, retaining industrial
light, system type, and ENERGY 1 / RHYTHM 2 / MOTION 1.

The live view gets the flexible center of a viewport-fit dialog; a navy circular
shutter remains bottom-center within thumb reach. The light control surface keeps
the existing conversation identity, while the dark viewfinder separates photographs
from controls. Contain-fit preview and review show the complete frame, not a crop.
Canvas capture uses the camera stream's original pixel dimensions.

Close, gallery fallback, and camera switch remain visible above the viewfinder.
Captured photos join the existing unsent draft immediately without closing the camera.
The count and horizontal thumbnail strip confirm each capture; selecting a thumbnail
opens full-frame review with removal and return controls. Remove then capture again
retakes a photo without losing other attachments. Selesai returns to chat; only Kirim
pesan uploads. Close and Escape also preserve completed captures.

Phones use the full dynamic viewport with safe-area padding. Larger portrait screens
use a bounded dialog; short landscape screens put controls beside the viewfinder.
Only thumbnails scroll horizontally; the shutter never sits below a scrolling page.
Controls retain 44px targets, native dialog focus containment, visible focus, and
text feedback. No animation, new assets, filters, zoom, or dependencies are introduced.
Permission failures retain retry and gallery access; streams stop on switch, close,
unmount, timeout, and late permission completion.

## Verification handoff

Conversation browser tests cover text/photos/both, actual camera capture and upload,
escaped multiline content, historical photos, machine drafts, pagination, failures,
own-author alignment, date groups, timeline-only scrolling, axe, and responsive widths.
Search coverage adds full-history matching, machine isolation, literal text,
matching photos, result pagination, URL navigation, stale requests, search errors,
empty results, draft preservation, and sends that do not match the active query.
Camera cases cover repeated capture, full-size output, shared selection and twenty-photo
limit, review/removal, done/close/reopen, permission denial with gallery fallback,
stream cleanup (including late permission), and portrait/landscape shutter visibility.
Runtime, contrast, zoom, and device checks must be performed by the main verifier;
no visual or accessibility pass is claimed solely from implementation.
