package redis

import (
	"bufio"
	"net"
	"time"
)

const defaultBufSize = 4096

var noTimeout = time.Time{}

// Stubbed in tests.
var now = time.Now

type conn struct {
	netcn net.Conn
	rd    *bufio.Reader
	buf   []byte

	UsedAt       time.Time
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func newConnDialer(opt *Options) func() (*conn, error) {
	dialer := opt.getDialer()
	return func() (*conn, error) {
		netcn, err := dialer()
		if err != nil {
			return nil, err
		}
		cn := &conn{
			netcn: netcn,
			buf:   make([]byte, defaultBufSize),

			UsedAt: now(),
		}
		cn.rd = bufio.NewReader(cn)
		return cn, cn.init(opt)
	}
}

func (cn *conn) init(opt *Options) error {
	if opt.Password == "" && opt.DB == 0 {
		return nil
	}

	// Temp client for Auth and Select.
	client := newClient(opt, newSingleConnPool(cn))

	if opt.Password != "" {
		if err := client.Auth(opt.Password).Err(); err != nil {
			return err
		}
	}

	if opt.DB > 0 {
		if err := client.Select(opt.DB).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (cn *conn) writeCmds(cmds ...Cmder) error {
	cn.buf = cn.buf[:0]
	for _, cmd := range cmds {
		var err error
		cn.buf, err = appendArgs(cn.buf, cmd.args())
		if err != nil {
			return err
		}
	}

	_, err := cn.Write(cn.buf)
	return err
}

func (cn *conn) Read(b []byte) (int, error) {
	cn.UsedAt = now()
	if cn.ReadTimeout != 0 {
		cn.netcn.SetReadDeadline(cn.UsedAt.Add(cn.ReadTimeout))
	} else {
		cn.netcn.SetReadDeadline(noTimeout)
	}
	return cn.netcn.Read(b)
}

func (cn *conn) Write(b []byte) (int, error) {
	cn.UsedAt = now()
	if cn.WriteTimeout != 0 {
		cn.netcn.SetWriteDeadline(cn.UsedAt.Add(cn.WriteTimeout))
	} else {
		cn.netcn.SetWriteDeadline(noTimeout)
	}
	return cn.netcn.Write(b)
}

func (cn *conn) RemoteAddr() net.Addr {
	return cn.netcn.RemoteAddr()
}

func (cn *conn) Close() error {
	return cn.netcn.Close()
}

func isSameSlice(s1, s2 []byte) bool {
	return len(s1) > 0 && len(s2) > 0 && &s1[0] == &s2[0]
}

func (cn *conn) copyBuf(b []byte) []byte {
	if isSameSlice(b, cn.buf) {
		new := make([]byte, len(b))
		copy(new, b)
		return new
	}
	return b
}
