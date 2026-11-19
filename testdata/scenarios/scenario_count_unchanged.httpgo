GET {{base_url}}/orders

HTTP 200
[Captures]
orders_before: jsonpath "$.items" count

POST {{base_url}}/orders/preview
Content-Type: application/json
{"productId":"p1"}

HTTP 200

GET {{base_url}}/orders

HTTP 200
[Asserts]
jsonpath "$.items" count == {{orders_before}}
