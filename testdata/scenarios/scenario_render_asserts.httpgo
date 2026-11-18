POST {{base_url}}/orders
HTTP 200
[Captures]
order_number: jsonpath "$.number"
order_name: jsonpath "$.name"

GET {{base_url}}/orders/{{order_number}}
HTTP 200
[Asserts]
jsonpath "$.number" == {{order_number}}
jsonpath "$.name" == "{{order_name}}"
