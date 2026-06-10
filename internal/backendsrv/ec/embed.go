package ec

// embed.go builds the EC auction alert's discordgo rich embed (D-04/D-05) and the
// inbox-row Detail summary. It is pure presentation — no network, no DB. The
// embed rides notify.Send's existing send-path (Alert.Embed); this file only
// SHAPES the payload, it never sends.
//
// Field discipline (D-05):
//   - Price is OMITTED when nil (never render "0pp" — a null price is "unknown").
//   - Seller is OMITTED when unresolved (best-effort via the players map; the
//     key→auction relationship is undocumented, so this often returns "").
//   - "Why you wanted it" echoes the want's saved Note (or the "on your
//     wantlist" fallback) so a ping months later still makes sense.
//
// SECURITY (V7): item names + the players map are wiki/PigParse-controlled text
// rendered into a DM the bot sends; this file shapes them into the embed but
// NEVER logs them (the slog discipline lives in ec.go and logs ids/status only).

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/boejowen/SquireBot/internal/backendsrv/enrich"
	"github.com/boejowen/SquireBot/internal/backendsrv/wantmatch"
)

// embedColor is the EC alert's left-bar accent — a warm EC-tunnel gold (Claude's
// discretion, D-04). Cosmetic only.
const embedColor = 0xC9A227

// wikiURLFor derives the P1999 wiki page URL from an item name (D-06: the embed
// links to the wiki, NOT a native item route — none exists on the SvelteKit
// frontend). It replicates the web idiom (web/src/lib/tooltip/composeNotes.ts
// wikiUrlFor): https://wiki.project1999.com/<Item_Name> with spaces → underscores,
// then enrich.EncodeURIComponent (the SAME escaper the wiki job uses — matches the
// JS encodeURIComponent byte-for-byte). Returns "" for a blank name so the caller
// can omit the URL.
func wikiURLFor(name string) string {
	t := strings.TrimSpace(name)
	if t == "" {
		return ""
	}
	return "https://wiki.project1999.com/" + enrich.EncodeURIComponent(strings.ReplaceAll(t, " ", "_"))
}

// seenAgo renders the auction's age from its RFC3339-ish timestamp t (e.g.
// "~3 min ago"). The getdetails t carries a +00:00 offset and VARIABLE
// fractional-second precision (e.g. ...29.35+00:00 vs ...29.279+00:00), so it is
// parsed with the RFC3339 family (which tolerates the fractional part). An
// unparseable or future t returns "" so the caller omits the Seen field rather
// than rendering a nonsense age.
func seenAgo(t string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return ""
	}
	d := now.Sub(parsed)
	if d < 0 {
		return "" // a future timestamp ⇒ omit rather than render "in -2m"
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("~%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("~%d hr ago", int(d.Hours()))
	default:
		return fmt.Sprintf("~%d days ago", int(d.Hours()/24))
	}
}

// resolveSeller is the best-effort seller lookup (D-05). ItemAuctionDetail carries
// NO seller field; the players map's key→specific-auction relationship is
// undocumented (21-SPIKE.md), so a deterministic per-auction seller is not
// resolvable from the record alone. We try the auction-instance id (a.I) as the
// players-map key — the only plausible join — and return "" on any miss. The DM is
// NEVER blocked on this: an unresolved seller silently omits the field.
func resolveSeller(a enrich.ItemAuctionDetail, players map[string]string) string {
	if players == nil {
		return ""
	}
	if name, ok := players[strconv.Itoa(a.I)]; ok {
		return strings.TrimSpace(name)
	}
	return ""
}

// whyWanted composes the "why you wanted it" line from the want's optional saved
// Note (D-05) — the wantlister's own context. Returns the trimmed note when
// present, else the "on your wantlist" fallback — NEVER empty, so the embed
// field is always present (the never-empty contract buildEmbed relies on).
func whyWanted(hit wantmatch.Hit) string {
	if hit.Note != nil {
		if note := strings.TrimSpace(*hit.Note); note != "" {
			return note
		}
	}
	return "on your wantlist"
}

// buildEmbed shapes the rich embed for one wantlister's hit on one new WTS auction
// (D-04/D-05). Title carries the item name + the WTS tag (always WTS — D-02);
// URL points at the P1999 wiki (D-06). Price is OMITTED when a.P is nil (never
// "0pp"); Seen is omitted when t is unparseable; Seller is omitted when unresolved.
// seenStr/seller are passed in (computed once by the caller) so the embed builder
// stays pure and easily tested.
func buildEmbed(hit wantmatch.Hit, a enrich.ItemAuctionDetail, seller, seenStr string) *discordgo.MessageEmbed {
	fields := make([]*discordgo.MessageEmbedField, 0, 4)

	// Price — OMIT when nil (a null price renders nothing, never "0pp" — D-05).
	if a.P != nil {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Price",
			Value:  fmt.Sprintf("~%d pp", *a.P),
			Inline: true,
		})
	}
	// Seen — OMIT when t is unparseable (seenStr == "").
	if seenStr != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Seen",
			Value:  seenStr,
			Inline: true,
		})
	}
	// Seller — best-effort; OMIT when unresolved (D-05).
	if seller != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Seller",
			Value:  seller,
			Inline: true,
		})
	}
	// Why you wanted it — always present (the saved Note, else the fallback).
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Why you wanted it",
		Value:  whyWanted(hit),
		Inline: false,
	})
	// For <char> — DISPLAY-ONLY (CWANT-05), present ONLY when the matched want is
	// character-tagged (CharacterName non-nil) AND the name is non-blank (the OMIT-when-
	// empty idiom the Price/Seen/Seller fields use). This NEVER touches the send path or
	// DiscordUserID — naming the character does not change who receives the DM (T-28-06).
	// The value is a PLAIN-TEXT embed field (character.name is constrained in-game text);
	// it is NOT interpolated into a URL or markdown-evaluated for mentions.
	if hit.CharacterName != nil && strings.TrimSpace(*hit.CharacterName) != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "For",
			Value:  *hit.CharacterName,
			Inline: true,
		})
	}

	return &discordgo.MessageEmbed{
		Title:  hit.ItemName + " — WTS",
		URL:    wikiURLFor(hit.ItemName),
		Color:  embedColor,
		Fields: fields,
		Footer: &discordgo.MessageEmbedFooter{Text: "EC-tunnel auction · SquireBot"},
	}
}

// buildDetail is the short inbox-row summary stored in alert_log.detail (Alert.Detail).
// It mirrors the embed essentials in one line ("~2000pp · seen ~3 min ago"),
// omitting the price clause when unknown and the seen clause when unparseable.
func buildDetail(a enrich.ItemAuctionDetail, seenStr string) string {
	parts := make([]string, 0, 2)
	if a.P != nil {
		parts = append(parts, fmt.Sprintf("~%d pp", *a.P))
	}
	if seenStr != "" {
		parts = append(parts, "seen "+seenStr)
	}
	if len(parts) == 0 {
		return "WTS in EC"
	}
	return strings.Join(parts, " · ")
}
