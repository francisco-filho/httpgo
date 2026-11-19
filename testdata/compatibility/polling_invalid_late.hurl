GET http://example.invalid/health
HTTP 200
GET http://example.invalid/poll
[Options]
retry: 1.5
HTTP 200
