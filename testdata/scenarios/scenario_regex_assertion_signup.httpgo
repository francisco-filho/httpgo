GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
signup_code: regex "verificação é: ([0-9]{6})"
[Asserts]
regex "verificação é: ([0-9]{6})" matches "^[0-9]{6}$"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{signup_code}}"}
HTTP 200
