# dnsrouter

A simplistic dns daemon that you can use as your local DNS server
and have it route DNS requests to upstream servers based on the
requested domain.

I've created this so that I can effectively set up split DNS
(sometimes called split horizon DNS) such that my setup will
properly forward DNS requests through a VPN connection only when
the domain being queried is told to do so.

For example:
- I have a VPN connection to my office
- This office has private DNS hostnames like `intranet.myoffice.lan`
- I don't want all of my DNS requests to go through the VPN, only the office ones
- I setup a regex in dnsrouter config for `*.myoffice\.lan` so that it forwards any DNS
query to the office VPN `10.0.0.1`
- I keep my default DNS server `1.1.1.1` for all other queries
- I run `dnsrouter` locally
- I tell my machine to use localhost as the DNS server
- I profit

## Configuration

Copy the `config.json.example` file and adjust to your needs.

Place this file anywhere you want, the default location is
`/etc/dnsrouter/config.json`

When running the `dnsrouter` binary, you can optionally specify the
location of the configuration file like

```bash
./dnsrouter -c /path/to/config.json
```

## Building

```bash
git clone https://github.com/jc21/dnsrouter.git
cd dnsrouter
BUILD_COMMIT=$(git rev-parse --short HEAD) \
BUILD_VERSION=0.0.1 \
./scripts/build.sh
```

Binary will output to `bin/dnsrouter`

## Running

Be aware that running on port 53 will require root permissions on Linux systems.
