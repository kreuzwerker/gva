package gva

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const ( // see include/vcli.h in varnish source
	CLIS_SYNTAX    = 100
	CLIS_UNKNOWN   = 101
	CLIS_UNIMPL    = 102
	CLIS_TOOFEW    = 104
	CLIS_TOOMANY   = 105
	CLIS_PARAM     = 106
	CLIS_AUTH      = 107
	CLIS_OK        = 200
	CLIS_TRUNCATED = 201
	CLIS_CANT      = 300
	CLIS_COMMS     = 400
	CLIS_CLOSE     = 500
)

var Debug bool

type Connection struct {
	sync.Mutex
	conn net.Conn
	stop func()
}

type Response struct {
	Status int
	Body   string
}

func (r *Response) IsSuccess() bool {
	return r.Status == CLIS_OK
}

// NewConnection initializes a new connection to a given varnish administration interface. If the
// optional secret is given, it tries to authenticate using S/PSK authentication.
func NewConnection(host string, port uint16, secret *string) (*Connection, error) {

	remote := fmt.Sprintf("%s:%d", host, port)

	if Debug {
		log.Printf("Connecting to %s", remote)
	}

	if conn, err := net.DialTimeout("tcp", remote, 5*time.Second); err != nil {
		return nil, err
	} else {

		c := &Connection{conn: conn}

		return c, c.auth(secret)

	}

}

// auth handles the S/PSK authentication to varnish
// see https://www.varnish-cache.org/docs/trunk/reference/varnish-cli.html
func (c *Connection) auth(secret *string) error {

	resp, err := c.read()

	if err != nil {
		return err
	}

	if secret != nil {

		if resp.Status == CLIS_AUTH {

			challenge := strings.Split(resp.Body, "\n")[0]

			response := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s%s\n", challenge, *secret, challenge)))

			resp, err := c.Cmd("auth", hex.EncodeToString(response[:]))

			if err != nil {
				return err
			}

			if !resp.IsSuccess() {
				return fmt.Errorf("authentication failed with %d", resp.Status)
			}

			return nil

		} else {
			return fmt.Errorf("secret was passed, but status %d was returned (expected CLIS_AUTH)", resp.Status)
		}

	} else {

		if !resp.IsSuccess() {
			return fmt.Errorf("secret was not passed, but status %d was returned (expected CLIS_OK)", resp.Status)
		} else {
			return nil
		}

	}

}

// Keepalive installs a keep-alive timer which periodically sends a ping to Varnish
func (c *Connection) Keepalive(interval time.Duration) {

	ticker := time.NewTicker(interval)
	quit := make(chan struct{})

	go func() {

		for {

			select {
			case <-ticker.C:
				c.Cmd("ping")
			case <-quit:
				ticker.Stop()
				return
			}

		}

	}()

	c.stop = func() {
		ticker.Stop()
		close(quit)
	}

}

// Close will stop all keepalive timers and close the underlying tcp connection
func (c *Connection) Close() error {

	if c.stop != nil {
		c.stop()
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil

}

// Cmd writes a command and reads the response from varnish. Cmd can used
// concurrently.
func (c *Connection) Cmd(command string, args ...string) (*Response, error) {

	c.Lock()
	defer c.Unlock()

	if err := c.write(command, args...); err != nil {
		return nil, err
	}

	return c.read()

}

// read reads a response from varnish
func (c *Connection) read() (*Response, error) {

	var status, length int

	headers, err := fmt.Fscanf(c.conn, "%03d %8d\n", &status, &length)

	if err != nil {
		return nil, err
	}

	if headers != 2 {
		return nil, fmt.Errorf("read %d headers, expected 2", headers)
	}

	buf := make([]byte, length+1)

	body, err := c.conn.Read(buf)

	if err != nil {
		return nil, err
	}

	if body != cap(buf) {
		fmt.Errorf("read %d body bytes, expected %d", body, cap(buf))
	}

	resp := &Response{
		status, string(buf[0 : len(buf)-1]),
	}

	if Debug {
		log.Printf(" read %v", resp)
	}

	return resp, nil

}

// write sends a command to varnish
func (c *Connection) write(command string, args ...string) error {

	var buf = append([]string{command}, args...)

	body := fmt.Sprintf("%s\n", strings.Join(buf, " "))

	if Debug {
		log.Printf("write %v", body)
	}

	_, err := c.conn.Write([]byte(body))

	return err

}
