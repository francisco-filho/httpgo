GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
otp_code: regex "verificação é: ([0-9]{6})"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{otp_code}}"}
HTTP 200
