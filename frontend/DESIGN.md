# Machine Logbook Archive

Direction approved by the user: industrial light, warm white surfaces, charcoal text,
green accent, compact archive lists, large camera controls for machine operators.
Apply antislop during implementation.

Reading: a working archive for factory operators, using the clarity of a paper
register with a restrained industrial palette. ENERGY 1 / RHYTHM 2 / MOTION 1.

- Warm paper background keeps scanned logbook pages visually familiar.
- Charcoal text and high contrast controls support reading on the factory floor.
- Forest green identifies the primary action and selected machine, not decoration.
- System sans-serif keeps labels legible and avoids external font requests.
- Dates and page numbers form the repeated register motif; all values come from data.
- Choose a machine first, then browse its archive list. No cross-machine list,
  invented metrics, charts, or activity feeds.
- Native dialogs provide focus containment and Escape handling. Controls have visible
  focus, at least 44px touch targets, and meaningful text labels.
- Motion is limited to brief hover/focus transitions; reduced motion is respected.
- The user-supplied IMCPro logo is displayed unchanged on login and the app header.

## IMCPro refresh

The supplied logo now anchors the light industrial direction. Navy replaces green
for primary actions and selected machines; red and yellow remain in the logo only.
Login uses a pale navy panel and more compact mobile typography. White filter
surfaces distinguish search controls from archive results. Numbered photo sections
and the raised save bar separate preparation from submission. Existing accessible
controls and the ENERGY 1 / RHYTHM 2 / MOTION 1 dials remain in place.

## Machine photography

User-supplied machine cutouts identify the initial machine chooser: MAILENDER, MS3, COATING
(shared by COATING 1–3), and RuiYuan. Contain-fit images preserve the full machine.
Six columns on wide screens become three on tablets and two on phones. Capture
shows a smaller machine preview; the chosen machine is locked for new archives.

## Archive navigation

Archive lists always belong to one machine. Date and shift filters remain within
that machine. Previous/next shift links show the destination date and shift and
advance one slot at a time, including empty shifts. Shift 3 advances to Shift 1
on the next log date. Empty slots offer a prefilled new archive form. No counter
entry or comparison calculation is introduced.

Successful uploads offer an explicit “Input mesin lain” action alongside the
confirmation. The existing photo chooser then opens a new capture form directly,
carrying only the shift. New forms default to the previous local calendar day;
an explicitly selected empty shift retains its date. The success action spans
the width on phones so it is easy to reach without competing with photo controls.
