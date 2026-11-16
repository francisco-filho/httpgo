GET {{base_url}}/export

HTTP 200
[Asserts]
body contains "customer"
body not contains "passwordHash"
body not contains "trackingToken"
