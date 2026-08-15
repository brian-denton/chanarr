package guide

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"chanarr/internal/library"
	"chanarr/internal/schedule"
)

// timeLayout matches Plex's expected XMLTV datetime format, always in UTC —
// e.g. "20260815143000 +0000" (research-plex-guide-xmltv.md §"Datetime format").
const timeLayout = "20060102150405 -0700"

// ChannelSchedule pairs a Channel with the Epoch to render its guide from.
// The guide always uses the epoch active *now* — a rolling horizon never
// needs to resolve historical epochs, only the current one (spec.md §5).
type ChannelSchedule struct {
	Channel schedule.Channel
	Epoch   schedule.Epoch
}

// EpochProvider supplies the channels to render into the guide. TODO:
// implement against internal/store once channel persistence exists.
type EpochProvider func() ([]ChannelSchedule, error)

type xmltvDoc struct {
	XMLName           xml.Name         `xml:"tv"`
	GeneratorInfoName string           `xml:"generator-info-name,attr"`
	Channels          []xmltvChannel   `xml:"channel"`
	Programmes        []xmltvProgramme `xml:"programme"`
}

type xmltvChannel struct {
	ID           string   `xml:"id,attr"`
	DisplayNames []string `xml:"display-name"`
}

type xmltvProgramme struct {
	Start           string            `xml:"start,attr"`
	Stop            string            `xml:"stop,attr"`
	Channel         string            `xml:"channel,attr"`
	Title           xmltvText         `xml:"title"`
	SubTitle        *xmltvText        `xml:"sub-title"`
	Desc            xmltvText         `xml:"desc"`
	Category        xmltvText         `xml:"category"`
	EpisodeNums     []xmltvEpisodeNum `xml:"episode-num,omitempty"`
	PreviouslyShown *struct{}         `xml:"previously-shown"`
}

type xmltvText struct {
	Lang  string `xml:"lang,attr"`
	Value string `xml:",chardata"`
}

type xmltvEpisodeNum struct {
	System string `xml:"system,attr"`
	Value  string `xml:",chardata"`
}

// BuildXMLTV renders the guide for the given channels, covering the window
// [now, now+HorizonHours). It is the pure counterpart to Handler — kept
// separate so guide generation can be tested without an HTTP round trip and
// without depending on time.Now().
//
// Per-channel, per-airing fields:
//   - <title> is always the channel name (one Channel = one show/folder,
//     per the scheduling model — spec.md §2).
//   - <sub-title> is the episode title cleaned out of the filename
//     (library.EpisodeTitle: text after the SxxExx marker, separators
//     normalized, release cruft stripped) — omitted entirely when nothing
//     readable remains, since a fabricated title is worse than letting the
//     episode-num speak for itself.
//   - <desc> is a plain "Show — S01E02 — Episode Title" line built from
//     whatever parsed, so Plex's details pane isn't empty. chanarr has no
//     synopsis source in v1 (online metadata deferred, spec.md §13).
//   - <episode-num> is emitted (both xmltv_ns and onscreen systems) only
//     when internal/library.ParseEpisode recognizes an SxxExx pattern in
//     the filename; omitted otherwise — an absent episode number is more
//     honest than a fabricated one, matching the <rating> decision.
//   - <category>Series</category> always; <rating> never (spec.md §5).
func BuildXMLTV(channels []ChannelSchedule, now time.Time) ([]byte, error) {
	now = now.UTC()
	horizon := now.Add(HorizonHours * time.Hour)

	doc := xmltvDoc{GeneratorInfoName: "chanarr"}

	for _, cs := range channels {
		doc.Channels = append(doc.Channels, xmltvChannel{
			ID: cs.Channel.Number,
			DisplayNames: []string{
				cs.Channel.Number + " " + cs.Channel.Name,
				cs.Channel.Number,
				cs.Channel.Name,
			},
		})

		for at := now; at.Before(horizon); {
			airing, err := schedule.ProgramAt(cs.Epoch, at)
			if err != nil {
				return nil, fmt.Errorf("guide: channel %s: %w", cs.Channel.Number, err)
			}
			doc.Programmes = append(doc.Programmes, buildProgramme(cs.Channel, airing))
			at = airing.End
		}
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("guide: marshal xmltv: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

func buildProgramme(ch schedule.Channel, airing schedule.Airing) xmltvProgramme {
	filename := filepath.Base(airing.Item.Path)
	episodeTitle := library.EpisodeTitle(filename)

	p := xmltvProgramme{
		Start:           airing.Start.UTC().Format(timeLayout),
		Stop:            airing.End.UTC().Format(timeLayout),
		Channel:         ch.Number,
		Title:           xmltvText{Lang: "en", Value: ch.Name},
		Category:        xmltvText{Lang: "en", Value: "Series"},
		PreviouslyShown: &struct{}{},
	}
	if episodeTitle != "" {
		p.SubTitle = &xmltvText{Lang: "en", Value: episodeTitle}
	}

	descParts := []string{ch.Name}
	if season, episode, ok := library.ParseEpisode(filename); ok {
		p.EpisodeNums = []xmltvEpisodeNum{
			{System: "onscreen", Value: fmt.Sprintf("S%02dE%02d", season, episode)},
			{System: "xmltv_ns", Value: fmt.Sprintf("%d.%d.0/1", season-1, episode-1)},
		}
		descParts = append(descParts, fmt.Sprintf("S%02dE%02d", season, episode))
	}
	if episodeTitle != "" && episodeTitle != ch.Name {
		descParts = append(descParts, episodeTitle)
	}
	p.Desc = xmltvText{Lang: "en", Value: strings.Join(descParts, " — ")}

	return p
}
