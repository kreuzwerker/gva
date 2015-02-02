package gva

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	SecretFile    = "/tmp/secret.txt"
	Secret        = "opensesame\n"
	WithSecret    = true
	WithoutSecret = false
)

var varnishd string

func init() {

	var err error

	varnishd, err = exec.LookPath("varnishd")

	if err != nil {
		panic("Cannot find varnishd")
	}

}

func run(argv ...string) (func() error, error) {

	cmd := exec.Command(argv[0], argv[1:len(argv)]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		cmd.Wait()
	}()

	time.Sleep(1 * time.Second)

	return func() error {
		return cmd.Process.Kill()
	}, nil

}

func varnish(t *testing.T, port uint16, secret bool) func() error {

	assert := assert.New(t)

	if secret {
		err := ioutil.WriteFile(SecretFile, []byte(Secret), 0644)
		assert.NoError(err)
	}

	var uuid = make([]byte, 20)
	rand.Read(uuid)

	args := []string{
		varnishd,
		"-n",
		hex.EncodeToString(uuid),
		"-F",
		"-a",
		fmt.Sprintf("127.0.0.1:%d", port),
		"-b",
		"127.0.0.1",
		"-T",
		fmt.Sprintf("127.0.0.1:%d", port+1),
		"-S",
	}

	if secret {
		args = append(args, SecretFile)
	} else {
		args = append(args, "")
	}

	stop, err := run(args...)
	assert.NoError(err)

	return stop

}

func TestAuth(t *testing.T) {

	t.Parallel()

	var port uint16 = 10000

	stop := varnish(t, port, WithSecret)
	defer stop()

	assert := assert.New(t)

	conn, err := NewConnection("127.0.0.1", port+1, nil)
	assert.Error(err)
	assert.NoError(conn.Close())

	secret := "wrongsecret"

	conn, err = NewConnection("127.0.0.1", port+1, &secret)
	assert.Error(err)
	assert.NoError(conn.Close())

	secret = Secret

	conn, err = NewConnection("127.0.0.1", port+1, &secret)
	assert.NoError(err)

	resp, err := conn.Cmd("ping")
	assert.NoError(err)
	assert.True(resp.IsSuccess())
	assert.Regexp(`^PONG \d+ 1.0$`, resp.Body)

	assert.NoError(conn.Close())

}

func TestNoAuth(t *testing.T) {

	t.Parallel()

	var port uint16 = 20000

	stop := varnish(t, port, WithoutSecret)
	defer stop()

	assert := assert.New(t)

	secret := "somesecret"

	conn, err := NewConnection("127.0.0.1", port+1, &secret)
	assert.Error(err)
	assert.NoError(conn.Close())

	conn, err = NewConnection("127.0.0.1", port+1, nil)
	assert.NoError(err)

	resp, err := conn.Cmd("ping")
	assert.NoError(err)
	assert.True(resp.IsSuccess())
	assert.Regexp(`^PONG \d+ 1.0$`, resp.Body)

	assert.NoError(conn.Close())

}

func TestInlineVLC(t *testing.T) {

	t.Parallel()

	var port uint16 = 30000

	stop := varnish(t, port, WithoutSecret)
	defer stop()

	assert := assert.New(t)

	conn, err := NewConnection("127.0.0.1", port+1, nil)
	assert.NoError(err)

	tpl := `<< VCL
vcl 4.0;

backend default {
  .host = "127.0.0.1";
  .port = "12345";
}

sub vcl_recv {
	return (synth(999, ""));
}

sub vcl_synth {
	if (resp.status == 999) {
  	set resp.status = 200;
  	set resp.http.Content-Type = "text/plain; charset=utf-8";
    synthetic("hello world");
  	return (deliver);
	}
}
VCL`

	resp, err := conn.Cmd("vcl.inline", "hello", tpl)
	assert.NoError(err)
	assert.True(resp.IsSuccess())

	resp, err = conn.Cmd("vcl.use", "hello")
	assert.NoError(err)
	assert.True(resp.IsSuccess())

	time.Sleep(1 * time.Second)

	cresp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	assert.NoError(err)

	body, _ := ioutil.ReadAll(cresp.Body)

	assert.Equal("hello world", string(body))
	assert.NoError(conn.Close())

}

func TestKeepalive(t *testing.T) {

	t.Parallel()

	var port uint16 = 40000

	stop := varnish(t, port, WithoutSecret)
	defer stop()

	assert := assert.New(t)

	conn, err := NewConnection("127.0.0.1", port+1, nil)
	assert.NoError(err)

	conn.Keepalive(1 * time.Second)

	time.Sleep(5 * time.Second)

	assert.NoError(conn.Close())

}
