GET http://example.invalid/poll
[Options]
retry: 3
retry-interval: 0
HTTP 200
[Asserts]
jsonpath "$.status" == "READY"
GET http://example.invalid/health
HTTP 200
