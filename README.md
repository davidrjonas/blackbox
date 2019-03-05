blackbox
========

Blackbox is a simple yaml-driven http testing tool built on go's included testing tools.

- Define tests in YAML.
- YAML can use [go templates](https://golang.org/pkg/text/template/) with [Sprig functions](http://masterminds.github.io/sprig/)
- .env support

Usage
-----

By default, `blackbox` looks for files named `test*.yaml` in the current directory. You may specify as many files you like. You may also read from stdin, `cat test_example.yaml | blackbox -- -`.

```bash
Usage of ./blackbox [files...]
  # ... lots of `go test` flags elided. Just the useful ones for us included here.
  -test.failfast
        do not start new tests after the first test failure
  -test.run regexp
        run only tests and examples matching regexp
  -test.v
        verbose: print additional output
  -wait-extra int
        Seconds to wait regardless of -wait-for-url status [env: BLACKBOX_WAIT_EXTRA]
  -wait-for-url string
        Wait for this url to become available (status 200) [env: BLACKBOX_WAIT_FOR_URL]
```

Run only select tests

```bash
blackbox -test.v -test.run '.*/Test_POST_json'
```

A heavily annotated example of all the options. Also see the [test_example.yaml](test_example.yaml).

```yaml
# The top level object is an array. You may have many tests or just one.
- name: Test 1

  # The URL to test. You can use env vars easily.
  url: http://httpbin.org/get?home={{env "HOME" | urlencode }}

  # Defafults to GET unless data is set, then POST
  method: GET

  # Setting data automatically makes the request a POST and sets the
  # content-type to application/x-www-form-urlencoded if it is not already set.
  # If you want json, for now you need to set the header.
  data:
    content: example=test

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

    # Okay, this is cool. Use another request as a prototype against which to
    # compare. When testing the new version of an API you can compare against the
    # old version.
    fromRequest:
      # All the same options as the top level request
      url: http://httpbin.org/get

      # There is one that isn't at the top level, headerWhitelist. These are
      # the headers that will be compared. We can't compare all headers because
      # many change per request, such as Date:
      headerWhitelist:
        - content-type
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
    image: davidrjonas/blackbox
    environment:
      # In your tests use HOST like `url: http://{{env "HOST"}}/get`
      - HOST=httpbin:8080
      - BLACKBOX_WAIT_FOR_URL=http://httpbin:8080/get
      - BLACKBOX_WAIT_EXTRA=2
    volumes:
      - ./:/tests
```

Now run your tests in that environment. `--abort-on-container-exit` will make the containers shut down when `blackbox` exits.

```bash
docker-compose up --abort-on-container-exit
```

Build From Source
-----------------

The build is a little odd since we want to run `TestMain()` and use the built-in test framework which is normally stripped from a binary. Luckily, `go test` has an escape hatch for building a test binary.

Regardless, [Makefile](Makefile) has everything you need.

```bash
make blackbox
```

Cross-compile with the usual `GOOS` and `GOARCH`

```bash
GOOS=linux GOARCH=arm64 make blackbox
# outputs blackbox-linux-arm64
```

Future Features
---------------

- [ ] JSON Schema matching (expect.body.jsonSchema)
- [X] Use another request response as a template to match (expect.fromRequest)


Similar Projects
----------------

- [pyresttest](https://github.com/svanoort/pyresttest)
- [tavern](https://github.com/taverntesting/tavern)
