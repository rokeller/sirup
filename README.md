# sirup

sirup is a simple reverse proxy written in go. The name _sirup_ is derived from
*Si*mple *R*everse *P*roxy or *SiRP* and pronounced like _sirup_. I've made
_sirup_ because I needed it, and everything else I've considered, including
things like nginx, traefik, etc. had too big a footprint, are too difficult to
configure for my needs and taste, and because I wanted to make it.

Use it at your own peril.

## What it does

_sirup_ takes a list of host names (FQDNs) mapped to target base URLs by appending
the local path and query to the target URL, and creating new requests for the
resulting URLs, for which the responses are proxied back to the caller of
_sirup_. All request and response headers, as well as request and response
bodies are copied too. For example:

Mapping `from.host.a` to `http://to.host.b/` forwards requests as follows:

- `GET http://from.host.a/path/to/resource?version=123` is forwarded as
  `GET http://to.host.b/path/to/resource?version=123`
- `POST https://from.host.a/?nothing` is forwarded as
  `POST http://to.host.b/?nothing`.
  > **Note**: The `https` request is downgraded to an `http` request because
  that's how the target is configured. Be careful what you configure.

Mapping `foo.bar.com` to `https://bar.baz.org/sub-path` forwards requests as
follows:

- `GET http://foo.bar.com/path/to/resource?version=123` is forwarded as
  `GET https://bar.baz.org/sub-path/path/to/resource?version=123`
  > **Note**: All forwarded requests are always sent with `https` because that's
  how the target is configured. Be careful what you configure because sensitive
  data that is sent back by the target server could be sniffed when sent to the
  caller of the _sirup_ exposed host.
- `POST https://foo.bar.com/?nothing` is forwarded as
  `https://bar.baz.org/sub-path/?nothing`.

It is important to note that _sirup_ treats the targets `https://foo.com/a` and
`https://foo.com/a/` the same way, that is to say that a trailing slash is not
necessary / makes no difference.

## Configuration

_sirup_ expects a configuration file, which by default is looked for with the
file name `config.yaml` in the current working directory. In the container image,
that is the `/app` directory, therefore _sirup_ looks for `/app/config.yaml`.

You can change the path that _sirup_ uses for the config file by specifying the
`-c <config-path>` flag:

```bash
sirup -c /path/to/config.yaml
```

The config file itself is YAML formatted and expects a single property `mapping`
that defines which incoming host names are mapped to which target URLs. The
following example configures the mappings mentioned above in
[What it does](#what-it-does)

```yaml
mapping:
  from.host.a: http://to.host.b/
  foo.bar.com: https://bar.baz.org/sub-path
```
