# What link import can do (coverage report)

**Measured:** 6 November 2026 · Keeps extractor `extract-1` · 20 public URLs, fetched one at a time from a UK residential connection.

This is a small, dated sample, below the 50-URL evaluation the plan calls for. It shows what happened on that day. It is **not** a list of supported shops, and sites change without notice.

Whatever a link gives, you can always add the item by hand or upload a receipt. Link import never provides the price you paid, the purchase date or your invoice. Those come from you.

## Results by shop

| Shop                        | URLs | Product details read | Name        | Listing price (GBP) | Photos         | Notes                                                                                                                                                            |
| --------------------------- | ---- | -------------------- | ----------- | ------------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| John Lewis                  | 5    | 5/5                  | 5/5 correct | 4/5                 | 5/5            | The discontinued product had no price, which is correct. The out-of-stock product still showed its listing price.                                                |
| IKEA (GB)                   | 5    | 4/5 correct          | 4/5         | 4/5                 | 4/5 (4–5 each) | One old product URL redirected to a general products page. Keeps showed that page's title as a partial draft, so check the name on the review screen.            |
| Independent (tooled-up.com) | 1    | 1/1                  | 1/1         | 1/1                 | 1/1            |                                                                                                                                                                  |
| Argos                       | 5    | 0/5                  | —           | —                   | —              | The site refused automated requests (`blocked`). Use "Add manually" or "Upload receipt".                                                                         |
| eBay UK                     | 2    | 0/2                  | —           | —                   | —              | Refused automated requests (`blocked`).                                                                                                                          |
| Currys                      | 1    | 0/1                  | —           | —                   | —              | Refused (`blocked`). The URL was a category page; no product URLs were found for the test.                                                                       |
| Amazon UK                   | 1    | —                    | —           | —                   | —              | An order-history page correctly showed "This site needs a login". No Amazon product URLs were found for the test, so **Amazon product pages were not measured**. |

**Overall:** 10 of 20 URLs gave usable product details; 9 were refused by the site or were not product pages; 1 private page was recognised as needing a login. Each page took between 0.3 and 1.6 seconds.

## How the shops' pages were read

Every successful page was read from schema.org Product data (JSON-LD). Prices kept their exact pennies and the GBP currency. Keeps follows each site's `robots.txt`, and all 20 URLs were allowed.

## Not tested yet

- Amazon and Currys product pages.
- More than one variant on a single page.
- Prices in currencies other than GBP from live sites. These are covered only by offline test fixtures.

Re-measure with:

```bash
go test -tags retailerbenchmark -run TestRetailerBenchmark ./internal/importer/
```

Set `KEEPS_BENCHMARK_URLS` and `KEEPS_BENCHMARK_OUTPUT` first. The URL list and the raw results are kept with the planning documents.

## When a shop blocks Keeps: the Add to Keeps button

Shops that refuse automated requests (Argos, eBay and Currys in the sample above) still show their pages to you. The **Add to Keeps** browser button (Add item → "Shop blocks Keeps?") reads the product data the open page already publishes and opens Keeps' review screen with it. Keeps does not contact the shop for the page, and the data passes through the same extractor and limits. Photos are still downloaded through the safe fetcher; if a shop's image server also refuses, add photos yourself. Not yet measured live per shop.
