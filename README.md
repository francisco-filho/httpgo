# httpgo

CLI independente em Go para executar um **subconjunto documentado** de arquivos
`.hurl`. Implementação própria, sem reutilizar código-fonte do Hurl.

## Executar

Com Go na versão de `go.mod`:

```sh
CGO_ENABLED=0 make build
bin/httpgo --help
bin/httpgo test testdata/scenarios/scenario_get_status.hurl
```

Para usar o servidor dummy do projeto, execute em outro terminal:

```sh
make serve
# Alias equivalente: make dummy
```

Ele escuta somente em `http://127.0.0.1:8080` (também acessível via localhost),
em primeiro plano. Ctrl+C encerra o servidor. A porta precisa estar livre.

| Método | Caminho | Status |
|---|---|---|
| GET | `/health` | 200 |
| GET | `/inexistente` | 404 |
| GET | `/error` | 500 |
| POST | `/products` | 201 |
| GET / PUT / PATCH | `/products/1` | 200 |
| DELETE | `/products/1` | 204 |
| GET / POST / PUT / PATCH / DELETE | `/echo` | 200 |

Os produtos são demonstrativos, **sem persistência**. `/echo` devolve os bytes do
JSON e os headers de resposta `X-Dummy-Method` e `X-Dummy-Body-Bytes`, sem ecoar
headers de autenticação/cookies. Use apenas dados fictícios.

O cenário `scenario_get_status.hurl` executa `/health`, `/inexistente` e repete
`/health`, validando três respostas em sequência. Também é possível executar:

```sh
bin/httpgo test testdata/scenarios/scenario_unvalidated_status.hurl
# POST de demonstração e sequência de métodos da US-02:
bin/httpgo test testdata/scenarios/scenario_json_inline.hurl
bin/httpgo test testdata/scenarios/scenario_dummy_methods.hurl
```

Esse cenário recebe 500 e informa explicitamente que não validou status.
O dummy em `tools/dummy/main.go` é um utilitário **exclusivamente manual** de
desenvolvimento, sem dependências novas. Não integra o binário httpgo nem os
pacotes do build, lint ou testes automáticos. Os testes continuam usando seus
próprios servidores isolados em portas dinâmicas: **não precisam de `make serve`**.
Veja [servidor dummy](docs/dev-server.md) para os detalhes.

```hurl
GET http://localhost:8080/health
HTTP 200

GET http://localhost:8080/inexistente
HTTP 404
```

Também são suportados `POST`, `PUT`, `PATCH` e `DELETE`, com headers e um valor
JSON inline opcional (objeto, array ou escalar; inclusive multilinha). GET aceita
headers, mas permanece sem body. Exemplo contra uma API local com `/products`:

```hurl
POST http://localhost:8080/products
Content-Type: application/json
{"name":"Pizza","price":4500}

HTTP 201
```

Salve como `products.hurl` e execute `bin/httpgo test products.hurl`. O mesmo
exemplo está em `testdata/scenarios/scenario_json_inline.hurl` e é exercitado
pelo binário nos E2E. O dummy manual também implementa essa demonstração: basta
iniciar `make serve`. Headers e JSON são validados antes de qualquer envio, e os bytes do valor
JSON estático são preservados, sem serialização. Declare Content-Type quando necessário;
não há inferência automática desse header.

## Parametrizar URL e headers (US-03 / issue #11)

Salve o exemplo abaixo como `products.hurl`:

```hurl
GET {{api.baseUrl}}/products
Authorization: Bearer {{token}}

HTTP 200
```

```sh
bin/httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var token=valor-ficticio --secret token
# Troque somente api.baseUrl para executar o mesmo arquivo em outro destino.
```

O cenário também está em `testdata/scenarios/scenario_products_variables.hurl`;
requer uma API com `GET /products` retornando 200 (o dummy manual não oferece esse
GET). Os E2E demonstram dois destinos locais isolados sem editar o cenário.

`--var nome=valor` é repetível. **Mudança de contrato na US-04:** um literal JSON
válido preserva seu tipo; o restante permanece texto, inclusive vazio. Strings JSON
perdem as aspas delimitadoras e interpretam escapes. O primeiro `=` separa nome/valor;
os seguintes são preservados. Nomes ASCII pontuados, como `api.baseUrl`, resolvem por
nome exato. Duplicações e conflitos ancestral/descendente, como `api`/`api.baseUrl`,
são erros independentemente do tipo, sem merge nem sobrescrita. Objetos são opacos:
`--var 'product={"name":"Pizza"}'` **não** define `product.name`.
URL e valores de headers aceitam `{{nome}}` e `{{ nome }}`, sem recursão. Nesses
contextos só são aceitos escalares; arrays/objetos geram erro antes do envio.
A URL final exige encoding explícito; nomes de headers permanecem estáticos.

## Parametrizar JSON preservando tipos (US-04 / issue #13)

Salve como `products.hurl`:

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}

HTTP 201
```

```sh
bin/httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var product.name=Pizza --var product.price=4500 \
  --var product.active=true --var 'product.code="001"'
```

A API recebe `name` e `code` como strings, `price` como número e `active` como
booleano. O cenário versionado é `testdata/scenarios/scenario_products_typed.hurl`;
os E2E verificam os dados recebidos por HTTP local. O dummy pode ser usado
manualmente, sem alterar seu código ou depender dele nos testes.

Placeholder sem aspas em posição de valor JSON completo preserva qualquer tipo,
inclusive `null`, array e objeto, também na raiz (`{{body}}`). Dentro de strings
JSON, aceita apenas escalares e composição textual, com escape correto de aspas,
barras invertidas e controles, preservando Unicode. Números conservam sua
representação JSON sem arredondamento, inclusive dentro de estruturas. Conteúdo
inserido semelhante a placeholder permanece literal. Chaves JSON são estáticas;
arquivos externos, expressões e acesso a propriedades não são suportados.

A interpretação tipada de `--var` é uma **extensão própria HTTPGo**, não promessa
de equivalência com a CLI do Hurl. A sintaxe contextual compartilhada e as diferenças
observadas contra Hurl 8.0.0 estão em [compatibilidade](docs/compatibility.md).

**É proibido fornecer credenciais reais em argumentos, inclusive `--var`.** Use
somente valores não sigilosos ou fictícios. `--secret nome` apenas marca proteção
nas saídas; não fornece valor nem protege histórico de shell/listagem de processos.
Marcar um nome ainda ausente é permitido, mas não resolve sua referência. A fonte
segura para credenciais iniciais pelo ambiente do processo chega na US-13;
stdin não está implementado. Capturas de respostas estão disponíveis desde a US-05,
ampliadas para qualquer tipo JSON na US-06.

## Autenticar e reutilizar token (US-05 / issue #14)

Salve como `products.hurl` (também em
`testdata/scenarios/scenario_auth_capture.hurl`):

```hurl
POST {{base_url}}/auth/login
Content-Type: application/json
{"username":"{{username}}","password":"{{password}}"}

HTTP 200
[Captures]
accessToken: jsonpath "$.accessToken"

GET {{base_url}}/products
Authorization: Bearer {{accessToken}}
HTTP 200
```

```sh
bin/httpgo test products.hurl --var base_url=http://localhost:8080 \
  --var username=demo --var password=senha-ficticia \
  --secret password --secret accessToken
```

Requer API que devolva `{"accessToken":"token-ficticio"}` no login e aceite esse
Bearer em `/products`; o dummy manual não implementa autenticação. Os E2E usam
servidor HTTP isolado e verificam o token efetivamente recebido. Todas as
credenciais do exemplo são **fictícias**; credenciais reais em argumentos continuam
proibidas.

`[Captures]` associa-se à resposta anterior, depois de `HTTP` quando declarado.
JSONPath aceita somente raiz `$`, propriedades por ponto (`$.product.id`) e
índices de array não negativos (`$.items[0]`). Desde a US-06, o resultado aceita
qualquer tipo JSON, inclusive raiz, `null`, string vazia, arrays/objetos vazios.
JSON inválido, caminho ausente ou índice fora do array é erro de captura (código 2).
Consultas não suportadas em qualquer entrada impedem todos os envios.

Capturas ficam disponíveis após consumir/fechar a resposta e aprovar o status,
quando esperado, e as assertions de headers declaradas, para URLs, valores de
headers e JSON inline seguintes. Strings
como `"123"`, `"null"` ou `"{{literal}}"` continuam strings, sem reinterpretação.
Nomes pontuados resolvem por nome exato; duplicações e conflitos hierárquicos com
variáveis iniciais ou capturas anteriores falham sem sobrescrever. Cada execução
começa sem capturas, e continua aceitando exatamente um arquivo. `--secret` pode
marcar uma captura futura sem criá-la; valores descobertos ficam ocultos nos
streams e diagnósticos, inclusive variantes de escape conhecidas. Não há garantia
de reconhecer segredos arbitrários antes de descobri-los ou codificações desconhecidas.

## Encadear operações com capturas tipadas (US-06 / issue #16)

Salve como `products.hurl` (também em
`testdata/scenarios/scenario_capture_products.hurl`):

```hurl
POST {{base_url}}/products
Content-Type: application/json
{"name":"Pizza"}

HTTP 201
[Captures]
productId: jsonpath "$.id"

GET {{base_url}}/products/{{productId}}
HTTP 200

POST {{base_url}}/echo
Content-Type: application/json
{"productId":{{productId}}}
HTTP 200
```

```sh
bin/httpgo test products.hurl --var base_url=http://localhost:8080
```

Uma API que retorna `{"id":9007199254740993}` recebe o caminho
`/products/9007199254740993` e o último body com esse mesmo ID **numérico**, sem
arredondamento. Os três status são validados. Os E2E verificam esses bytes por
HTTP real e também o fluxo login → token → criação → ID → consulta → echo de
`testdata/scenarios/scenario_capture_auth_products.hurl`. O dummy manual não
implementa esse contrato dinâmico de ID/autenticação.

**Ampliação intencional do contrato US-05:** capturas agora preservam string,
número, booleano, `null`, array e objeto em posição de valor JSON completo,
inclusive na raiz. Números mantêm precisão também dentro de estruturas; strings
semelhantes a JSON ou placeholders não são reinterpretadas. Em URL, header ou
string JSON, escalares usam sua representação textual (string decodificada,
número JSON, `true`/`false`, `null`); arrays/objetos falham antes do envio dependente.
Não há percent-encoding automático; escape JSON e restrições de headers continuam.
Capturar `product` como objeto não cria `product.id`: declare outra captura com
nome não conflitante, por exemplo `productId: jsonpath "$.id"`. Colisões seguem
proibidas em qualquer tipo; publicação do lote é atômica. `--secret` acompanha
capturas escalares e compostas e protege também seus constituintes e escapes
conhecidos sem mudar os dados enviados nem criar variáveis para as propriedades.

A estrutura do arquivo inteiro é validada antes da rede. Variáveis ausentes e
valores finais inválidos falham imediatamente antes da requisição dependente,
sem contá-la como iniciada e sem enviar as seguintes. As requisições são
sequenciais e a primeira falha de status, header, captura, renderização ou transporte
interrompe a execução. Uma falha de captura preserva a resposta concluída e o
status aprovado nas contagens, mas impede o próximo envio. Sem `HTTP`, qualquer status recebido é aceito,
com indicação explícita de **sem validação de status**.

- `0`: sucesso; `1`: status ou assertion de header divergente; `2`: demais erros.
- Timeout total de 10 segundos por requisição, incluindo redirects e leitura do body.
- TLS verificado; até 10 redirects, apenas na mesma origem; nenhum retry.
- Cada resultado é emitido em stdout assim que a requisição termina, antes da próxima;
  resumo ao final e diagnósticos em stderr. Sempre texto sem cores.
- 301/302/303 convertem POST/PUT/PATCH/DELETE em GET sem body; 307/308 preservam ambos.
- Bodies e coleções de headers não são impressos. Apenas valores comuns sanitizados
  podem aparecer no diagnóstico de assertion; valores sensíveis nunca são expostos.
- Credenciais, query inteira e fragmento da URL não são exibidos.
  Valores marcados com `--secret` também são ocultos em paths e headers comuns,
  antes de escapes/normalização de URL. O restante do path permanece visível:
  não coloque segredos arbitrários nele. A CLI não detecta toda credencial ou
  codificação desconhecida automaticamente. A proteção cobre formas brutas,
  decodificadas e escapes conhecidos (JSON, quoting de diagnóstico e URL), inclusive
  em falhas anteriores à construção do contexto. Não há payload de diagnóstico.

## Validar headers da resposta (US-07 / issue #18)

Salve como `health.hurl` e execute `bin/httpgo test health.hurl` contra uma API
que devolva `Content-Type: application/json; charset=utf-8` e `X-Request-Id: req-123`:

```hurl
GET http://localhost:8080/health
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
header "X-Request-Id" exists
header "X-Request-Id" == "req-123"
```

O cenário também está em `testdata/scenarios/scenario_assert_headers.hurl` e é
verificado por HTTP real nos E2E. O dummy manual não promete esses headers.
Esta é uma **ampliação intencional**: `[Asserts]`, antes rejeitada globalmente,
agora aceita somente essas três formas, com strings literais entre aspas duplas.
Nomes são case-insensitive; igualdade e substring distinguem caixa dos valores.
`{{nome}}` no esperado permanece literal, sem interpolação. Regex, assertions
JSON, filtros, captura de headers e outros operadores continuam não suportados.

Só a primeira ocorrência de cada header participa da comparação, sem concatenar
ou separar vírgulas. Ausência faz todos os operadores falharem; presente vazio
satisfaz `exists` e `== ""`. `contains ""` passa em qualquer valor presente.

`[Asserts]` exige declaração `HTTP` na entrada. `HTTP *` é aceito com ou sem
assertions/capturas e permite qualquer status, inclusive 404/503. Sua linha mostra
`status aceito por HTTP *`; o resumo conta em `sem_validação`, não em `validadas`.
Isso dispensa **só a comparação numérica**, não headers nem capturas declarados.

Após consumo/fechamento da resposta final, a ordem é status numérico → headers →
capturas, independentemente da ordem das seções. Uma falha de header retorna 1,
localiza a assertion e esperado/recebido (ausência difere de `""`), não publica
capturas nem envia a próxima requisição. Erro estrutural tardio retorna 2 sem
qualquer envio. Escrita, consumo incompleto, timeout e cancelamento continuam 2.
Valores sensíveis de Authorization/Proxy-Authorization/Cookie/Set-Cookie são
ocultos nos dois lados, inclusive esperados literais; headers comuns usam a
proteção de segredos conhecidos. Nenhum body ou coleção de headers é exibido.

Equivalência com Hurl 8.0.0 é limitada aos pares observados: **repetidos são listas
no Hurl**, que também interpola esperados e avalia capturas antes das assertions.
Essas divergências são deliberadas, preservando as regras da história; veja
[matriz de compatibilidade](docs/compatibility.md).

## Documentação e desenvolvimento

- [Subconjunto e contratos da CLI](docs/subset.md)
- [Arquitetura e ciclo de vida](docs/architecture.md)
- [Servidor dummy para desenvolvimento](docs/dev-server.md)
- [Compatibilidade diferencial com Hurl 8.0.0](docs/compatibility.md)
- [Comandos, evidências e bloqueios de validação](docs/validation.md)
- [Regras do repositório](AGENTS.md)

```sh
make fmt lint test
# Explicitamente, além do lint:
go vet ./...
make build
make compatibility
# Opcional durante as histórias; build cruzado não comprova execução nativa:
make portable
```

Os testes com `-race` exigem CGO e compilador/headers C no ambiente de
**desenvolvimento**, não no binário entregue. O gate local com race já passou;
a evidência Linux por história está registrada em `docs/validation.md`, separada dos
históricos anteriores. Conforme o plano v4, seção 7, cross-build e testes nativos
macOS/Windows são opcionais nesta história e ficam para a validação final de
portabilidade/release, sem bloquear a entrega. Não foi executado runtime nessas
plataformas nesta história; cross-build não o comprovaria. A aprovação da história
passa pelo gate de revisão; não há alegação de compatibilidade Hurl completa.
