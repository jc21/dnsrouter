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

The command is able to write it's default configuration and exit:

```bash
./dnsrouter -w
# optionally specify the file to write
./dnsrouter -w -c /path/to/config.json
```

Then it's up to you to edit this file to your liking. The default location is `/etc/dnsrouter/config.json`

Refer to the `config.json.example` file for upstream routing examples.

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
