GET {{base}}/capture
HTTP 200
[Captures]
value: jsonpath "$"

POST {{base}}/next
{
  "text": "prefix{{value}}"
}
HTTP 200

GET {{base}}/last
