package netfs

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"path"
	"strings"
	"time"

	smb2 "github.com/hirochachacha/go-smb2"
)

const smbDialTimeout = 10 * time.Second

type smbBackend struct {
	conn    net.Conn
	session *smb2.Session
	share   *smb2.Share
	dir     string // path below the share, slash-separated, may be ""
}

func dialSMB(loc Location, creds Credentials) (backend, error) {
	conn, err := net.DialTimeout("tcp", hostWithDefaultPort(loc.Host, "445"), smbDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("netfs: smb dial %s: %w", loc.Host, err)
	}
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{User: creds.Username, Password: creds.Password},
	}
	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("netfs: smb login to %s (user %q): %w", loc.Host, creds.Username, err)
	}
	share, err := session.Mount(loc.Share)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, fmt.Errorf("netfs: smb mount //%s/%s: %w", loc.Host, loc.Share, err)
	}
	return &smbBackend{conn: conn, session: session, share: share, dir: loc.Dir}, nil
}

func (b *smbBackend) fs() fs.FS { return b.share.DirFS(b.dir) }

func (b *smbBackend) open(rel string) (io.ReadSeekCloser, int64, error) {
	name := strings.ReplaceAll(path.Join(b.dir, rel), "/", `\`)
	f, err := b.share.Open(name)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (b *smbBackend) close() error {
	b.share.Umount()
	b.session.Logoff()
	return b.conn.Close()
}

func hostWithDefaultPort(host, port string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, port)
}
