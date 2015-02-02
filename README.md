# Golang Varnish admin 

[![Build Status](https://travis-ci.org/kreuzwerker/gva.svg?branch=master)](https://travis-ci.org/kreuzwerker/gva)

Golang Varnish admin (`gva`) is a Golang interface for the Varnish CLI. Is has been tested with Varnish 4.

## Example

```
package main

import "github.com/kreuzwerker/gva"

func main() {

	conn, err := gva.NewConnection("127.0.0.1", 10000, nil)

	if err != nil {
		panic(err)
	}

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

	if err != nil {
		panic(err)
	}

	if !resp.IsSuccess() {
		panic("That didn't work")
	}

}
```