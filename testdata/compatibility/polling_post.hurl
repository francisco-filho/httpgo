POST http://example.invalid/poll
X-Probe: same-input
Content-Type: application/json
[Options]
retry: 2
retry-interval: 0
{"job":"123"}
HTTP 200
[Asserts]
jsonpath "$.status" == "READY"
GET http://example.invalid/health
HTTP 200
