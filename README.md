blackbox
========

Blackbox is a simple yaml-driven http testing tool built on go's included testing tools.

- Define tests in YAML.
- YAML can use [go templates](https://golang.org/pkg/text/template/) with [Sprig functions](http://masterminds.github.io/sprig/)

Usage
-----

By default, `blackbox` looks for files named `test*.yaml` in the current directory. You may specify as many files you like. You may also read from stdin, `cat test_example.yaml | blackbox -- -`.

```bash
Usage of ./blackbox [files...]
  ... # lots of `go test` flags elided
  -test.v
        verbose: print additional output
  -wait-extra int
        Seconds to wait regardless of -wait-for-url status
  -wait-for-url string
        Wait for this url to become available (status 200)
```

A heavily annotated example of all the options. Also see the [test_example.yaml](test_example.yaml).

```yaml
# The top level object is an array. You may have many tests or just one.
- name: Test 1

  # The URL to test. You can use env vars easily.
  url: http://httpbin.org/get?home={{env "HOME" | urlencode }}

  # Add headers
  headers:
    host: publicname.example.org
    authorization: Bearer ABC123

  # Add basic auth
  basicAuth: ["username", "password"]

  # If the the response is a 3xx, follow to the new Location. By default this
  # is false and probably what you want for testing.
  followRedirects: true

  # And now some expectations for the response.
  expect:
    # Check the HTTP status code
    status: 200

    # Check the body. All specified must match!
    body:
      # Match this string exactly. New lines and whitespace too.
      content: "Hello, World!"

      # Since "content: ''" would appear the same as if you didn't specify the
      # "content" key, set "empty" to true to expect an empty body.
      empty: true

      # Match a regular expression. See https://github.com/google/re2/wiki/Syntax
      regex: '^Hello'

    # Header values must match exactly
    headers:
      content-type: text/plain
```

Additional Template Functions
-----------------------------

### urlencode

Encode strings to be used safely as part of a url.

```
# Will evaluate to "this%2Fthat"
{{ this/that | urlencode }}
```

Docker-compose Example
----------------------

```
version: "3.4"

services:
  httpbin:
    image: mccutchen/go-httpbin

  blackbox:
    image: busybox
    command:
      - blackbox
      - -test.v
      - -wait-for-url
      - http://httpbin:8080/get
      - -wait-extra
      - "2"
    environment:
      - HOST=httpbin:8080
    workdir: /tests
    volumes:
      - blackbox-linux-amd64:/usr/local/bin/blackbox
      - ./:/tests
```

Now run your tests in that environment. `--abort-on-container-exit` will make the containers shut down when `blackbox` exits.

```bash
docker-compose up --abort-on-container-exit
```

Future Features
---------------

- [ ] JSON Schema matching (expect.body.jsonSchema)
- [ ] Use another request response as a template to match (expect.fromRequest)


Similar Projects
----------------

- [pyresttest](https://github.com/svanoort/pyresttest)
- [tavern](https://github.com/taverntesting/tavern)
