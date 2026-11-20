POST http://example.invalid/health
Content-Type: application/json
X-Probe: {{code}}
[Options]
variable: n=42
variable: decimal=1.25
variable: flag=false
variable: nil=null
variable: code="123"
variable: empty=
variable: text="escaped\n\u{96ea}\""
variable: array=[1,true]
variable: object={"a":1}
{"n":{{n}},"decimal":{{decimal}},"flag":{{flag}},"nil":{{nil}},"code":"{{code}}","empty":"{{empty}}","text":"{{text}}","array":"{{array}}","object":"{{object}}"}
HTTP 200
