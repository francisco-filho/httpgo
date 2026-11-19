GET {{base_url}}/orders
HTTP 200
[Captures]
size: jsonpath "$.items" count

POST {{base_url}}/counts
Content-Type: application/json
{"count":{{size}}}
HTTP 200
