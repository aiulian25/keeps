# SSRF adversarial test list (IMP-022 / R-15)

All cases live in `internal/safefetch`. Run with `go test ./internal/safefetch/`.
Every case must fail closed; a failure here is release-blocking.

Tests use `httptest.NewTLSServer`, a fake resolver (host -> address sequence) and two
unexported, test-only options (`withAllowedPortsForTests`, `withInsecureAllowLoopbackForTests`)
that cannot be set from outside the package.

## URL and host validation (no DNS, no network)

`TestAdversarialURLsFailClosedWithoutNetwork` (policy_test.go) checks each URL through both
`NormaliseURL` and `Fetcher.Get`, and asserts the resolver was never called.

| Case                               | Input                                        | Expected              |
| ---------------------------------- | -------------------------------------------- | --------------------- |
| localhost                          | `https://localhost/`                         | not_public            |
| localhost subdomain                | `https://shop.localhost/`                    | not_public            |
| single-label intranet host         | `https://intranet/`                          | not_public            |
| short IPv4                         | `https://127.1/`                             | invalid_url           |
| hex IPv4                           | `https://0x7f.1/`                            | invalid_url           |
| octal IPv4                         | `https://0177.0.0.1/`                        | invalid_url           |
| integer IPv4                       | `https://2130706433/`                        | invalid_url           |
| IPv4 with trailing dot             | `https://127.0.0.1./`                        | invalid_url           |
| IPv6 loopback                      | `https://[::1]/`                             | not_public            |
| IPv4-mapped loopback               | `https://[::ffff:127.0.0.1]/`                | not_public            |
| cloud metadata                     | `https://169.254.169.254/...`                | not_public            |
| private 10/8                       | `https://10.0.0.1/`                          | not_public            |
| CGNAT                              | `https://100.64.0.1/`                        | not_public            |
| IPv6 unique-local                  | `https://[fc00::1]/`                         | not_public            |
| NAT64 embedding loopback           | `https://[64:ff9b::7f00:1]/`                 | not_public            |
| 6to4 embedding loopback            | `https://[2002:7f00:1::]/`                   | not_public            |
| zoned link-local IPv6              | `https://[fe80::1%25eth0]/`                  | invalid_url           |
| http scheme                        | `http://example.com/`                        | invalid_url           |
| other scheme                       | `ftp://example.com/`                         | invalid_url           |
| non-443 port                       | `https://example.com:8443/`                  | invalid_url           |
| empty port                         | `https://example.com:/`                      | invalid_url           |
| userinfo                           | `https://user:pass@example.com/`             | invalid_url           |
| userinfo host confusion            | `https://example.com@10.0.0.1/`              | invalid_url           |
| empty host                         | `https:///path`                              | invalid_url           |
| backslash confusion                | `https://example.com\@10.0.0.1/`             | invalid_url           |
| whitespace                         | `https://exam ple.com/`                      | invalid_url           |
| non-ASCII host (punycode required) | `https://bücher.example/`                    | invalid_url           |
| opaque URL                         | `https:example.com`                          | invalid_url           |
| S3 pre-signed invoice              | `...?X-Amz-Credential=..&X-Amz-Signature=..` | private_document_link |
| S3 credential only                 | `...?x-amz-credential=a`                     | private_document_link |
| CloudFront signed                  | `...?Expires=..&Signature=..`                | private_document_link |
| Azure SAS                          | `...?se=..&sig=..`                           | private_document_link |
| invoice token                      | `/orders/invoice.pdf?token=..`               | private_document_link |
| invoice path with query            | `/account/invoice/42?download=1`             | private_document_link |

`TestNormaliseURLCanonicalises`: host lowercased, trailing dot and default port removed,
fragment stripped, query preserved.

## Address policy table

`TestIsPublicAddress` (policy_test.go) covers every row of `nonPublicPrefixes` plus
positives (8.8.8.8, 1.1.1.1, 2606:4700::1111, ::ffff:8.8.8.8): loopback, 0/8, 10/8,
172.16/12 (both ends), 192.168/16, 169.254/16, 100.64/10, 192.0.0/24, 192.0.2/24,
198.18/15, 198.51.100/24, 203.0.113/24, multicast, 240/4, broadcast, ::, ::1, mapped
loopback/private/link-local, fe80::/10, fec0::/10, fc00::/7, ff00::/8, 2001:db8::/32,
3fff::/20, 100::/64, NAT64 (both prefixes, incl. embedding a public IPv4), 6to4 (incl.
embedding a public IPv4), Teredo, 2001:2::/48, 2001:10::/28, IPv4-compatible `::127.0.0.1`,
and space outside 2000::/3 (`5f00::1`).

## Fetch behaviour (fetcher_test.go)

| Case                                                                                                                                                                                     | Test                                                                                          |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Successful fetch returns body, status, content type and final URL after a relative redirect; fragment stripped; UA/Accept/Accept-Language sent; no Cookie/Authorization/Referer          | `TestGetSuccessReturnsBodyAndFinalURL`                                                        |
| Redirect to host resolving to private, to host resolving to public+private mix, to `10.0.0.1`, to `169.254.169.254`, to `[::ffff:127.0.0.1]`, to `localhost`, to `http://`, to port 8443 | `TestRedirectToPrivateDestinationIsRejected`                                                  |
| Host whose resolution contains ANY private address; unresolvable host                                                                                                                    | `TestHostResolvingToAnyPrivateAddressIsRejected`                                              |
| Redirect chain longer than 3 hops (server hit exactly 4 times)                                                                                                                           | `TestRedirectChainLongerThanLimitFails`                                                       |
| Redirect to sign-in path -> needs_login                                                                                                                                                  | `TestRedirectToLoginPathNeedsLogin`                                                           |
| 404/410 -> not_found; 403 -> blocked; 403 login form -> needs_login; 401 -> needs_login; 429/503 -> blocked; 500/418 -> fetch_failed                                                     | `TestStatusMapping`                                                                           |
| Oversized body -> too_large                                                                                                                                                              | `TestOversizedBodyIsTooLarge`                                                                 |
| Gzip bomb (64 MiB decoded, < 1 MiB compressed) stopped by decoded limit -> too_large                                                                                                     | `TestGzipBombIsStoppedByDecodedLimit`                                                         |
| Slow server exceeds total budget -> timeout                                                                                                                                              | `TestSlowServerTimesOut`                                                                      |
| Signed S3 invoice link -> private_document_link with zero server hits and zero DNS lookups                                                                                               | `TestSignedInvoiceLinkIsNeverFetched`                                                         |
| `HTTPS_PROXY`/`HTTP_PROXY`/`ALL_PROXY` set to a recording proxy are ignored; transport `Proxy` is nil                                                                                    | `TestEnvironmentProxyIsIgnored`                                                               |
| DNS rebinding: resolver answers permitted then private; one fetch resolves exactly once (no second uncontrolled lookup by net/http); next fetch rejected                                 | `TestDNSRebindingCannotReachPrivateAddress/single_fetch_resolves_exactly_once`                |
| DNS rebinding between redirect hops on the same host                                                                                                                                     | `TestDNSRebindingCannotReachPrivateAddress/rebinding_between_redirect_hops`                   |
| Dial-time `Control` hook rejects 127.0.0.1, ::1, 10.0.0.1 sockets even when resolution is bypassed (verified to fail if the hook is removed)                                             | `TestDNSRebindingCannotReachPrivateAddress/dial-time_check_rejects_non-public_sockets`        |
| Loopback refused when the test-only option is absent                                                                                                                                     | `TestDNSRebindingCannotReachPrivateAddress/loopback_is_not_permitted_without_the_test_option` |
| Error text never contains query strings (404, DNS failure, private resolution, private literal, bad scheme)                                                                              | `TestErrorDetailNeverContainsQuery`                                                           |
| Per-host concurrency is 1                                                                                                                                                                | `TestPerHostConcurrencyIsOne`                                                                 |
| Host slot acquisition honours context cancellation                                                                                                                                       | `TestHostSlotRespectsContext`                                                                 |
