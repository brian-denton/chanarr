# chanarr

Turns folders of local media into looping virtual-timeline TV channels that Plex detects via HDHomeRun tuner emulation.

## Language

**Channel**:
A configured source folder plus playback settings (shuffle on/off, shuffle seed), exposed to Plex as one tuner lineup entry with its own guide.
_Avoid_: Station, feed

**Epoch**:
An immutable, timestamped snapshot of a Channel's Playlist — its ordered Playlist Items, their cached durations, and (if shuffled) their shuffle order — created whenever the underlying folder's contents or order change. `epochStart` anchors all position math for timestamps within that epoch; an epoch never mutates once created, and adding, removing, or reordering files always creates a new one rather than editing the current one.
_Avoid_: Schedule, version, generation

**Playlist Item**:
One media file's membership within an Epoch: the file plus its cached duration and its position in the epoch's order.
_Avoid_: Program (see Airing), track, entry

**Airing**:
The result of evaluating a Channel's timeline at a specific instant `t`: a Playlist Item paired with the concrete start and end time it occupies for that query. The unit both the stream (to pick the file and compute a seek offset) and the guide (to render a listing) consume — deliberately not called "Program" to avoid colliding with "program" as in TV-show-the-concept.
_Avoid_: Program, showing, slot

**ProgramAt**:
The single pure function `ProgramAt(channel, t) → Airing` that both the streamer (point query at tune-in) and the XMLTV guide writer (range query, implemented as repeated point queries walking forward to each Airing's end time) call. This is the mechanism that keeps the live stream and the published guide from ever disagreeing about what's on.
_Avoid_: Scheduler, timeline resolver
