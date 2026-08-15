package netfs

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		spec    string
		want    Location
		wantErr bool
	}{
		{spec: "/media/tv/My Show", want: Location{Protocol: ProtocolLocal, Dir: "/media/tv/My Show"}},
		{spec: "smb://nas/media/tv/My Show", want: Location{Protocol: ProtocolSMB, Host: "nas", Share: "media", Dir: "tv/My Show"}},
		{spec: "smb://nas:1445/media", want: Location{Protocol: ProtocolSMB, Host: "nas:1445", Share: "media", Dir: ""}},
		{spec: "smb://nas", wantErr: true}, // no share
		{spec: "nfs://nas/volume1/media/Shows", want: Location{Protocol: ProtocolNFS, Host: "nas", Dir: "/volume1/media/Shows"}},
		{spec: "nfs://nas:2049/export/", want: Location{Protocol: ProtocolNFS, Host: "nas:2049", Dir: "/export"}},
		{spec: "nfs://nas", wantErr: true}, // no path
	}
	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			got, err := Parse(c.spec)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestLocationSpecRoundTrips(t *testing.T) {
	cases := []struct {
		folder, rel, want string
	}{
		{"/media/tv/Show", "Season 1/e1.mkv", "/media/tv/Show/Season 1/e1.mkv"},
		{"smb://nas/media/tv/Show", "Season 1/e1.mkv", "smb://nas/media/tv/Show/Season 1/e1.mkv"},
		{"smb://nas/media", "e1.mkv", "smb://nas/media/e1.mkv"},
		{"nfs://nas/vol/Show", "Season 1/e1.mkv", "nfs://nas/vol/Show/Season 1/e1.mkv"},
	}
	for _, c := range cases {
		loc, err := Parse(c.folder)
		if err != nil {
			t.Fatalf("parse %s: %v", c.folder, err)
		}
		if got := loc.Spec(c.rel); got != c.want {
			t.Errorf("Spec(%q, %q) = %q, want %q", c.folder, c.rel, got, c.want)
		}
		// An item spec must re-parse to the same location family, so the
		// bridge can open what Scan stored.
		if _, err := Parse(loc.Spec(c.rel)); err != nil {
			t.Errorf("item spec %q does not re-parse: %v", loc.Spec(c.rel), err)
		}
	}
}
