# chanarr walking skeleton — PROTOTYPE, throwaway

Answers [ticket 07](../../issues/07-prototype-walking-skeleton.md): does real Plex detect a minimal HDHomeRun emulation and play a looping remuxed stream?

## Run

```bash
cd .scratch/chanarr-spec/assets/walking-skeleton && go build -o chanarr-proto . && ./chanarr-proto
```

Serves on `:5004`: `/discover.json`, `/lineup.json`, `/lineup_status.json`, `/device.xml`, `/epg.xml`, `/stream/1` (spawns `ffmpeg -re -stream_loop -1 -f concat … -c copy -f mpegts` per tuner request, killed on disconnect). Content: two generated 30s episodes — moving test pattern @ 440 Hz tone, then SMPTE bars @ 880 Hz tone — looping forever.

## Plex test checklist (run from the Plex server's UI)

1. Plex Web → Settings → Live TV & DVR → Set Up Plex DVR
2. If no tuner tile appears, choose "Don't see your device? Enter its network address" → `192.168.1.148:5004` (quirk from research: no tile may render even after entry — Continue still works)
3. Channel scan shows "1 Chanarr Proto" → Continue
4. On the guide step choose "use an XMLTV guide" → `http://192.168.1.148:5004/epg.xml`
5. Finish setup; open Live TV → tune channel 1
6. Watch ≥70s: pattern→bars swap at ~30s (episode boundary), bars→pattern at ~60s (loop). Note any freeze/glitch/buffer at swaps, and whether guide shows "Chanarr Test Loop"
7. Record PMS version (Settings → General) — still owed to ticket 06
