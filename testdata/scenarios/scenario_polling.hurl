GET {{base_url}}/jobs/123
[Options]
retry: 3
retry-interval: 500
HTTP 200
[Asserts]
jsonpath "$.status" == "READY"
