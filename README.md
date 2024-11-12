# dnsrouter

A simplistic dns daemon that you can use as your local DNS server
and have it route DNS requests to upstream servers based on the
requested domain.

I've created this so that I can effectively set up split DNS
(sometimes called split horizon DNS) such that my setup will
properly forward DNS requests through a VPN connection only when
the domain being queried is told to do so.

Why? Split DNS is apparently supported by common DNS server software but
my experience is that it's not implemented in the way I expect. For
example `dnsmasq` will send DNS requests to all servers regardless
of the rules and when an upstream DNS server is dropping connections
it will hang the whole server.

This service can also answer with internal DNS entries, avoiding the
need for host file modifications.

## Installation

Only the following methods are available for installation. For all other systems, see Building below.

### MacOS Homebrew

Homebrew won't accept this package until it has higher reputation (stars, forks) so until then
you'll have to use the development Formula and rebuild it to install it:

```bash
export HOMEBREW_NO_INSTALL_FROM_API=1
brew update
brew tap --force homebrew/core
cd $(brew --repo homebrew/core)
git remote add jc21 https://github.com/jc21/homebrew-core.git
git fetch --all
git checkout jc21/dnsrouter
brew install --build-from-source dnsrouter
# and if you need to rebuild:
brew reinstall --build-from-source dnsrouter

# Edit config which is located at:
# /opt/homebrew/etc/dnsrouter/config.json

sudo brew services start dnsrouter

# Switch back to homebrew-core master
git checkout master
```

### RHEL based distros

RPM's are built [here](https://github.com/jc21-rpm/dnsrouter) and hosted [here](https://yum.jc21.com).

```bash
sudo yum localinstall https://yum.jc21.com/jc21.rpm
sudo yum install dnsrouter

# Edit config which is located at:
# /etc/dnsrouter/config.json

sudo systemctl enable dnsrouter --now
```

## Configuration

The command is able to write it's default configuration and exit:

```bash
./dnsrouter -w
# optionally specify the file to write
./dnsrouter -w -c /path/to/config.json
```

Then it's up to you to edit this file to your liking. The default location is `/etc/dnsrouter/config.json`

Refer to the `config.json.example` file for upstream routing examples.

### Examples

Given the following configuration:

```json
{
  "servers": [
    {
      "host": "127.0.0.1",
      "port": 53,
      "default_upstream": "1.1.1.1",
      "internal": [
        {
          "regex": "mail.example.com",
          "A": "192.168.0.10",
          "AAAA": "2001:db8::1234:5678",
          "TXT": "omgyesitworked",
          "MX": "10 mailserver1.example.com.\n20 mailserver2.example.com."
        }
      ],
      "upstreams": [
        {
          "regex": "local",
          "nxdomain": true
        },
        {
          "regex": ".*\\.example.com",
          "upstream": "8.8.8.8"
        },
        {
          "regex": ".*\\.localdomain",
          "upstream": "10.0.0.1"
        },
        {
          "regex": ".*\\.(office\\.lan|myoffice\\.com)",
          "upstream": "10.0.0.1"
        }
      ]
    }
  ]
}
```

*Requesting DNS for `test.example.com`*

1. DNS client connects to `dnsrouter` and asks for `test.example.com`
2. `dnsrouter` matches with the 2nd _upstream_ rule
3. `dnsrouter` forwards the DNS question to upstream DNS server `8.8.8.8`
4. `dnsrouter` returns the answer to the DNS client

*Requesting DNS for `google.com`*

1. DNS client connects to `dnsrouter` and asks for `google.com`
2. `dnsrouter` does not match with any defined rules
3. `dnsrouter` forwards the DNS question to default upstream DNS server `1.1.1.1`
4. `dnsrouter` returns the answer to the DNS client

*Requesting DNS for `local`*

1. DNS client connects to `dnsrouter` and asks for `local`
2. `dnsrouter` matches with the 1st _upstream_ rule
3. `dnsrouter` returns an error to the client with NXDOMAIN

*Requesting DNS for `myoffice.com`*

1. DNS client connects to `dnsrouter` and asks for `myoffice.com`
2. `dnsrouter` does not match with any defined rules
3. `dnsrouter` forwards the DNS question to default upstream DNS server `1.1.1.1`
4. `dnsrouter` returns the answer to the DNS client

_Note: This is a trick example. The domain matching regex will match `*.myoffice.com` but not `myoffice.com`_

*Requesting DNS for `mail.example.com`*

1. DNS client connects to `dnsrouter` and asks for `mail.example.com`
2. `dnsrouter` matches with the 1st _internal_ rule
3. `dnsrouter` returns the answer value to the DNS client with the A/AAAA/MX/TXT record as requested

## Building

```bash
git clone https://github.com/jc21/dnsrouter.git
cd dnsrouter
./scripts/build.sh
```

Binary will output to `bin/dnsrouter`

## Running

```bash
./dnsrouter
# optionally specify the file to read
./dnsrouter -c /path/to/config.json
```

Be aware that running on port `53` will require root permissions on Linux systems.

After the service is running you just have to use it. Modify your network interface's DNS
servers (or /etc/resolv.conf) to use the IP running `dnsrouter` ie `127.0.0.1` if it's
the same machine.

You may choose to run in verbose mode by specifying `-v` this will output each incoming
DNS request and the determined forwarding DNS server.

### Multiple Servers

The configuration allows you to run multiple DNS servers on different interfaces and ports
and have different rules for them.

### Additional Notes

1. Cache, if enabled, won't respect the TTL of the record, but is controlled internally
2. Regex's are prefixed with `^` and appended with `$` so there is no need to add them
3. Performance on a desktop used heavily appears to be great. Has not been tested for an entire office.
